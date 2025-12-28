package handlers

import (
	"RIP/internal/db"
	"RIP/internal/models"
	"RIP/internal/session"
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"
)

type CartInfo struct {
	ID    string `json:"id"`
	Count int    `json:"count"`
}

// GetCartInfo godoc
// @Summary Get cart info
// @Description Get current draft request info (user required)
// @Tags tpq_requests
// @Produce json
// @Success 200 {object} CartInfo
// @Failure 401 {string} string "Unauthorized"
// @Security BearerAuth
// @Router /api/tpq_requests/cart [get]
func GetCartInfo(w http.ResponseWriter, r *http.Request) {
	sess := session.GetUser(r)
	if sess == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	currentReq := getCurrentDraftRequest(sess.UserID)
	if currentReq == nil {
		json.NewEncoder(w).Encode(CartInfo{ID: "", Count: 0})
		return
	}
	json.NewEncoder(w).Encode(CartInfo{ID: currentReq.ID, Count: len(currentReq.TPQItems)})
}

// GetTPQRequests godoc
// @Summary Get TPQ requests
// @Description Get list of TPQ requests, filtered by user or all for moderator
// @Tags tpq_requests
// @Produce json
// @Param status query string false "Status filter"
// @Param from_date query string false "From date"
// @Param to_date query string false "To date"
// @Success 200 {array} models.TPQRequest
// @Failure 401 {string} string "Unauthorized"
// @Security BearerAuth
// @Router /api/tpq_requests [get]
func GetTPQRequests(w http.ResponseWriter, r *http.Request) {
	sess := session.GetUser(r)
	if sess == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	status := r.URL.Query().Get("status")
	fromDate := r.URL.Query().Get("from_date")
	toDate := r.URL.Query().Get("to_date")
	var requests []models.TPQRequest
	q := db.DB.Preload("TPQItems.Artifact").Where("status NOT IN (?, ?)", "draft", "deleted")
	if !sess.IsModerator {
		q = q.Where("creator_id = ?", sess.UserID)
	}
	if status != "" {
		q = q.Where("status = ?", status)
	}
	if fromDate != "" && toDate != "" {
		toDateEnd, err := time.Parse("2006-01-02", toDate)
		if err != nil {
			log.Println("Error parsing to_date:", err)
			http.Error(w, "Invalid to_date format", http.StatusBadRequest)
			return
		}
		toDateEnd = toDateEnd.Add(24 * time.Hour).Add(-time.Second) // End of day
		q = q.Where("formed_at >= ? AND formed_at <= ?", fromDate, toDateEnd)
	}
	if err := q.Find(&requests).Error; err != nil {
		log.Println("Database query error:", err)
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}
	log.Printf("Found %d requests", len(requests))
	for i := range requests {
		var creator models.User
		if err := db.DB.Where("id = ?", requests[i].CreatorID).First(&creator).Error; err == nil {
			log.Printf("Request %s: Creator login=%s", requests[i].ID, creator.Login)
		}
		if requests[i].ModeratorID != nil {
			var moderator models.User
			if err := db.DB.Where("id = ?", *requests[i].ModeratorID).First(&moderator).Error; err == nil {
				log.Printf("Request %s: Moderator login=%s", requests[i].ID, moderator.Login)
			}
		}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(requests)
}

// GetTPQRequest godoc
// @Summary Get TPQ request by ID
// @Description Get details of a specific TPQ request (user required, own or moderator)
// @Tags tpq_requests
// @Produce json
// @Param id path string true "Request ID"
// @Success 200 {object} models.TPQRequest
// @Failure 401 {string} string "Unauthorized"
// @Failure 403 {string} string "Forbidden"
// @Failure 404 {string} string "Request not found"
// @Security BearerAuth
// @Router /api/tpq_requests/{id} [get]
func GetTPQRequest(w http.ResponseWriter, r *http.Request) {
	sess := session.GetUser(r)
	if sess == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 4 {
		http.Error(w, "Invalid URL", http.StatusBadRequest)
		return
	}
	id := pathParts[3]
	var req models.TPQRequest
	if err := db.DB.Preload("TPQItems.Artifact").Where("id = ? AND status != ?", id, "deleted").First(&req).Error; err != nil {
		http.Error(w, "Request not found", http.StatusNotFound)
		return
	}
	if !sess.IsModerator && req.CreatorID != sess.UserID {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	json.NewEncoder(w).Encode(req)
}

// UpdateTPQRequest godoc
// @Summary Update TPQ request
// @Description Update a TPQ request (user required, own draft)
// @Tags tpq_requests
// @Accept json
// @Produce json
// @Param id path string true "Request ID"
// @Param request body models.TPQRequest true "Updated request data"
// @Success 200 {object} models.TPQRequest
// @Failure 401 {string} string "Unauthorized"
// @Failure 403 {string} string "Forbidden"
// @Failure 404 {string} string "Request not found"
// @Security BearerAuth
// @Router /api/tpq_requests/{id} [put]
func UpdateTPQRequest(w http.ResponseWriter, r *http.Request) {
	sess := session.GetUser(r)
	if sess == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 4 {
		http.Error(w, "Invalid URL", http.StatusBadRequest)
		return
	}
	id := pathParts[3]
	var req models.TPQRequest
	if err := db.DB.Where("id = ?", id).First(&req).Error; err != nil {
		http.Error(w, "Request not found", http.StatusNotFound)
		return
	}
	if req.CreatorID != sess.UserID || req.Status != "draft" {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	var updates models.TPQRequest
	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	req.Excavation = updates.Excavation
	if err := db.DB.Save(&req).Error; err != nil {
		http.Error(w, "Error updating request", http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(req)
}

// FormTPQRequest godoc
// @Summary Form TPQ request
// @Description Form a draft TPQ request (user required)
// @Tags tpq_requests
// @Produce json
// @Param id path string true "Request ID"
// @Success 200 {object} models.TPQRequest
// @Failure 401 {string} string "Unauthorized"
// @Failure 400 {string} string "Cannot form"
// @Security BearerAuth
// @Router /api/tpq_requests/{id}/form [put]
func FormTPQRequest(w http.ResponseWriter, r *http.Request) {
	sess := session.GetUser(r)
	if sess == nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 4 {
		http.Error(w, `{"error":"invalid url"}`, http.StatusBadRequest)
		return
	}
	id := pathParts[3]

	var req models.TPQRequest
	if err := db.DB.Preload("TPQItems").Where("id = ? AND status = ?", id, "draft").First(&req).Error; err != nil {
		http.Error(w, `{"error":"request not found or not draft"}`, http.StatusBadRequest)
		return
	}

	if req.CreatorID != sess.UserID {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}

	if len(req.TPQItems) == 0 {
		http.Error(w, `{"error":"request is empty"}`, http.StatusBadRequest)
		return
	}

	now := time.Now()
	req.FormedAt = &now
	req.Status = "formed"
	req.ModerationStatus = "" // явно очищаем
	req.ModeratedAt = nil

	if err := db.DB.Save(&req).Error; err != nil {
		http.Error(w, `{"error":"db error"}`, http.StatusInternalServerError)
		return
	}

	// go func(requestID string) {
	// 	payload := map[string]string{"pk": requestID}
	// 	jsonData, _ := json.Marshal(payload)

	// 	resp, err := http.Post("http://localhost:8001/moderate/", "application/json", bytes.NewBuffer(jsonData))
	// 	if err != nil {
	// 		log.Printf("[Moderation] Error calling async service for %s: %v", requestID, err)
	// 		return
	// 	}
	// 	resp.Body.Close()
	// 	log.Printf("[Moderation] Moderation started for request %s", requestID)
	// }(req.ID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(req)
}

// CompleteTPQRequest godoc
// @Summary Complete TPQ request
// @Description Complete a formed TPQ request (moderator required) - starts async calculation
// @Tags tpq_requests
// @Produce json
// @Param id path string true "Request ID"
// @Success 200 {object} models.TPQRequest
// @Failure 401 {string} string "Unauthorized"
// @Failure 403 {string} string "Forbidden"
// @Failure 400 {string} string "Cannot complete"
// @Security BearerAuth
// @Router /api/tpq_requests/{id}/complete [put]
func CompleteTPQRequest(w http.ResponseWriter, r *http.Request) {
	sess := session.GetUser(r)
	if sess == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	if !sess.IsModerator {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 4 {
		http.Error(w, "Invalid URL", http.StatusBadRequest)
		return
	}
	id := pathParts[3]

	var req models.TPQRequest
	if err := db.DB.Preload("TPQItems.Artifact").Where("id = ? AND status = ?", id, "formed").First(&req).Error; err != nil {
		http.Error(w, "Cannot complete: not formed", http.StatusBadRequest)
		return
	}

	moderatorID := sess.UserID
	req.ModeratorID = &moderatorID
	now := time.Now()
	req.CompletedAt = &now

	req.Status = "processing"
	req.ModerationStatus = "pending"

	req.Result = nil

	if err := db.DB.Save(&req).Error; err != nil {
		http.Error(w, "Error updating request", http.StatusInternalServerError)
		return
	}

	go func(requestID string) {
		payload := map[string]string{"pk": requestID}
		jsonData, _ := json.Marshal(payload)

		log.Printf("[Async Calculation] Starting async calculation for request %s", requestID)

		resp, err := http.Post("http://localhost:8001/moderate/", "application/json", bytes.NewBuffer(jsonData))
		if err != nil {
			log.Printf("[Async Calculation] Error calling async service for %s: %v", requestID, err)

			var failedReq models.TPQRequest
			if err := db.DB.First(&failedReq, "id = ?", requestID).Error; err == nil {
				failedReq.Status = "failed"
				failedReq.ModerationStatus = "calculation_failed"
				db.DB.Save(&failedReq)
			}
			return
		}
		resp.Body.Close()
		log.Printf("[Async Calculation] Async calculation started for request %s", requestID)
	}(req.ID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(req)
}

// RejectTPQRequest godoc
// @Summary Reject TPQ request
// @Description Reject a formed TPQ request (moderator required)
// @Tags tpq_requests
// @Produce json
// @Param id path string true "Request ID"
// @Success 200 {object} models.TPQRequest
// @Failure 401 {string} string "Unauthorized"
// @Failure 403 {string} string "Forbidden"
// @Failure 400 {string} string "Cannot reject"
// @Security BearerAuth
// @Router /api/tpq_requests/{id}/reject [put]
func RejectTPQRequest(w http.ResponseWriter, r *http.Request) {
	sess := session.GetUser(r)
	if sess == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	if !sess.IsModerator {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 4 {
		http.Error(w, "Invalid URL", http.StatusBadRequest)
		return
	}
	id := pathParts[3]
	var req models.TPQRequest
	if err := db.DB.Where("id = ? AND status = ?", id, "formed").First(&req).Error; err != nil {
		http.Error(w, "Cannot reject: not formed", http.StatusBadRequest)
		return
	}
	moderatorID := sess.UserID
	req.ModeratorID = &moderatorID
	now := time.Now()
	req.CompletedAt = &now
	req.Status = "rejected"
	if err := db.DB.Save(&req).Error; err != nil {
		http.Error(w, "Error rejecting request", http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(req)
}

// DeleteTPQRequest godoc
// @Summary Delete TPQ request
// @Description Delete a draft TPQ request (user required, own draft)
// @Tags tpq_requests
// @Param id path string true "Request ID"
// @Success 204 {string} string "No Content"
// @Failure 401 {string} string "Unauthorized"
// @Failure 403 {string} string "Forbidden"
// @Failure 404 {string} string "Cannot delete"
// @Security BearerAuth
// @Router /api/tpq_requests/{id} [delete]
func DeleteTPQRequest(w http.ResponseWriter, r *http.Request) {
	sess := session.GetUser(r)
	if sess == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 4 {
		http.Error(w, "Invalid URL", http.StatusBadRequest)
		return
	}
	id := pathParts[3]
	var req models.TPQRequest
	if err := db.DB.Where("id = ? AND status = ?", id, "draft").First(&req).Error; err != nil {
		http.Error(w, "Cannot delete: not draft", http.StatusBadRequest)
		return
	}
	if req.CreatorID != sess.UserID {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	req.Status = "deleted"
	if err := db.DB.Save(&req).Error; err != nil {
		http.Error(w, "Error deleting request", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
