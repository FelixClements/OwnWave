package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRequireUserRejectsMissingToken(t *testing.T) {
	s := &Service{}
	called := false
	handler := s.RequireUser(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	require.Equal(t, http.StatusUnauthorized, rec.Code)
	require.False(t, called)
}

func TestRequireAdminRejectsNonAdmin(t *testing.T) {
	s := &Service{}
	handler := s.RequireAdmin(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestUserFromContext(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	user := User{ID: "1", Username: "admin", IsAdmin: true}
	ctx := WithUser(req.Context(), user)
	got, ok := UserFromContext(ctx)
	require.True(t, ok)
	require.Equal(t, user.ID, got.ID)
}
