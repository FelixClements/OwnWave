package main

import (
	"encoding/json"
	"errors"
	"net/http"

	"ownwave/api/internal/auth"
)

func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username    string `json:"username"`
		Password    string `json:"password"`
		InviteToken string `json:"invite_token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	token, user, err := h.auth.Register(r.Context(), req.Username, req.Password, req.InviteToken)
	if err != nil {
		switch {
		case errors.Is(err, auth.ErrUsernameTaken), errors.Is(err, auth.ErrRegistrationClosed), errors.Is(err, auth.ErrInvalidInvite), errors.Is(err, auth.ErrForbidden):
			http.Error(w, "forbidden", http.StatusForbidden)
		case errors.Is(err, auth.ErrWeakPassword), errors.Is(err, auth.ErrPasswordTooLong), errors.Is(err, auth.ErrInvalidUsername):
			http.Error(w, err.Error(), http.StatusBadRequest)
		default:
			writeInternalError(w, r, "register", err)
		}
		return
	}

	auth.WriteSessionCookie(w, token, h.cookieSecure)
	writeAuthUser(w, user)
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.Username == "" || req.Password == "" {
		http.Error(w, "username and password required", http.StatusBadRequest)
		return
	}
	if h.logins.Blocked(req.Username) {
		tooManyLogins(w)
		return
	}

	token, user, err := h.auth.Login(r.Context(), req.Username, req.Password)
	if err != nil {
		if errors.Is(err, auth.ErrInvalidCredentials) {
			h.logins.Fail(req.Username)
			http.Error(w, "invalid credentials", http.StatusUnauthorized)
			return
		}
		writeInternalError(w, r, "login", err)
		return
	}
	h.logins.Succeed(req.Username)

	auth.WriteSessionCookie(w, token, h.cookieSecure)
	writeAuthUser(w, user)
}

func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	token := auth.TokenFromRequest(r)
	if err := h.auth.Logout(r.Context(), token); err != nil {
		writeInternalError(w, r, "logout", err)
		return
	}
	auth.ClearSessionCookie(w, h.cookieSecure)
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.UserFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"id":        user.ID,
		"username":  user.Username,
		"email":     user.Email,
		"full_name": user.FullName,
		"is_admin":  user.IsAdmin,
	})
}

func (h *Handler) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.UserFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req struct {
		Email    *string `json:"email"`
		FullName *string `json:"full_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
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
		writeInternalError(w, r, "update profile", err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.UserFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	if req.CurrentPassword == "" || req.NewPassword == "" {
		http.Error(w, "current and new password required", http.StatusBadRequest)
		return
	}

	token, err := h.auth.ChangePassword(r.Context(), user, req.CurrentPassword, req.NewPassword)
	if err != nil {
		if errors.Is(err, auth.ErrInvalidCredentials) {
			http.Error(w, "invalid current password", http.StatusUnauthorized)
			return
		}
		if errors.Is(err, auth.ErrWeakPassword) || errors.Is(err, auth.ErrPasswordTooLong) {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		writeInternalError(w, r, "change password", err)
		return
	}

	auth.WriteSessionCookie(w, token, h.cookieSecure)
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func writeAuthUser(w http.ResponseWriter, user auth.User) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"user": map[string]interface{}{
			"id":       user.ID,
			"username": user.Username,
			"is_admin": user.IsAdmin,
		},
	})
}

// LoginDevice signs an app in: it returns a long-lived device token to
// send as "Authorization: Bearer <token>" (no cookie is set).
func (h *Handler) LoginDevice(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username   string `json:"username"`
		Password   string `json:"password"`
		DeviceName string `json:"device_name"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.Username == "" || req.Password == "" {
		http.Error(w, "username and password required", http.StatusBadRequest)
		return
	}
	if h.logins.Blocked(req.Username) {
		tooManyLogins(w)
		return
	}
	token, user, device, err := h.auth.LoginDevice(r.Context(), req.Username, req.Password, req.DeviceName)
	if err != nil {
		if errors.Is(err, auth.ErrInvalidCredentials) {
			h.logins.Fail(req.Username)
			http.Error(w, "invalid credentials", http.StatusUnauthorized)
			return
		}
		writeInternalError(w, r, "device login", err)
		return
	}
	h.logins.Succeed(req.Username)
	writeJSON(w, http.StatusOK, map[string]any{"token": token, "user": user, "device": device})
}

func (h *Handler) ListDevices(w http.ResponseWriter, r *http.Request) {
	user, ok := h.currentUser(w, r)
	if !ok {
		return
	}
	devices, err := h.auth.ListDevices(r.Context(), user.ID, auth.TokenFromRequest(r))
	if err != nil {
		writeInternalError(w, r, "list devices", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"devices": devices})
}

func (h *Handler) RevokeDevice(w http.ResponseWriter, r *http.Request) {
	user, ok := h.currentUser(w, r)
	if !ok {
		return
	}
	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	if err := h.auth.RevokeDevice(r.Context(), user.ID, id); err != nil {
		if errors.Is(err, auth.ErrDeviceNotFound) {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		writeInternalError(w, r, "revoke device", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// tooManyLogins answers a login for a username that has failed too often.
func tooManyLogins(w http.ResponseWriter) {
	w.Header().Set("Retry-After", "900")
	http.Error(w, "too many failed logins, try again later", http.StatusTooManyRequests)
}
