package yggconf

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// freezeTime pins nowFunc for the rest of the test and restores it on cleanup.
func freezeTime(t *testing.T, at time.Time) {
	t.Helper()
	prev := nowFunc
	nowFunc = func() time.Time { return at }
	t.Cleanup(func() { nowFunc = prev })
}

// writeConfig writes content to path with perm, setting the mode explicitly so
// the process umask cannot influence it.
func writeConfig(t *testing.T, path, content string, perm os.FileMode) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), perm); err != nil {
		t.Fatalf("write source %s: %v", path, err)
	}
	if err := os.Chmod(path, perm); err != nil {
		t.Fatalf("chmod source %s: %v", path, err)
	}
}

func TestCreateBackup(t *testing.T) {
	frozen := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	const stamp = "20260102-030405"

	t.Run("creates_timestamped_sibling", func(t *testing.T) {
		freezeTime(t, frozen)

		dir := t.TempDir()
		src := filepath.Join(dir, "yggdrasil.conf")
		content := "PrivateKey: abc\nPeers: []\n"
		writeConfig(t, src, content, 0o640)

		backup, err := CreateBackup(src)
		if err != nil {
			t.Fatalf("CreateBackup() error = %v", err)
		}

		want := src + ".bak-" + stamp
		if backup != want {
			t.Errorf("backup path = %q, want %q", backup, want)
		}
		if got := filepath.Dir(backup); got != dir {
			t.Errorf("backup dir = %q, want %q", got, dir)
		}

		got, err := os.ReadFile(backup)
		if err != nil {
			t.Fatalf("read backup: %v", err)
		}
		if !bytes.Equal(got, []byte(content)) {
			t.Errorf("backup content = %q, want %q", got, content)
		}

		info, err := os.Stat(backup)
		if err != nil {
			t.Fatalf("stat backup: %v", err)
		}
		if perm := info.Mode().Perm(); perm != 0o640 {
			t.Errorf("backup mode = %o, want 640", perm)
		}

		// The source itself must be left untouched.
		if srcData, err := os.ReadFile(src); err != nil {
			t.Fatalf("read source: %v", err)
		} else if string(srcData) != content {
			t.Errorf("source changed to %q, want %q", srcData, content)
		}
	})

	t.Run("never_overwrites", func(t *testing.T) {
		freezeTime(t, frozen)

		dir := t.TempDir()
		src := filepath.Join(dir, "yggdrasil.conf")
		writeConfig(t, src, "first\n", 0o640)

		first, err := CreateBackup(src)
		if err != nil {
			t.Fatalf("first CreateBackup() error = %v", err)
		}

		writeConfig(t, src, "second\n", 0o640)
		second, err := CreateBackup(src)
		if err != nil {
			t.Fatalf("second CreateBackup() error = %v", err)
		}

		if first == second {
			t.Fatalf("second backup reused path %q", first)
		}
		if want := src + ".bak-" + stamp + "-2"; second != want {
			t.Errorf("second backup path = %q, want %q", second, want)
		}

		if got, want := readFile(t, first), "first\n"; got != want {
			t.Errorf("first backup content = %q, want %q", got, want)
		}
		if got, want := readFile(t, second), "second\n"; got != want {
			t.Errorf("second backup content = %q, want %q", got, want)
		}

		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatalf("read dir: %v", err)
		}
		if len(entries) != 3 {
			t.Errorf("dir has %d entries, want 3 (source + two backups): %v", len(entries), entries)
		}
	})

	t.Run("preserves_mode", func(t *testing.T) {
		freezeTime(t, frozen)

		dir := t.TempDir()
		src := filepath.Join(dir, "yggdrasil.conf")
		writeConfig(t, src, "mode\n", 0o640)

		backup, err := CreateBackup(src)
		if err != nil {
			t.Fatalf("CreateBackup() error = %v", err)
		}

		info, err := os.Stat(backup)
		if err != nil {
			t.Fatalf("stat backup: %v", err)
		}
		if perm := info.Mode().Perm(); perm != 0o640 {
			t.Errorf("backup mode = %o, want 640", perm)
		}
	})

	t.Run("missing_source_errors", func(t *testing.T) {
		freezeTime(t, frozen)

		dir := t.TempDir()
		src := filepath.Join(dir, "does-not-exist.conf")

		backup, err := CreateBackup(src)
		if err == nil {
			t.Fatalf("CreateBackup() error = nil, want error; backup path = %q", backup)
		}
		if backup != "" {
			t.Errorf("backup path = %q, want empty on error", backup)
		}

		entries, readErr := os.ReadDir(dir)
		if readErr != nil {
			t.Fatalf("read dir: %v", readErr)
		}
		if len(entries) != 0 {
			t.Errorf("CreateBackup created %d entries for a missing source: %v", len(entries), entries)
		}
	})
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}
