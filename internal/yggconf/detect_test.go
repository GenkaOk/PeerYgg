package yggconf

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

// stubCandidates replaces candidatesFor for the rest of the test.
func stubCandidates(t *testing.T, paths ...string) {
	t.Helper()
	prev := candidatesFor
	candidatesFor = func(string) []string { return paths }
	t.Cleanup(func() { candidatesFor = prev })
}

// stubStat replaces statFile with a fake that reports exactly the given paths as
// existing. Every other path is reported as not existing.
func stubStat(t *testing.T, existing ...string) {
	t.Helper()
	set := make(map[string]struct{}, len(existing))
	for _, p := range existing {
		set[p] = struct{}{}
	}
	prev := statFile
	statFile = func(name string) (os.FileInfo, error) {
		if _, ok := set[name]; ok {
			return fakeFileInfo{name: filepath.Base(name)}, nil
		}
		return nil, &os.PathError{Op: "stat", Path: name, Err: os.ErrNotExist}
	}
	t.Cleanup(func() { statFile = prev })
}

type fakeFileInfo struct{ name string }

func (f fakeFileInfo) Name() string       { return f.name }
func (f fakeFileInfo) Size() int64        { return 0 }
func (f fakeFileInfo) Mode() os.FileMode  { return 0o644 }
func (f fakeFileInfo) ModTime() time.Time { return time.Time{} }
func (f fakeFileInfo) IsDir() bool        { return false }
func (f fakeFileInfo) Sys() any           { return nil }

func TestCandidates(t *testing.T) {
	generic := []string{
		"/etc/yggdrasil/yggdrasil.conf",
		"/etc/yggdrasil.conf",
		"/opt/etc/yggdrasil.conf",
		"/usr/local/etc/yggdrasil.conf",
	}

	t.Run("candidates_linux", func(t *testing.T) {
		got := Candidates("linux")
		if !reflect.DeepEqual(got, generic) {
			t.Fatalf("Candidates(linux) = %v, want %v", got, generic)
		}
	})

	t.Run("candidates_darwin_generic", func(t *testing.T) {
		got := Candidates("darwin")
		if !reflect.DeepEqual(got, generic) {
			t.Fatalf("Candidates(darwin) = %v, want %v", got, generic)
		}
	})

	t.Run("candidates_windows", func(t *testing.T) {
		dir := t.TempDir()
		t.Setenv("ProgramData", dir)
		want := []string{filepath.Join(dir, "Yggdrasil", "yggdrasil.conf")}
		got := Candidates("windows")
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("Candidates(windows) = %v, want %v", got, want)
		}
	})

	t.Run("candidates_freebsd", func(t *testing.T) {
		got := Candidates("freebsd")
		if len(got) == 0 || got[0] != "/usr/local/etc/yggdrasil.conf" {
			t.Fatalf("Candidates(freebsd)[0] = %v, want /usr/local/etc/yggdrasil.conf", got)
		}
		seen := map[string]bool{}
		for _, p := range got {
			if seen[p] {
				t.Fatalf("Candidates(freebsd) = %v contains duplicate %q", got, p)
			}
			seen[p] = true
		}
		for _, p := range generic {
			if !seen[p] {
				t.Fatalf("Candidates(freebsd) = %v missing %q", got, p)
			}
		}
	})
}

