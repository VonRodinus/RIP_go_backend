// internal/handlers/api_moderate.go
package handlers

import (
	"RIP/internal/db"
	"RIP/internal/models"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"
)

const MODERATION_TOKEN = "A1B2C3D4"

func ModerateTPQRequest(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.TrimSuffix(r.URL.Path, "/"), "/")
	if len(parts) < 5 || parts[4] != "moderate" {
		http.Error(w, `{"error":"invalid url"}`, http.StatusBadRequest)
		return
	}
	id := parts[3]

	var input struct {
		ResultTPQ int    `json:"result_tpq"`
		Token     string `json:"token"`
	}

	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}

	if input.Token != MODERATION_TOKEN {
		http.Error(w, `{"error":"invalid token"}`, http.StatusUnauthorized)
		return
	}

	var req models.TPQRequest
	if err := db.DB.Preload("TPQItems.Artifact").First(&req, "id = ?", id).Error; err != nil {
		http.Error(w, `{"error":"request not found"}`, http.StatusNotFound)
		return
	}

	if req.Status != "processing" && req.Status != "formed" {
		http.Error(w, `{"error":"request must be in processing or formed status"}`, http.StatusBadRequest)
		return
	}

	if req.Status == "completed" && req.ModerationStatus != "" {
		log.Printf("[ModerateTPQRequest] Overriding existing result for request %s", id)
	}

	now := time.Now()
	req.Status = "completed"
	req.ModerationStatus = "calculated"
	req.ModeratedAt = &now
	req.CompletedAt = &now
	req.Result = &input.ResultTPQ

	if err := db.DB.Save(&req).Error; err != nil {
		http.Error(w, `{"error":"db error"}`, http.StatusInternalServerError)
		return
	}

	log.Printf("[ModerateTPQRequest] Request %s completed with TPQ=%d", id, input.ResultTPQ)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"message":      "tpq calculated and applied",
		"status":       req.Status,
		"result_tpq":   req.Result,
		"moderated_at": now,
	})
}

// UpdateTPQResult godoc
// @Summary Update TPQ result manually
// @Description Manually update TPQ result for a request (for testing/demo purposes)
// @Tags tpq_requests
// @Accept json
// @Produce json
// @Param id path string true "Request ID"
// @Param input body UpdateTPQResultInput true "Update data"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]string "Invalid input"
// @Failure 401 {object} map[string]string "Unauthorized - invalid token"
// @Failure 404 {object} map[string]string "Request not found"
// @Failure 409 {object} map[string]string "Already processed"
// @Router /api/tpq_requests/{id}/update_result [put]
func UpdateTPQResult(w http.ResponseWriter, r *http.Request) {

	log.Printf("[UpdateTPQResult] Received manual update request")

	parts := strings.Split(strings.TrimSuffix(r.URL.Path, "/"), "/")
	if len(parts) < 5 || parts[4] != "update_result" {
		http.Error(w, `{"error":"invalid url"}`, http.StatusBadRequest)
		return
	}
	id := parts[3]

	var input struct {
		ResultTPQ int    `json:"result_tpq"`
		Token     string `json:"token"`
		Comment   string `json:"comment,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		log.Printf("[UpdateTPQResult] JSON decode error: %v", err)
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}

	if input.Token != MODERATION_TOKEN {
		log.Printf("[UpdateTPQResult] Invalid token provided: %s", input.Token)
		http.Error(w, `{"error":"invalid token"}`, http.StatusUnauthorized)
		return
	}

	log.Printf("[UpdateTPQResult] Processing request %s with TPQ=%d", id, input.ResultTPQ)

	var req models.TPQRequest
	if err := db.DB.Preload("TPQItems.Artifact").First(&req, "id = ?", id).Error; err != nil {
		http.Error(w, `{"error":"request not found"}`, http.StatusNotFound)
		return
	}

	if req.Status == "draft" {
		http.Error(w, `{"error":"request is still in draft status, form it first"}`, http.StatusBadRequest)
		return
	}

	now := time.Now()

	oldStatus := req.Status
	oldResult := req.Result
	oldModStatus := req.ModerationStatus

	req.Status = "completed"
	req.ModerationStatus = "manually_updated"
	req.ModeratedAt = &now
	req.CompletedAt = &now
	req.Result = &input.ResultTPQ

	if err := db.DB.Save(&req).Error; err != nil {
		log.Printf("[UpdateTPQResult] DB save error: %v", err)
		http.Error(w, `{"error":"db error"}`, http.StatusInternalServerError)
		return
	}

	log.Printf("[UpdateTPQResult] Successfully updated request %s: TPQ=%d (was %v), status=%s (was %s)",
		id, input.ResultTPQ, oldResult, req.Status, oldStatus)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message":               "tpq manually updated",
		"request_id":            req.ID,
		"old_status":            oldStatus,
		"old_moderation_status": oldModStatus,
		"old_result_tpq":        oldResult,
		"new_status":            req.Status,
		"new_moderation_status": req.ModerationStatus,
		"new_result_tpq":        req.Result,
		"moderated_at":          req.ModeratedAt,
		"comment":               input.Comment,
		"source":                "manual_update",
	})
}

type UpdateTPQResultInput struct {
	ResultTPQ int    `json:"result_tpq" example:"150"`
	Token     string `json:"token" example:"A1B2C3D4"`
	Comment   string `json:"comment,omitempty" example:"Manual update for demo"`
}
