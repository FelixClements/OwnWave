package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestTokenFromRequestPrefersCookie(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: SessionCookieName, Value: "cookie-token"})
	req.Header.Set("Authorization", "Bearer header-token")
	if got := TokenFromRequest(req); got != "cookie-token" {
		t.Fatalf("got %q, want cookie-token", got)
	}
}

func TestTokenFromRequestFallsBackToBearer(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer header-token")
	if got := TokenFromRequest(req); got != "header-token" {
		t.Fatalf("got %q, want header-token", got)
	}
}

func TestTokenFromRequestEmpty(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	if got := TokenFromRequest(req); got != "" {
		t.Fatalf("got %q, want empty", got)
	}
}

func TestWriteSessionCookieFlags(t *testing.T) {
	rec := httptest.NewRecorder()
	WriteSessionCookie(rec, "secret", true)
	cookie := rec.Result().Cookies()[0]
	if !cookie.HttpOnly {
		t.Fatal("expected HttpOnly")
	}
	if !cookie.Secure {
		t.Fatal("expected Secure")
	}
	if cookie.SameSite != http.SameSiteLaxMode {
		t.Fatalf("SameSite = %v", cookie.SameSite)
	}
	if cookie.Path != "/" {
		t.Fatalf("Path = %q", cookie.Path)
	}
}
