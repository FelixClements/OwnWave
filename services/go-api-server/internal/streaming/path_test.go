package streaming

import (
	"path/filepath"
	"testing"
)

func TestResolvePathRejectsTraversal(t *testing.T) {
	musicDir := t.TempDir()
	s := New(Config{MusicDir: musicDir})

	_, err := s.ResolvePath("../outside.flac")
	if err == nil {
		t.Fatal("expected error for traversal path")
	}

	safe := filepath.Join("artist", "album", "track.flac")
	resolved, err := s.ResolvePath(safe)
	if err != nil {
		t.Fatalf("ResolvePath safe: %v", err)
	}
	expected := filepath.Join(musicDir, safe)
	if resolved != expected {
		t.Fatalf("got %q want %q", resolved, expected)
	}
}
