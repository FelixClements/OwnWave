// Package testffmpeg provides a fake ffmpeg binary for tests that exercise
// cover art extraction without a real ffmpeg or real audio files.
package testffmpeg

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// JPEG is what the fake writes to stdout for a file with cover art.
const JPEG = "\xff\xd8\xff\xe0fake-jpeg"

// Fake is a shell script standing in for ffmpeg. For `-i <file> ...` it
// reads <file>'s contents:
//
//	"cover" -> writes JPEG to stdout, exits 0
//	"slow"  -> sleeps 10s (for cancellation tests)
//	other   -> exits 1, like ffmpeg on a file without a video stream
type Fake struct {
	Path string
	log  string
}

// New writes the fake into a temp dir. Requires /bin/sh.
func New(t *testing.T) *Fake {
	t.Helper()
	dir := t.TempDir()
	f := &Fake{Path: filepath.Join(dir, "ffmpeg"), log: filepath.Join(dir, "runs.log")}
	script := `#!/bin/sh
echo "$2" >> '` + f.log + `'
case "$(cat "$2")" in
  cover) printf '` + octal(JPEG) + `' ;;
  slow) exec sleep 10 ;;
  *) exit 1 ;;
esac
`
	if err := os.WriteFile(f.Path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return f
}

// Runs returns how many times the fake has been executed.
func (f *Fake) Runs(t *testing.T) int {
	t.Helper()
	b, err := os.ReadFile(f.log)
	if os.IsNotExist(err) {
		return 0
	}
	if err != nil {
		t.Fatal(err)
	}
	return strings.Count(string(b), "\n")
}

// WriteFile creates path with the given contents ("cover", "slow" or
// anything else), creating parent dirs.
func WriteFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}

// octal escapes every byte for printf, so binary output survives the shell.
func octal(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		fmt.Fprintf(&b, "\\%03o", s[i])
	}
	return b.String()
}
