package handlers

import (
	"RIP/internal/db"
	"RIP/internal/models"
	"RIP/internal/session"
	"encoding/json"
	"net/http"

	"golang.org/x/crypto/bcrypt"
)

// RegisterUser godoc
// @Summary Register new user
// @Description Create a new user account
// @Tags users
// @Accept json
// @Produce json
// @Param user body models.User true "User data"
// @Success 200 {object} models.User
// @Failure 400 {string} string "Invalid request body"
// @Failure 500 {string} string "Error creating user"
// @Router /api/users/register [post]
func RegisterUser(w http.ResponseWriter, r *http.Request) {
	var user models.User
	if err := json.NewDecoder(r.Body).Decode(&user); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(user.Password), bcrypt.DefaultCost)
	if err != nil {
		http.Error(w, "Error hashing password", http.StatusInternalServerError)
		return
	}
	user.Password = string(hashedPassword)
	if err := db.DB.Create(&user).Error; err != nil {
		http.Error(w, "Error creating user", http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(user)
}

// GetMe godoc
// @Summary Get current user
// @Description Get details of the logged-in user
// @Tags users
// @Produce json
// @Success 200 {object} models.User
// @Failure 401 {string} string "Unauthorized"
// @Failure 404 {string} string "User not found"
// @Security BearerAuth
// @Router /api/users/me [get]
func GetMe(w http.ResponseWriter, r *http.Request) {
	sess := session.GetUser(r)
	if sess == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	var user models.User
	if err := db.DB.Where("id = ?", sess.UserID).First(&user).Error; err != nil {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}
	json.NewEncoder(w).Encode(user)
}

// UpdateMe godoc
// @Summary Update current user
// @Description Update logged-in user details
// @Tags users
// @Accept json
// @Produce json
// @Param user body models.User true "Updated user data"
// @Success 200 {object} models.User
// @Failure 401 {string} string "Unauthorized"
// @Failure 400 {string} string "Invalid request body"
// @Failure 500 {string} string "Error updating user"
// @Security BearerAuth
// @Router /api/users/me [put]
func UpdateMe(w http.ResponseWriter, r *http.Request) {
	sess := session.GetUser(r)
	if sess == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	var user models.User
	if err := db.DB.Where("id = ?", sess.UserID).First(&user).Error; err != nil {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}
	var updates models.User
	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	user.Login = updates.Login
	if updates.Password != "" {
		hashed, err := bcrypt.GenerateFromPassword([]byte(updates.Password), bcrypt.DefaultCost)
		if err != nil {
			http.Error(w, "Error hashing password", http.StatusInternalServerError)
			return
		}
		user.Password = string(hashed)
	}
	user.IsModerator = updates.IsModerator
	if err := db.DB.Save(&user).Error; err != nil {
		http.Error(w, "Error updating user", http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(user)
}

// Login godoc
// @Summary User login
// @Description Authenticate user and return JWT token
// @Tags users
// @Accept json
// @Produce json
// @Param credentials body object true "Login credentials"
// @Success 200 {object} object "Token"
// @Failure 400 {string} string "Invalid request body"
// @Failure 401 {string} string "Invalid credentials"
// @Router /api/users/login [post]
func Login(w http.ResponseWriter, r *http.Request) {
	var creds struct {
		Login    string `json:"login"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&creds); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	var user models.User
	if err := db.DB.Where("login = ?", creds.Login).First(&user).Error; err != nil {
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(creds.Password)); err != nil {
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}
	token := session.CreateSession(&user) // ← только один аргумент
	json.NewEncoder(w).Encode(map[string]string{"token": token})
}

// Logout godoc
// @Summary User logout
// @Description Clear JWT token
// @Tags users
// @Success 200 {string} string "OK"
// @Security BearerAuth
// @Router /api/users/logout [post]
func Logout(w http.ResponseWriter, r *http.Request) {
	session.Logout(w, r)
}
