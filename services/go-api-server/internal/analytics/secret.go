package analytics

import (
	"fmt"
	"strings"
)

// MinSecretLen is the shortest ANALYTICS_API_SECRET the server accepts.
const MinSecretLen = 16

// placeholderSecrets are example values from old compose files and
// .env.example. A deployment that still uses one is refused.
var placeholderSecrets = map[string]bool{
	"local-analytics-secret-change-me":          true,
	"replace-me-generate-analytics-secret-32ch": true,
	"changeme":  true,
	"change-me": true,
}

// CheckSecret rejects an empty, placeholder or short shared secret, so the
// service does not start with a value anyone could guess.
func CheckSecret(secret string) error {
	s := strings.TrimSpace(secret)
	switch {
	case s == "":
		return fmt.Errorf("ANALYTICS_API_SECRET is not set; generate one with: openssl rand -hex 32")
	case placeholderSecrets[strings.ToLower(s)]:
		return fmt.Errorf("ANALYTICS_API_SECRET is still the example placeholder; generate one with: openssl rand -hex 32")
	case len(s) < MinSecretLen:
		return fmt.Errorf("ANALYTICS_API_SECRET must be at least %d characters", MinSecretLen)
	}
	return nil
}
