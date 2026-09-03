package streaming

import (
	"os"
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
	expected, err := filepath.EvalSymlinks(musicDir)
	if err != nil {
		expected = musicDir
	}
	expected = filepath.Join(expected, safe)
	if resolved != expected {
		t.Fatalf("got %q want %q", resolved, expected)
	}
}

func TestResolvePathRejectsSymlinkEscape(t *testing.T) {
	musicDir := t.TempDir()
	outside := t.TempDir()
	secret := filepath.Join(outside, "secret.flac")
	if err := os.WriteFile(secret, []byte("flac"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(musicDir, "escape.flac")
	if err := os.Symlink(secret, link); err != nil {
		t.Fatal(err)
	}

	s := New(Config{MusicDir: musicDir})
	if _, err := s.ResolvePath("escape.flac"); err == nil {
		t.Fatal("expected error for symlink pointing outside music dir")
	}
}

func TestResolvePathAllowsSymlinkInsideMusicDir(t *testing.T) {
	musicDir := t.TempDir()
	realFile := filepath.Join(musicDir, "real.flac")
	if err := os.WriteFile(realFile, []byte("flac"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(realFile, filepath.Join(musicDir, "alias.flac")); err != nil {
		t.Fatal(err)
	}

	s := New(Config{MusicDir: musicDir})
	resolved, err := s.ResolvePath("alias.flac")
	if err != nil {
		t.Fatalf("ResolvePath alias: %v", err)
	}
	want, err := filepath.EvalSymlinks(filepath.Join(musicDir, "alias.flac"))
	if err != nil {
		t.Fatal(err)
	}
	if resolved != want {
		t.Fatalf("got %q want %q", resolved, want)
	}
}
