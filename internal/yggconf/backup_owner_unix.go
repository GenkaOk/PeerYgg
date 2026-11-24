//go:build unix

package yggconf

import (
	"errors"
	"os"
	"syscall"
)

// preserveOwner gives f the same owner as info. It is best-effort: an
// unprivileged process recreating a root-owned config gets EPERM, which must
// not turn a good backup into a failure.
func preserveOwner(f *os.File, info os.FileInfo) error {
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return nil
	}
	if err := f.Chown(int(st.Uid), int(st.Gid)); err != nil && !errors.Is(err, syscall.EPERM) {
		return err
	}
	return nil
}