func TestDetect(t *testing.T) {
	marker := openWrtMarker

	t.Run("detect_explicit_wins", func(t *testing.T) {
		dir := t.TempDir()
		explicit := filepath.Join(dir, "yggdrasil.conf")
		if err := os.WriteFile(explicit, []byte("{}"), 0o600); err != nil {
			t.Fatal(err)
		}
		candidate := filepath.Join(dir, "candidate.conf")
		stubCandidates(t, candidate)
		stubStat(t, explicit, candidate, marker)

		got, err := Detect(explicit)
		if err != nil {
			t.Fatalf("Detect(%q) error: %v", explicit, err)
		}
		if got != explicit {
			t.Fatalf("Detect(%q) = %q, want the explicit path", explicit, got)
		}
	})

	t.Run("detect_explicit_real_file", func(t *testing.T) {
		// No stubs: the explicit path is checked with the real os.Stat.
		explicit := filepath.Join(t.TempDir(), "yggdrasil.conf")
		if err := os.WriteFile(explicit, []byte("{}"), 0o600); err != nil {
			t.Fatal(err)
		}

		got, err := Detect(explicit)
		if err != nil {
			t.Fatalf("Detect(%q) error: %v", explicit, err)
		}
		if got != explicit {
			t.Fatalf("Detect(%q) = %q, want the explicit path", explicit, got)
		}
	})

	t.Run("detect_missing_explicit", func(t *testing.T) {
		stubCandidates(t)
		stubStat(t, marker)
		missing := filepath.Join(t.TempDir(), "nope.conf")

		_, err := Detect(missing)
		if err == nil {
			t.Fatal("Detect on missing explicit path returned nil error")
		}
		if !strings.Contains(err.Error(), missing) {
			t.Fatalf("error %q does not name the path %q", err, missing)
		}
		if !strings.Contains(err.Error(), "-config") {
			t.Fatalf("error %q does not mention -config", err)
		}
	})

	t.Run("detect_first_existing_candidate", func(t *testing.T) {
		a := "/probe/a/yggdrasil.conf"
		b := "/probe/b/yggdrasil.conf"
		c := "/probe/c/yggdrasil.conf"
		stubCandidates(t, a, b, c)
		stubStat(t, b, c)

		got, err := Detect("")
		if err != nil {
			t.Fatalf("Detect(\"\") error: %v", err)
		}
		if got != b {
			t.Fatalf("Detect(\"\") = %q, want first existing candidate %q", got, b)
		}
	})

	t.Run("detect_candidate_before_openwrt_marker", func(t *testing.T) {
		candidate := "/probe/yggdrasil.conf"
		stubCandidates(t, candidate)
		stubStat(t, candidate, marker)

		got, err := Detect("")
		if err != nil {
			t.Fatalf("Detect(\"\") error: %v", err)
		}
		if got != candidate {
			t.Fatalf("Detect(\"\") = %q, want %q", got, candidate)
		}
	})

	t.Run("detect_openwrt", func(t *testing.T) {
		stubCandidates(t, "/probe/a/yggdrasil.conf", "/probe/b/yggdrasil.conf")
		stubStat(t, marker)

		_, err := Detect("")
		if err == nil {
			t.Fatal("Detect on OpenWrt returned nil error")
		}
		msg := err.Error()
		if !strings.Contains(msg, "/etc/config/network") || !strings.Contains(msg, "UCI") {
			t.Fatalf("error %q does not explain UCI/network config", msg)
		}
		if !strings.Contains(msg, "not") {
			t.Fatalf("error %q does not say PeerYgg will not write it", msg)
		}
	})

	t.Run("detect_none_found", func(t *testing.T) {
		a := "/probe/a/yggdrasil.conf"
		b := "/probe/b/yggdrasil.conf"
		stubCandidates(t, a, b)
		stubStat(t)

		_, err := Detect("")
		if err == nil {
			t.Fatal("Detect without any config returned nil error")
		}
		msg := err.Error()
		if !strings.Contains(msg, a) || !strings.Contains(msg, b) {
			t.Fatalf("error %q does not list every probed path", msg)
		}
		if !strings.Contains(msg, "-config") {
			t.Fatalf("error %q does not suggest -config", msg)
		}
	})
}

func TestRestartCommand(t *testing.T) {
	tests := []struct {
		goos string
		cmd  string
	}{
		{"linux", "systemctl restart yggdrasil"},
		{"darwin", "launchctl kickstart -k system/yggdrasil"},
		{"freebsd", "service yggdrasil restart"},
		{"windows", "Restart-Service Yggdrasil"},
	}
	for _, tc := range tests {
		t.Run("restart_"+tc.goos, func(t *testing.T) {
			cmd, known := RestartCommand(tc.goos)
			if !known {
				t.Fatalf("RestartCommand(%q) known = false, want true", tc.goos)
			}
			if cmd != tc.cmd {
				t.Fatalf("RestartCommand(%q) = %q, want %q", tc.goos, cmd, tc.cmd)
			}
		})
	}

	t.Run("restart_unknown", func(t *testing.T) {
		cmd, known := RestartCommand("openbsd")
		if known {
			t.Fatalf("RestartCommand(openbsd) known = true, want false")
		}
		if strings.TrimSpace(cmd) == "" {
			t.Fatal("RestartCommand(openbsd) returned an empty hint")
		}
	})
}
