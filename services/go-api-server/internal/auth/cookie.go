package auth

import (
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	SessionCookieName = "ownwave_session"
	sessionTTL        = 7 * 24 * time.Hour
)

// CookieSecure reports whether the session cookie gets the Secure flag (and
// responses get HSTS). OWNWAVE_COOKIE_SECURE decides when set; when it is
// unset or empty, an https PUBLIC_APP_URL turns it on, so a TLS deployment
// is secure without setting it by hand.
func CookieSecure() bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv("OWNWAVE_COOKIE_SECURE")))
	if v == "" {
		appURL := strings.ToLower(strings.TrimSpace(os.Getenv("PUBLIC_APP_URL")))
		return strings.HasPrefix(appURL, "https://")
	}
	return v == "1" || v == "true" || v == "yes"
}

func WriteSessionCookie(w http.ResponseWriter, token string, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    token,
		Path:     "/",
		MaxAge:   int(sessionTTL.Seconds()),
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
}

func ClearSessionCookie(w http.ResponseWriter, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
}

func TokenFromRequest(r *http.Request) string {
	if c, err := r.Cookie(SessionCookieName); err == nil && c.Value != "" {
		return c.Value
	}
	authHeader := r.Header.Get("Authorization")
	if strings.HasPrefix(authHeader, "Bearer ") {
		return strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))
	}
	return ""
}
