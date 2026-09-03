package main

import (
	"context"
	"net/http"

	"ownwave/api/internal/auth"
)

func (h *Handler) setupOpen(ctx context.Context) bool {
	count, err := h.db.CountUsers(ctx)
	if err != nil {
		return false
	}
	return count == 0
}

func (h *Handler) requireSetupAccess(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if h.setupOpen(r.Context()) {
			next.ServeHTTP(w, r)
			return
		}
		user, ok := auth.UserFromContext(r.Context())
		if !ok {
			user, ok = h.auth.UserFromRequest(r)
		}
		if !ok || !user.IsAdmin {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r.WithContext(auth.WithUser(r.Context(), user)))
	})
}
