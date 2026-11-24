package yggconf

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"
)

// sampleConf is a realistic HJSON config: comments, a Listen array, a nested
// InterfacePeers map whose keys include an array, and a Peers array. Only the
// last one may be rewritten.
const sampleConf = `# Managed by hand. Do not edit while yggdrasil runs.
PrivateKey: abc123

Listen:
[
  tcp://0.0.0.0:9001
]

InterfacePeers:
  "eth0":
  [
    tcp://192.0.2.7:1234
  ]

Peers:
[
  tcp://old:443
]
`

// isolatePath replaces PATH with an empty directory so nothing can resolve a
// real yggdrasil daemon; the hooks are what the tests actually drive.
func isolatePath(t *testing.T) {
	t.Helper()
	t.Setenv("PATH", t.TempDir())
}

func stubLookPath(t *testing.T, path string, err error) {
	t.Helper()
	prev := lookPath
	lookPath = func(string) (string, error) { return path, err }
	t.Cleanup(func() { lookPath = prev })
}

func stubRunCommand(t *testing.T, fn func(name string, args ...string) (stdout, stderr []byte, err error)) {
	t.Helper()
	prev := runCommand
	runCommand = fn
	t.Cleanup(func() { runCommand = prev })
}

// dirEntries lists the file names directly inside dir.
func dirEntries(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir %s: %v", dir, err)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}

func TestInstall(t *testing.T) {
	t.Run("replaces_and_backs_up", func(t *testing.T) {
		isolatePath(t)
		dir := t.TempDir()
		conf := filepath.Join(dir, "yggdrasil.conf")
		writeConfig(t, conf, sampleConf, 0o640)
		before, err := os.ReadFile(conf)
		if err != nil {
			t.Fatalf("read original: %v", err)
		}

		stubLookPath(t, "/usr/local/bin/yggdrasil", nil)
		stubRunCommand(t, func(name string, args ...string) ([]byte, []byte, error) {
			// The daemon must be pointed at the temp copy, never the live
			// config, so a rejected candidate leaves the original untouched.
			if last := args[len(args)-1]; last == conf || !strings.Contains(last, ".tmp-") {
				t.Errorf("validated %q, want a temp copy of %q", last, conf)
			}
			return []byte(`{"Peers":["tcp://new:443"]}`), nil, nil
		})

		rep, err := Install(Options{Path: conf, Peers: []string{"tcp://new:443"}})
		if err != nil {
			t.Fatalf("Install: %v", err)
		}

		if rep == nil {
			t.Fatal("Install returned a nil report")
		}
		if rep.Path != conf {
			t.Errorf("Path = %q, want %q", rep.Path, conf)
		}
		if rep.BackupPath == "" {
			t.Error("BackupPath is empty, want a backup")
		}
		if want := []string{"tcp://old:443"}; !reflect.DeepEqual(rep.Old, want) {
			t.Errorf("Old = %v, want %v", rep.Old, want)
		}
		if want := []string{"tcp://new:443"}; !reflect.DeepEqual(rep.Added, want) {
			t.Errorf("Added = %v, want %v", rep.Added, want)
		}
		if want := []string{"tcp://old:443"}; !reflect.DeepEqual(rep.Removed, want) {
			t.Errorf("Removed = %v, want %v", rep.Removed, want)
		}
		if rep.Unchanged || rep.DryRun {
			t.Errorf("Unchanged/DryRun = %v/%v, want false/false", rep.Unchanged, rep.DryRun)
		}
		if !rep.Validated {
			t.Error("Validated = false, want true")
		}

		after, err := os.ReadFile(conf)
		if err != nil {
			t.Fatalf("read installed config: %v", err)
		}
		got := string(after)
		if !strings.Contains(got, `"tcp://new:443"`) {
			t.Errorf("installed config has no new peer:\n%s", got)
		}
		if strings.Contains(got, "tcp://old:443") {
			t.Errorf("installed config still has the old peer:\n%s", got)
		}
		for _, want := range []string{"# Managed by hand.", "Listen:", "tcp://0.0.0.0:9001", "InterfacePeers:", "tcp://192.0.2.7:1234"} {
			if !strings.Contains(got, want) {
				t.Errorf("installed config lost %q:\n%s", want, got)
			}
		}

		backup, err := os.ReadFile(rep.BackupPath)
		if err != nil {
			t.Fatalf("read backup: %v", err)
		}
		if string(backup) != string(before) {
			t.Error("backup is not byte-identical to the original")
		}
		info, err := os.Stat(rep.BackupPath)
		if err != nil {
			t.Fatalf("stat backup: %v", err)
		}
		if perm := info.Mode().Perm(); perm != 0o640 {
			t.Errorf("backup mode = %o, want 640", perm)
		}
		info, err = os.Stat(conf)
		if err != nil {
			t.Fatalf("stat installed config: %v", err)
		}
		if perm := info.Mode().Perm(); perm != 0o640 {
			t.Errorf("installed mode = %o, want 640", perm)
		}

		for _, name := range dirEntries(t, dir) {
			if strings.Contains(name, ".tmp-") {
				t.Errorf("temp file %q left behind", name)
			}
		}
	})

	t.Run("stale_temp_symlink_is_replaced", func(t *testing.T) {
		isolatePath(t)
		dir := t.TempDir()
		conf := filepath.Join(dir, "yggdrasil.conf")
		writeConfig(t, conf, sampleConf, 0o640)
		victim := filepath.Join(dir, "victim")
		writeConfig(t, victim, "do not touch\n", 0o644)
		tmp := conf + ".tmp-" + strconv.Itoa(os.Getpid())
		if err := os.Symlink(victim, tmp); err != nil {
			t.Skipf("symlinks unsupported here: %v", err)
		}
		stubLookPath(t, "/usr/local/bin/yggdrasil", nil)
		stubRunCommand(t, func(string, ...string) ([]byte, []byte, error) {
			return []byte(`{"Peers":["tcp://new:443"]}`), nil, nil
		})

		if _, err := Install(Options{Path: conf, Peers: []string{"tcp://new:443"}}); err != nil {
			t.Fatalf("Install: %v", err)
		}

		gotVictim, err := os.ReadFile(victim)
		if err != nil {
			t.Fatalf("read victim: %v", err)
		}
		if string(gotVictim) != "do not touch\n" {
			t.Errorf("install followed the planted symlink and wrote %q into the victim", gotVictim)
		}
		info, err := os.Stat(victim)
		if err != nil {
			t.Fatalf("stat victim: %v", err)
		}
		if perm := info.Mode().Perm(); perm != 0o644 {
			t.Errorf("victim mode = %o, want 644", perm)
		}
		after, err := os.ReadFile(conf)
		if err != nil {
			t.Fatalf("read config: %v", err)
		}
		if !strings.Contains(string(after), `"tcp://new:443"`) {
			t.Error("config was not updated")
		}
		if _, err := os.Lstat(tmp); !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("temp path still present: %v", err)
		}
	})

	t.Run("windows_rename_exist_replaces_config", func(t *testing.T) {
		isolatePath(t)
		dir := t.TempDir()
		conf := filepath.Join(dir, "yggdrasil.conf")
		writeConfig(t, conf, sampleConf, 0o640)
		stubLookPath(t, "/usr/local/bin/yggdrasil", nil)
		stubRunCommand(t, func(string, ...string) ([]byte, []byte, error) {
			return []byte(`{"Peers":["tcp://new:443"]}`), nil, nil
		})
		prevGoos, prevRename := goos, renameFile
		goos = "windows"
		t.Cleanup(func() { goos, renameFile = prevGoos, prevRename })
		calls := 0
		renameFile = func(oldpath, newpath string) error {
			calls++
			// Mimic Windows: renaming onto an existing destination fails.
			if _, err := os.Stat(newpath); err == nil {
				return &os.LinkError{Op: "rename", Old: oldpath, New: newpath, Err: fs.ErrExist}
			}
			return os.Rename(oldpath, newpath)
		}

		if _, err := Install(Options{Path: conf, Peers: []string{"tcp://new:443"}}); err != nil {
			t.Fatalf("Install: %v", err)
		}
		if calls != 2 {
			t.Errorf("rename called %d times, want 2 (fail on existing, retry after remove)", calls)
		}
		after, err := os.ReadFile(conf)
		if err != nil {
			t.Fatalf("read config: %v", err)
		}
		if !strings.Contains(string(after), `"tcp://new:443"`) {
			t.Errorf("config was not replaced:\n%s", after)
		}
		if strings.Contains(string(after), "tcp://old:443") {
			t.Error("config still has the old peer")
		}
		for _, name := range dirEntries(t, dir) {
			if strings.Contains(name, ".tmp-") {
				t.Errorf("temp file %q left behind", name)
			}
		}
	})

	t.Run("rename_refused_keeps_config_and_names_backup", func(t *testing.T) {
		isolatePath(t)
		dir := t.TempDir()
		conf := filepath.Join(dir, "yggdrasil.conf")
		writeConfig(t, conf, sampleConf, 0o640)
		before, err := os.ReadFile(conf)
		if err != nil {
			t.Fatalf("read original: %v", err)
		}
		stubLookPath(t, "/usr/local/bin/yggdrasil", nil)
		stubRunCommand(t, func(string, ...string) ([]byte, []byte, error) {
			return []byte(`{"Peers":["tcp://new:443"]}`), nil, nil
		})
		prevGoos, prevRename := goos, renameFile
		goos = "windows"
		t.Cleanup(func() { goos, renameFile = prevGoos, prevRename })
		renameFile = func(oldpath, newpath string) error {
			// An unclassified failure (not "destination exists") must never
			// make Install delete the live config.
			return &os.LinkError{Op: "rename", Old: oldpath, New: newpath, Err: errors.New("filter driver rejected the rename")}
		}

		_, installErr := Install(Options{Path: conf, Peers: []string{"tcp://new:443"}})
		if installErr == nil {
			t.Fatal("Install succeeded despite a failing rename")
		}
		after, err := os.ReadFile(conf)
		if err != nil {
			t.Fatalf("read config: %v", err)
		}
		if string(after) != string(before) {
			t.Error("failed rename modified the config")
		}
		backups, err := filepath.Glob(conf + ".bak-*")
		if err != nil {
			t.Fatalf("glob backups: %v", err)
		}
		if len(backups) != 1 {
			t.Fatalf("backups = %v, want exactly one", backups)
		}
		if !strings.Contains(installErr.Error(), backups[0]) {
			t.Errorf("error %q does not name the backup %q", installErr, backups[0])
		}
		for _, name := range dirEntries(t, dir) {
			if strings.Contains(name, ".tmp-") {
				t.Errorf("temp file %q left behind", name)
			}
		}
	})

	t.Run("dry_run_touches_nothing", func(t *testing.T) {
		isolatePath(t)
		dir := t.TempDir()
		conf := filepath.Join(dir, "yggdrasil.conf")
		writeConfig(t, conf, sampleConf, 0o640)
		before, err := os.ReadFile(conf)
		if err != nil {
			t.Fatalf("read original: %v", err)
		}
		stubLookPath(t, "/usr/local/bin/yggdrasil", nil)
		stubRunCommand(t, func(string, ...string) ([]byte, []byte, error) {
			t.Error("dry run ran the daemon")
			return nil, nil, nil
		})

		rep, err := Install(Options{Path: conf, Peers: []string{"tcp://new:443"}, DryRun: true})
		if err != nil {
			t.Fatalf("Install: %v", err)
		}
		if !rep.DryRun {
			t.Error("DryRun = false, want true")
		}
		if rep.BackupPath != "" {
			t.Errorf("BackupPath = %q, want empty", rep.BackupPath)
		}
		after, err := os.ReadFile(conf)
		if err != nil {
			t.Fatalf("read config: %v", err)
		}
		if string(after) != string(before) {
			t.Error("dry run modified the config")
		}
		for _, name := range dirEntries(t, dir) {
			if strings.Contains(name, ".bak-") {
				t.Errorf("dry run created backup %q", name)
			}
		}
	})

	t.Run("no_change_skips_write", func(t *testing.T) {
		isolatePath(t)
		dir := t.TempDir()
		conf := filepath.Join(dir, "yggdrasil.conf")
		writeConfig(t, conf, sampleConf, 0o640)
		frozen := time.Date(2026, 2, 3, 4, 5, 6, 0, time.UTC)
		if err := os.Chtimes(conf, frozen, frozen); err != nil {
			t.Fatalf("chtimes: %v", err)
		}
		before, err := os.ReadFile(conf)
		if err != nil {
			t.Fatalf("read original: %v", err)
		}
		stubLookPath(t, "/usr/local/bin/yggdrasil", nil)
		stubRunCommand(t, func(string, ...string) ([]byte, []byte, error) {
			t.Error("unchanged config was validated")
			return nil, nil, nil
		})

		rep, err := Install(Options{Path: conf, Peers: []string{"tcp://old:443"}})
		if err != nil {
			t.Fatalf("Install: %v", err)
		}
		if !rep.Unchanged {
			t.Error("Unchanged = false, want true")
		}
		if rep.BackupPath != "" {
			t.Errorf("BackupPath = %q, want empty", rep.BackupPath)
		}
		after, err := os.ReadFile(conf)
		if err != nil {
			t.Fatalf("read config: %v", err)
		}
		if string(after) != string(before) {
			t.Error("unchanged install rewrote the config")
		}
		info, err := os.Stat(conf)
		if err != nil {
			t.Fatalf("stat config: %v", err)
		}
		if !info.ModTime().Equal(frozen) {
			t.Errorf("mtime = %v, want %v", info.ModTime(), frozen)
		}
		for _, name := range dirEntries(t, dir) {
			if strings.Contains(name, ".bak-") {
				t.Errorf("unchanged install created backup %q", name)
			}
		}
	})

	t.Run("validation_failure_aborts", func(t *testing.T) {
		isolatePath(t)
		dir := t.TempDir()
		conf := filepath.Join(dir, "yggdrasil.conf")
		writeConfig(t, conf, sampleConf, 0o640)
		before, err := os.ReadFile(conf)
		if err != nil {
			t.Fatalf("read original: %v", err)
		}
		stubLookPath(t, "/usr/local/bin/yggdrasil", nil)
		stubRunCommand(t, func(string, ...string) ([]byte, []byte, error) {
			return nil, []byte("  unknown field \"Peerss\"  \n"), errors.New("exit status 1")
		})

		rep, err := Install(Options{Path: conf, Peers: []string{"tcp://new:443"}})
		if err == nil {
			t.Fatal("Install accepted a config the daemon rejected")
		}
		// The backup was already written before validation, so the partial
		// report must name it even though the install failed.
		if rep == nil {
			t.Fatal("report = nil, want the partial report naming the backup")
		}
		if rep.BackupPath == "" {
			t.Error("BackupPath is empty, want the backup made before validation")
		} else if _, statErr := os.Stat(rep.BackupPath); statErr != nil {
			t.Errorf("reported backup %q is missing: %v", rep.BackupPath, statErr)
		}
		if !strings.Contains(err.Error(), `unknown field "Peerss"`) {
			t.Errorf("error %q does not quote the daemon's stderr", err)
		}
		after, err := os.ReadFile(conf)
		if err != nil {
			t.Fatalf("read config: %v", err)
		}
		if string(after) != string(before) {
			t.Error("failed validation modified the config")
		}
		for _, name := range dirEntries(t, dir) {
			if strings.Contains(name, ".tmp-") {
				t.Errorf("temp file %q left behind after failed validation", name)
			}
		}
	})

	t.Run("validation_skipped_without_binary", func(t *testing.T) {
		isolatePath(t)
		dir := t.TempDir()
		conf := filepath.Join(dir, "yggdrasil.conf")
		writeConfig(t, conf, sampleConf, 0o640)
		stubLookPath(t, "", errors.New("executable file not found in $PATH"))
		stubRunCommand(t, func(string, ...string) ([]byte, []byte, error) {
			t.Error("daemon ran even though it was not found")
			return nil, nil, nil
		})

		rep, err := Install(Options{Path: conf, Peers: []string{"tcp://new:443"}})
		if err != nil {
			t.Fatalf("Install: %v", err)
		}
		if rep.Validated {
			t.Error("Validated = true, want false when yggdrasil is absent")
		}
		after, err := os.ReadFile(conf)
		if err != nil {
			t.Fatalf("read config: %v", err)
		}
		if !strings.Contains(string(after), `"tcp://new:443"`) {
			t.Error("config was not updated without the daemon")
		}
	})

	t.Run("symlink_writes_resolved_target", func(t *testing.T) {
		isolatePath(t)
		dir := t.TempDir()
		realDir := filepath.Join(dir, "dotfiles")
		if err := os.MkdirAll(realDir, 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		real := filepath.Join(realDir, "yggdrasil.conf")
		writeConfig(t, real, sampleConf, 0o640)
		link := filepath.Join(dir, "yggdrasil.conf")
		if err := os.Symlink(real, link); err != nil {
			t.Skipf("symlinks unsupported here: %v", err)
		}

		stubLookPath(t, "/usr/local/bin/yggdrasil", nil)
		stubRunCommand(t, func(string, ...string) ([]byte, []byte, error) {
			return []byte(`{"Peers":["tcp://new:443"]}`), nil, nil
		})

		rep, err := Install(Options{Path: link, Peers: []string{"tcp://new:443"}})
		if err != nil {
			t.Fatalf("Install: %v", err)
		}
		// EvalSymlinks also resolves symlinks in the parent path (on macOS
		// /var is a link to /private/var), so compare with its own result.
		wantPath, err := filepath.EvalSymlinks(real)
		if err != nil {
			t.Fatalf("eval target: %v", err)
		}
		if rep.Path != wantPath {
			t.Errorf("Path = %q, want the resolved target %q", rep.Path, wantPath)
		}

		info, err := os.Lstat(link)
		if err != nil {
			t.Fatalf("lstat link: %v", err)
		}
		if info.Mode()&os.ModeSymlink == 0 {
			t.Fatal("install replaced the symlink with a regular file")
		}
		after, err := os.ReadFile(real)
		if err != nil {
			t.Fatalf("read target: %v", err)
		}
		if !strings.Contains(string(after), `"tcp://new:443"`) {
			t.Errorf("target config was not updated:\n%s", after)
		}
		if strings.Contains(string(after), "tcp://old:443") {
			t.Error("target config still has the old peer")
		}

		if got := filepath.Dir(rep.BackupPath); got != filepath.Dir(wantPath) {
			t.Errorf("backup dir = %q, want %q (next to the target)", got, filepath.Dir(wantPath))
		}
		backups, err := filepath.Glob(real + ".bak-*")
		if err != nil {
			t.Fatalf("glob backups: %v", err)
		}
		if len(backups) != 1 {
			t.Errorf("backups next to target = %v, want exactly one", backups)
		}
		if links, _ := filepath.Glob(link + ".bak-*"); len(links) != 0 {
			t.Errorf("backup was made next to the symlink: %v", links)
		}
	})

	t.Run("broken_symlink_errors", func(t *testing.T) {
		isolatePath(t)
		dir := t.TempDir()
		link := filepath.Join(dir, "yggdrasil.conf")
		if err := os.Symlink(filepath.Join(dir, "missing.conf"), link); err != nil {
			t.Skipf("symlinks unsupported here: %v", err)
		}

		rep, err := Install(Options{Path: link, Peers: []string{"tcp://new:443"}})
		if err == nil {
			t.Fatal("Install succeeded through a dangling symlink")
		}
		if rep != nil {
			t.Errorf("report = %+v, want nil before any write", rep)
		}
		if !strings.Contains(err.Error(), link) {
			t.Errorf("error %q does not name the symlink %q", err, link)
		}
	})

	t.Run("permission_error_has_sudo_hint", func(t *testing.T) {
		isolatePath(t)
		dir := t.TempDir()
		conf := filepath.Join(dir, "yggdrasil.conf")
		writeConfig(t, conf, sampleConf, 0o444)
		before, err := os.ReadFile(conf)
		if err != nil {
			t.Fatalf("read original: %v", err)
		}
		if err := os.Chmod(dir, 0o555); err != nil {
			t.Fatalf("chmod dir: %v", err)
		}
		t.Cleanup(func() { os.Chmod(dir, 0o700) })
		// Root, and platforms without permission bits, ignore the mode; probe
		// once instead of hard-coding a unix-only euid check.
		if probe := filepath.Join(dir, "probe"); os.WriteFile(probe, nil, 0o600) == nil {
			os.Remove(probe)
			t.Skip("directory permissions are not enforced on this platform/user")
		}
		stubLookPath(t, "/usr/local/bin/yggdrasil", nil)
		stubRunCommand(t, func(string, ...string) ([]byte, []byte, error) {
			t.Error("daemon ran despite an unwritable directory")
			return nil, nil, nil
		})

		_, err = Install(Options{Path: conf, Peers: []string{"tcp://new:443"}})
		if err == nil {
			t.Fatal("Install succeeded without write permission")
		}
		if !strings.Contains(err.Error(), "sudo") {
			t.Errorf("error %q has no sudo hint", err)
		}
		after, err := os.ReadFile(conf)
		if err != nil {
			t.Fatalf("read config: %v", err)
		}
		if string(after) != string(before) {
			t.Error("failed install modified the config")
		}
		for _, name := range dirEntries(t, dir) {
			if strings.Contains(name, ".tmp-") {
				t.Errorf("temp file %q left behind", name)
			}
		}
	})
}

func TestValidate(t *testing.T) {
	t.Run("missing_binary_skips", func(t *testing.T) {
		isolatePath(t)
		conf := filepath.Join(t.TempDir(), "yggdrasil.conf")
		writeConfig(t, conf, sampleConf, 0o600)
		stubLookPath(t, "", errors.New("executable file not found in $PATH"))
		stubRunCommand(t, func(string, ...string) ([]byte, []byte, error) {
			t.Error("daemon ran even though it was not found")
			return nil, nil, nil
		})

		checked, err := Validate(conf)
		if err != nil {
			t.Fatalf("Validate: %v", err)
		}
		if checked {
			t.Error("checked = true, want false when yggdrasil is absent")
		}
	})

	t.Run("compares_peers", func(t *testing.T) {
		isolatePath(t)
		conf := filepath.Join(t.TempDir(), "yggdrasil.conf")
		writeConfig(t, conf, sampleConf, 0o600)
		stubLookPath(t, "/usr/local/bin/yggdrasil", nil)
		stubRunCommand(t, func(name string, args ...string) ([]byte, []byte, error) {
			want := []string{"-normaliseconf", "-json", "-useconffile", conf}
			if !reflect.DeepEqual(args, want) {
				t.Errorf("args = %v, want %v", args, want)
			}
			return []byte(`{"Peers":["tcp://other:443"]}`), nil, nil
		})

		checked, err := Validate(conf)
		if err == nil {
			t.Fatal("Validate accepted a config whose peers changed under normalisation")
		}
		if !checked {
			t.Error("checked = false, want true after the daemon ran")
		}
		if !strings.Contains(err.Error(), "tcp://other:443") {
			t.Errorf("error %q does not name the normalised peer", err)
		}
	})

	t.Run("matching_peers", func(t *testing.T) {
		isolatePath(t)
		conf := filepath.Join(t.TempDir(), "yggdrasil.conf")
		writeConfig(t, conf, sampleConf, 0o600)
		stubLookPath(t, "/usr/local/bin/yggdrasil", nil)
		stubRunCommand(t, func(string, ...string) ([]byte, []byte, error) {
			return []byte(`{"Peers":["tcp://old:443"]}`), nil, nil
		})

		checked, err := Validate(conf)
		if err != nil {
			t.Fatalf("Validate: %v", err)
		}
		if !checked {
			t.Error("checked = false, want true")
		}
	})

	t.Run("exit_error_reports_stderr", func(t *testing.T) {
		isolatePath(t)
		conf := filepath.Join(t.TempDir(), "yggdrasil.conf")
		writeConfig(t, conf, sampleConf, 0o600)
		stubLookPath(t, "/usr/local/bin/yggdrasil", nil)
		stubRunCommand(t, func(string, ...string) ([]byte, []byte, error) {
			return []byte("partial"), []byte("\n fatal: bad config \n"), errors.New("exit status 1")
		})

		checked, err := Validate(conf)
		if err == nil {
			t.Fatal("Validate ignored a non-zero exit")
		}
		if !checked {
			t.Error("checked = false, want true after the daemon ran")
		}
		if !strings.Contains(err.Error(), "fatal: bad config") {
			t.Errorf("error %q does not quote stderr", err)
		}
	})

	t.Run("run_failure_is_not_a_rejection", func(t *testing.T) {
		isolatePath(t)
		conf := filepath.Join(t.TempDir(), "yggdrasil.conf")
		writeConfig(t, conf, sampleConf, 0o600)
		stubLookPath(t, "/usr/local/bin/yggdrasil", nil)
		stubRunCommand(t, func(string, ...string) ([]byte, []byte, error) {
			return nil, nil, &exec.Error{Name: "yggdrasil", Err: exec.ErrNotFound}
		})

		checked, err := Validate(conf)
		if err == nil {
			t.Fatal("Validate ignored a daemon that could not start")
		}
		if checked {
			t.Error("checked = true, want false when the daemon never ran")
		}
		if !strings.Contains(err.Error(), "failed to run") {
			t.Errorf("error %q does not say the daemon failed to run", err)
		}
		if strings.Contains(err.Error(), "rejected config") {
			t.Errorf("error %q blames a config the daemon never read", err)
		}
	})

	t.Run("timeout_is_not_a_rejection", func(t *testing.T) {
		isolatePath(t)
		conf := filepath.Join(t.TempDir(), "yggdrasil.conf")
		writeConfig(t, conf, sampleConf, 0o600)
		stubLookPath(t, "/usr/local/bin/yggdrasil", nil)
		stubRunCommand(t, func(string, ...string) ([]byte, []byte, error) {
			return nil, nil, fmt.Errorf("killed after %s: %w", validateTimeout, context.DeadlineExceeded)
		})

		checked, err := Validate(conf)
		if err == nil {
			t.Fatal("Validate ignored a daemon timeout")
		}
		if checked {
			t.Error("checked = true, want false after a timeout")
		}
		if !strings.Contains(err.Error(), "did not finish") {
			t.Errorf("error %q does not report the timeout", err)
		}
		if strings.Contains(err.Error(), "rejected config") {
			t.Errorf("error %q blames the config for a timeout", err)
		}
	})

	t.Run("stderr_is_truncated", func(t *testing.T) {
		isolatePath(t)
		conf := filepath.Join(t.TempDir(), "yggdrasil.conf")
		writeConfig(t, conf, sampleConf, 0o600)
		stubLookPath(t, "/usr/local/bin/yggdrasil", nil)

		var dump strings.Builder
		for i := 0; i < 30; i++ {
			fmt.Fprintf(&dump, "goroutine %d: panic line\n", i)
		}
		stubRunCommand(t, func(string, ...string) ([]byte, []byte, error) {
			return nil, []byte(dump.String()), errors.New("exit status 2")
		})

		checked, err := Validate(conf)
		if err == nil {
			t.Fatal("Validate ignored a non-zero exit")
		}
		if !checked {
			t.Error("checked = false, want true after the daemon ran")
		}
		if !strings.Contains(err.Error(), "goroutine 0: panic line") {
			t.Errorf("error %q lost the first stderr lines", err)
		}
		if strings.Contains(err.Error(), "goroutine 20") {
			t.Errorf("error %q was not truncated:\n%s", err, err)
		}
		if !strings.Contains(err.Error(), "truncated") {
			t.Errorf("error %q does not mark truncated output", err)
		}
	})

	t.Run("empty_peers_skips_comparison", func(t *testing.T) {
		isolatePath(t)
		conf := filepath.Join(t.TempDir(), "yggdrasil.conf")
		writeConfig(t, conf, "Peers: []\n", 0o600)
		stubLookPath(t, "/usr/local/bin/yggdrasil", nil)
		stubRunCommand(t, func(string, ...string) ([]byte, []byte, error) {
			// A daemon may omit an empty Peers array entirely.
			return []byte(`{"Peers":["tcp://filler:443"]}`), nil, nil
		})

		checked, err := Validate(conf)
		if err != nil {
			t.Fatalf("Validate compared peers against an empty file list: %v", err)
		}
		if !checked {
			t.Error("checked = false, want true")
		}
	})
}
