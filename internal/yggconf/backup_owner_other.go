//go:build !unix

package yggconf

import "os"

// preserveOwner is a no-op on platforms without POSIX file ownership.
func preserveOwner(*os.File, os.FileInfo) error { return nil }
