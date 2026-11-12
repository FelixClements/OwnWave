package analytics

import "testing"

func TestCheckSecret(t *testing.T) {
	cases := []struct {
		secret string
		ok     bool
	}{
		{"", false},
		{"   ", false},
		{"local-analytics-secret-change-me", false},
		{"replace-me-generate-analytics-secret-32ch", false},
		{"CHANGEME", false},
		{"short", false},
		{"0123456789abcdef", true},
		{"9f2c1e6b8a7d4c3b2a1f0e9d8c7b6a5f4e3d2c1b0a9f8e7d6c5b4a3f2e1d0c9b", true},
	}
	for _, tc := range cases {
		err := CheckSecret(tc.secret)
		if (err == nil) != tc.ok {
			t.Errorf("CheckSecret(%q) = %v, want ok=%v", tc.secret, err, tc.ok)
		}
	}
}
