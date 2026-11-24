package yggconf

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strconv"

	"github.com/genkaok/PeerYgg/internal/peer"
)

// goos and renameFile are test seams: tests set goos to "windows" and replace
// renameFile to exercise the remove-then-rename fallback on any platform.
var (
	goos       = runtime.GOOS
	renameFile = os.Rename
)

// Options describes one install of peers into a yggdrasil config.
type Options struct {
	Path   string
	Peers  []string
	DryRun bool
}

// Report describes what Install did, or would do when Options.DryRun is set.
type Report struct {
	Path       string
	BackupPath string
	Old        []string
	Added      []string
	Removed    []string
	Unchanged  bool
	DryRun     bool
	Validated  bool
}

// Install replaces the top-level Peers array of the config at opts.Path with
// opts.Peers, after backing the file up and validating the candidate with the
// daemon when it is installed.
//
// Nothing is written when the peer set is already what opts.Peers asks for, or
// when opts.DryRun is set; the returned Report then describes the change that
// was skipped. Otherwise the new contents go to a temp file in the same
// directory, are validated there, and only then replace the config in one
// rename. Any failure leaves the original config untouched and removes the
// temp file; the backup made before the write stays behind so the user can
// always restore. On failure the returned Report is the partial report, which
// names the backup when one was already made.
//
// opts.Path may be a symlink: the link is resolved first and the read, backup,
// write and rename all target the resolved file, so the link itself survives
// and the report's Path names the file actually written.
func Install(opts Options) (*Report, error) {
	path, err := resolvePath(opts.Path)
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, sudoHint(fmt.Errorf("yggconf: reading config %s: %w", path, err))
	}
	info, err := os.Stat(path)
	if err != nil {
		return nil, sudoHint(fmt.Errorf("yggconf: stat config %s: %w", path, err))
	}

	out, old, err := ReplacePeers(data, opts.Peers)
	if err != nil {
		return nil, err
	}

	added, removed := peer.Diff(old, opts.Peers)
	rep := &Report{
		Path:    path,
		Old:     old,
		Added:   added,
		Removed: removed,
	}
	if len(added) == 0 && len(removed) == 0 {
		rep.Unchanged = true
		return rep, nil
	}
	if opts.DryRun {
		rep.DryRun = true
		return rep, nil
	}

	// The backup comes before any write, so even an interrupted install has a
	// copy of the bytes the user started with.
	backup, err := CreateBackup(path)
	if err != nil {
		return rep, sudoHint(err)
	}
	rep.BackupPath = backup

	tmp := path + ".tmp-" + strconv.Itoa(os.Getpid())
	if err := writeTemp(tmp, out, info); err != nil {
		os.Remove(tmp)
		return rep, sudoHint(err)
	}

	checked, err := Validate(tmp)
	if err != nil {
		os.Remove(tmp)
		return rep, err
	}
	rep.Validated = checked

	if err := renameOver(tmp, path); err != nil {
		os.Remove(tmp)
		// Name the backup in the error: on Windows the fallback can leave the
		// config missing, and then the backup is the only copy left.
		return rep, sudoHint(fmt.Errorf("yggconf: installing config %s (backup at %s): %w", path, rep.BackupPath, err))
	}
	return rep, nil
}

// resolvePath returns the file Install must actually write. A config reached
// through a symlink (dotfiles, nix, a CMS) would be replaced by a regular file
// by rename, leaving the real config with the old peers; resolve the link so
// the read, backup, write and rename all touch its target.
func resolvePath(path string) (string, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return "", sudoHint(fmt.Errorf("yggconf: stat config %s: %w", path, err))
	}
	if info.Mode()&os.ModeSymlink == 0 {
		return path, nil
	}
	target, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", sudoHint(fmt.Errorf("yggconf: resolving symlinked config %s: %w", path, err))
	}
	return target, nil
}

// writeTemp writes data to name, giving the new file the permission bits and,
// best effort, the owner of the config it will replace.
func writeTemp(name string, data []byte, info os.FileInfo) error {
	// O_EXCL refuses to follow a symlink or reuse a file planted at the
	// predictable temp name; only a leftover from a crashed run is reused,
	// after removing it once.
	f, err := os.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if errors.Is(err, fs.ErrExist) {
		if rmErr := os.Remove(name); rmErr != nil && !errors.Is(rmErr, fs.ErrNotExist) {
			return fmt.Errorf("yggconf: removing stale temp config %s: %w", name, rmErr)
		}
		f, err = os.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	}
	if err != nil {
		return fmt.Errorf("yggconf: writing temp config %s: %w", name, err)
	}

	writeErr := func() error {
		if _, err := f.Write(data); err != nil {
			return err
		}
		// OpenFile's mode is masked by the umask; set the source's exact bits.
		if err := f.Chmod(info.Mode().Perm()); err != nil {
			return err
		}
		if err := preserveOwner(f, info); err != nil {
			return err
		}
		// Flush before the rename: a crash must not publish a half-written
		// file under the config's name.
		return f.Sync()
	}()
	if closeErr := f.Close(); writeErr == nil {
		writeErr = closeErr
	}
	if writeErr != nil {
		return fmt.Errorf("yggconf: writing temp config %s: %w", name, writeErr)
	}
	return nil
}

// renameOver atomically replaces path with tmp. POSIX rename replaces an
// existing destination in one step. Windows may refuse to replace an existing
// file: only when the rename error says the destination exists (fs.ErrExist)
// is the destination removed and the rename retried once. Any other error is
// returned without touching the live config, because deleting a file we cannot
// write is not recoverable. The remove-then-rename path has a brief window
// without a config and is not atomic; the backup taken before the write is the
// safety net if the retry fails.
func renameOver(tmp, path string) error {
	err := renameFile(tmp, path)
	if err == nil {
		return nil
	}
	if goos != "windows" || !errors.Is(err, fs.ErrExist) {
		return err
	}
	if removeErr := os.Remove(path); removeErr != nil {
		return fmt.Errorf("removing existing config before rename: %w", removeErr)
	}
	if retryErr := renameFile(tmp, path); retryErr != nil {
		return fmt.Errorf("%s was removed and the new config could not be installed; restore it from the backup: %w", path, retryErr)
	}
	return nil
}

// sudoHint appends the actionable hint to a permission failure, which on a
// system config almost always means the process is not running as root.
func sudoHint(err error) error {
	if err == nil || !isPermissionError(err) {
		return err
	}
	return fmt.Errorf("%w; run as root (sudo)", err)
}

func isPermissionError(err error) bool {
	return errors.Is(err, fs.ErrPermission)
}
