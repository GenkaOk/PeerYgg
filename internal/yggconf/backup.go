package yggconf

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"
)

// nowFunc returns the current time. Tests replace it to freeze timestamps
// without sleeping.
var nowFunc = time.Now

// CreateBackup copies the file at path to a timestamped sibling and returns the
// backup's path. The name is "<path>.bak-YYYYMMDD-HHMMSS"; if that name is
// already taken, "-2", "-3", ... is appended until a free name is found. An
// existing file is never overwritten.
//
// The backup is byte-identical to the source and keeps its permission bits.
// Its owner is copied on a best-effort basis, so an unprivileged process can
// still back up a root-owned config. On error the incomplete backup is removed;
// if that removal itself fails, the returned error says so and the path is left
// on disk for the user to inspect.
func CreateBackup(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("yggconf: reading config %s: %w", path, err)
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("yggconf: stat config %s: %w", path, err)
	}

	base := path + ".bak-" + nowFunc().Format("20060102-150405")
	mode := info.Mode().Perm()

	for attempt := 1; ; attempt++ {
		name := base
		if attempt > 1 {
			name = base + "-" + strconv.Itoa(attempt)
		}

		// O_EXCL makes "never overwrite" atomic: a name that appeared after the
		// previous attempt is skipped rather than clobbered.
		f, err := os.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
		if err != nil {
			if errors.Is(err, os.ErrExist) {
				continue
			}
			return "", fmt.Errorf("yggconf: creating backup %s: %w", name, err)
		}

		if err := writeBackup(f, data, mode, info); err != nil {
			f.Close()
			if rmErr := os.Remove(name); rmErr != nil && !errors.Is(rmErr, os.ErrNotExist) {
				return "", fmt.Errorf("yggconf: writing backup %s: %w (incomplete backup left in place: %v)", name, err, rmErr)
			}
			return "", fmt.Errorf("yggconf: writing backup %s: %w", name, err)
		}
		if err := f.Close(); err != nil {
			if rmErr := os.Remove(name); rmErr != nil && !errors.Is(rmErr, os.ErrNotExist) {
				return "", fmt.Errorf("yggconf: writing backup %s: %w (incomplete backup left in place: %v)", name, err, rmErr)
			}
			return "", fmt.Errorf("yggconf: writing backup %s: %w", name, err)
		}
		return name, nil
	}
}

// writeBackup fills the newly created, still-open backup file.
func writeBackup(f *os.File, data []byte, mode os.FileMode, info os.FileInfo) error {
	if _, err := f.Write(data); err != nil {
		return err
	}
	// OpenFile's mode is masked by the process umask, so set the source's exact
	// permission bits afterwards.
	if err := f.Chmod(mode); err != nil {
		return err
	}
	if err := preserveOwner(f, info); err != nil {
		return err
	}
	// Flush before the caller closes: a crash must not leave a backup that
	// exists by name but is missing its bytes.
	return f.Sync()
}
