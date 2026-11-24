package yggconf

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"runtime"
	"strconv"

	"github.com/genkaok/PeerYgg/internal/peer"
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
// always restore.
func Install(opts Options) (*Report, error) {
	data, err := os.ReadFile(opts.Path)
	if err != nil {
		return nil, sudoHint(fmt.Errorf("yggconf: reading config %s: %w", opts.Path, err))
	}
	info, err := os.Stat(opts.Path)
	if err != nil {
		return nil, sudoHint(fmt.Errorf("yggconf: stat config %s: %w", opts.Path, err))
	}

	out, old, err := ReplacePeers(data, opts.Peers)
	if err != nil {
		return nil, err
	}

	added, removed := peer.Diff(old, opts.Peers)
	rep := &Report{
		Path:    opts.Path,
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
	backup, err := CreateBackup(opts.Path)
	if err != nil {
		return nil, sudoHint(err)
	}
	rep.BackupPath = backup

	tmp := opts.Path + ".tmp-" + strconv.Itoa(os.Getpid())
	if err := writeTemp(tmp, out, info); err != nil {
		os.Remove(tmp)
		return nil, sudoHint(err)
	}

	checked, err := Validate(tmp)
	if err != nil {
		os.Remove(tmp)
		return nil, err
	}
	rep.Validated = checked

	if err := renameOver(tmp, opts.Path); err != nil {
		os.Remove(tmp)
		return nil, sudoHint(fmt.Errorf("yggconf: installing config %s: %w", opts.Path, err))
	}
	return rep, nil
}

// writeTemp writes data to name, giving the new file the permission bits and,
// best effort, the owner of the config it will replace.
func writeTemp(name string, data []byte, info os.FileInfo) error {
	f, err := os.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
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
// existing destination in one step. Windows cannot, so it falls back to
// removing the destination first, which is a brief window where the config
// does not exist; every other platform keeps the atomic path.
func renameOver(tmp, path string) error {
	err := os.Rename(tmp, path)
	if err == nil {
		return nil
	}
	if runtime.GOOS != "windows" {
		return err
	}
	// Only the "destination exists" case needs the fallback; removing a
	// destination that is already gone would just destroy the config.
	if _, statErr := os.Stat(path); statErr != nil {
		return err
	}
	if removeErr := os.Remove(path); removeErr != nil {
		return removeErr
	}
	return os.Rename(tmp, path)
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
