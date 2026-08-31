package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"

	"ownwave/api/internal/auth"
)

func (h *Handler) CreateInvite(w http.ResponseWriter, r *http.Request) {
	admin, _ := auth.UserFromContext(r.Context())

	var req struct {
		Username *string `json:"username"`
		TTLHours *int    `json:"ttl_hours"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && err.Error() != "EOF" {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	ttl := 7 * 24 * time.Hour
	if req.TTLHours != nil && *req.TTLHours > 0 {
		ttl = time.Duration(*req.TTLHours) * time.Hour
	}

	token, invite, err := h.auth.CreateInvite(r.Context(), admin.ID, req.Username, ttl)
	if err != nil {
		writeInternalError(w, r, "create invite", err)
		return
	}

	baseURL := strings.TrimSuffix(os.Getenv("PUBLIC_APP_URL"), "/")
	if baseURL == "" {
		baseURL = "https://" + os.Getenv("OWNWAVE_DOMAIN")
	}
	if baseURL == "https://" || baseURL == "" {
		baseURL = "/invite"
	}
	inviteURL := baseURL + "/invite/" + token

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"invite_url": inviteURL,
		"expires_at": invite.ExpiresAt,
		"invite":     invite,
	})
}

func (h *Handler) CreateUser(w http.ResponseWriter, r *http.Request) {
	admin, _ := auth.UserFromContext(r.Context())

	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	user, err := h.auth.CreateUser(r.Context(), admin.ID, req.Username, req.Password)
	if err != nil {
		switch {
		case errors.Is(err, auth.ErrUsernameTaken):
			http.Error(w, "username taken", http.StatusConflict)
		case errors.Is(err, auth.ErrWeakPassword), errors.Is(err, auth.ErrInvalidUsername):
			http.Error(w, err.Error(), http.StatusBadRequest)
		default:
			writeInternalError(w, r, "create user", err)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{"user": user})
}

func (h *Handler) ListUsers(w http.ResponseWriter, r *http.Request) {
	users, err := h.auth.ListUsers(r.Context())
	if err != nil {
		writeInternalError(w, r, "list users", err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"users": users})
}

func (h *Handler) DeleteUser(w http.ResponseWriter, r *http.Request) {
	admin, _ := auth.UserFromContext(r.Context())
	targetID := chi.URLParam(r, "id")

	if err := h.auth.DeleteUser(r.Context(), admin.ID, targetID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.Error(w, "user not found", http.StatusNotFound)
			return
		}
		if err.Error() == "cannot delete own account" {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		writeInternalError(w, r, "delete user", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
