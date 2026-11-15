package main

import (
	"net/http"

	"ownwave/api/internal/auth"
)

// requireSetupAccess admits admins only. The steps after the first account
// (scan, summary, completing setup) all run as that admin, so nothing here
// is open before an account exists.
func (h *Handler) requireSetupAccess(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
