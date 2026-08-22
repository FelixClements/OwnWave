package main

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)
func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	if req.Username == "" || req.Password == "" {
		http.Error(w, "username and password required", 400)
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	userID, err := h.db.CreateUser(r.Context(), req.Username, string(hash))
	if err != nil {
		http.Error(w, "username taken", 409)
		return
	}

	token, err := h.createSession(r.Context(), userID)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"token": token,
		"user": map[string]string{
			"id":       userID,
			"username": req.Username,
		},
	})
}
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	if req.Username == "" || req.Password == "" {
		http.Error(w, "username and password required", 400)
		return
	}

	user, err := h.db.GetUserByUsername(r.Context(), req.Username)
	if err != nil {
		http.Error(w, "invalid credentials", 401)
		return
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		http.Error(w, "invalid credentials", 401)
		return
	}

	token, err := h.createSession(r.Context(), user.ID)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"token": token,
		"user": map[string]string{
			"id":       user.ID,
			"username": user.Username,
		},
	})
}
func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	auth := r.Header.Get("Authorization")
	if !strings.HasPrefix(auth, "Bearer ") {
		http.Error(w, "unauthorized", 401)
		return
	}
	if _, ok := h.authUser(r); !ok {
		http.Error(w, "unauthorized", 401)
		return
	}
	token := strings.TrimPrefix(auth, "Bearer ")
	if err := h.db.DeleteSession(r.Context(), hashToken(token)); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	user, ok := h.authUser(r)
	if !ok {
		http.Error(w, "unauthorized", 401)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"id":        user.ID,
		"username":  user.Username,
		"email":     user.Email,
		"full_name": user.FullName,
		"is_admin":  user.IsAdmin,
	})
}
func (h *Handler) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	user, ok := h.authUser(r)
	if !ok {
		http.Error(w, "unauthorized", 401)
		return
	}

	var req struct {
		Email    *string `json:"email"`
		FullName *string `json:"full_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}

	email := ""
	if req.Email != nil {
		email = *req.Email
	}
	fullName := ""
	if req.FullName != nil {
		fullName = *req.FullName
	}

	if err := h.db.UpdateUserProfile(r.Context(), user.ID, email, fullName); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
func (h *Handler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	user, ok := h.authUser(r)
	if !ok {
		http.Error(w, "unauthorized", 401)
		return
	}

	var req struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	if req.CurrentPassword == "" || req.NewPassword == "" {
		http.Error(w, "current and new password required", 400)
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.CurrentPassword)); err != nil {
		http.Error(w, "invalid current password", 401)
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	if err := h.db.UpdateUserPassword(r.Context(), user.ID, string(hash)); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
func (h *Handler) authUser(r *http.Request) (User, bool) {
	auth := r.Header.Get("Authorization")
	if !strings.HasPrefix(auth, "Bearer ") {
		return User{}, false
	}
	token := strings.TrimPrefix(auth, "Bearer ")
	if token == "" {
		return User{}, false
	}
	user, err := h.db.GetUserByTokenHash(r.Context(), hashToken(token))
	if err != nil {
		return User{}, false
	}
	return user, true
}
func (h *Handler) createSession(ctx context.Context, userID string) (string, error) {
	token, err := generateSessionToken()
	if err != nil {
		return "", err
	}
	expiresAt := time.Now().Add(7 * 24 * time.Hour)
	if _, err := h.db.CreateSession(ctx, userID, hashToken(token), expiresAt); err != nil {
		return "", err
	}
	return token, nil
}
