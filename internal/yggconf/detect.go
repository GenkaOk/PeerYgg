package yggconf

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// Candidates returns the config locations to probe, in priority order, for the
// given GOOS. The yggdrasil daemon has no built-in default config path — the
// path comes from the service unit — so PeerYgg guesses the packaged locations.
func Candidates(goos string) []string {
	switch goos {
	case "windows":
		return []string{filepath.Join(os.Getenv("ProgramData"), "Yggdrasil", "yggdrasil.conf")}
	case "freebsd":
		// The FreeBSD port installs to /usr/local/etc; keep the generic
		// locations as fallbacks for hand-built installs.
		return unique(append([]string{"/usr/local/etc/yggdrasil.conf"}, genericConfigPaths...))
	default:
		return append([]string(nil), genericConfigPaths...)
	}
}

// genericConfigPaths are the unix locations shared by the default branch.
var genericConfigPaths = []string{
	"/etc/yggdrasil/yggdrasil.conf",
	"/etc/yggdrasil.conf",
	"/opt/etc/yggdrasil.conf",
	"/usr/local/etc/yggdrasil.conf",
}

// unique removes duplicates while preserving the first occurrence's order.
func unique(paths []string) []string {
	seen := make(map[string]struct{}, len(paths))
	out := make([]string, 0, len(paths))
	for _, p := range paths {
		if _, ok := seen[p]; ok {
			continue
		}
		seen[p] = struct{}{}
		out = append(out, p)
	}
	return out
}

// Seams replaced by tests.
var (
	candidatesFor = Candidates
	statFile      = os.Stat
	openWrtMarker = "/lib/netifd/proto/yggdrasil.sh"
)

// Detect resolves the yggdrasil config path.
//
// An explicit path wins over detection; it must exist. Otherwise the first
// existing candidate from Candidates(runtime.GOOS) is returned. When nothing is
// found, an OpenWrt/netifd install is reported specially — it keeps its
// configuration in UCI (/etc/config/network) and has no yggdrasil.conf, so
// PeerYgg refuses to write anything there. Any other miss lists every probed
// path.
//
// Detection only checks existence, never readability: packaged configs are
// typically 0640 root:yggdrasil and the caller may not be able to open them yet.
func Detect(explicit string) (string, error) {
	if explicit != "" {
		if _, err := statFile(explicit); err != nil {
			return "", fmt.Errorf("yggconf: config %s: %w; pass -config <path> to point at the yggdrasil config", explicit, err)
		}
		return explicit, nil
	}

	probed := candidatesFor(runtime.GOOS)
	for _, path := range probed {
		if _, err := statFile(path); err == nil {
			return path, nil
		}
	}

	if _, err := statFile(openWrtMarker); err == nil {
		return "", fmt.Errorf("yggconf: yggdrasil on OpenWrt is managed through UCI in /etc/config/network; there is no yggdrasil.conf and PeerYgg will not write it")
	}

	return "", fmt.Errorf("yggconf: yggdrasil config not found (probed: %s); pass -config <path> to point at it", strings.Join(probed, ", "))
}
