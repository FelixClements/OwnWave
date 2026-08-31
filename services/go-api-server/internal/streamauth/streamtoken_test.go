package streamauth

import (
	"testing"

	"github.com/golang-jwt/jwt/v5"
)

func TestGetString(t *testing.T) {
	claims := jwt.MapClaims{
		"track_id": "abc",
		"number":   float64(42),
	}
	if got := getString(claims, "track_id"); got != "abc" {
		t.Errorf("getString(track_id) = %q, want abc", got)
	}
	if got := getString(claims, "number"); got != "42" {
		t.Errorf("getString(number) = %q, want 42", got)
	}
	if got := getString(claims, "missing"); got != "" {
		t.Errorf("getString(missing) = %q, want empty", got)
	}
}

func TestTrackTokenBindsToTrackID(t *testing.T) {
	tokens := New([]byte("test-secret-key-with-enough-length!!"))
	token, err := tokens.SignTrack("track-a", "flac")
	if err != nil {
		t.Fatalf("SignTrack: %v", err)
	}

	trackID, format, err := tokens.ValidateTrack(token)
	if err != nil {
		t.Fatalf("ValidateTrack: %v", err)
	}
	if trackID != "track-a" || format != "flac" {
		t.Fatalf("got track=%q format=%q", trackID, format)
	}
}

func TestStationTokenBindsToStationID(t *testing.T) {
	tokens := New([]byte("test-secret-key-with-enough-length!!"))
	token, err := tokens.SignStation("station-a", "mp3")
	if err != nil {
		t.Fatalf("SignStation: %v", err)
	}

	stationID, format, err := tokens.ValidateStation(token)
	if err != nil {
		t.Fatalf("ValidateStation: %v", err)
	}
	if stationID != "station-a" || format != "mp3" {
		t.Fatalf("got station=%q format=%q", stationID, format)
	}
}
