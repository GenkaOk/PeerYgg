package yggconf

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// validateTimeout bounds how long the daemon may take to normalise a config.
// A hung yggdrasil must not hang an install.
const validateTimeout = 10 * time.Second

// lookPath and runCommand are test seams: tests replace them to drive Validate
// without a real yggdrasil binary.
var (
	lookPath   = exec.LookPath
	runCommand = runCommandExec
)

// runCommandExec runs the daemon and captures its output.
func runCommandExec(name string, args ...string) (stdout, stderr []byte, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), validateTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, name, args...)
	var out, errOut bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errOut
	err = cmd.Run()
	return out.Bytes(), errOut.Bytes(), err
}

// Validate asks the yggdrasil daemon to normalise the config at path. checked
// is false when no yggdrasil binary is on PATH: a machine that cannot run the
// daemon has nothing to validate with, which is not an install failure.
//
// The normalised document is compared with the file's own peers as sets,
// because the daemon re-quotes and may reorder them. An empty peers list in
// the file skips the comparison: the daemon may leave an empty array out
// altogether, so the exit code is the only signal left.
func Validate(path string) (checked bool, err error) {
	bin, err := lookPath("yggdrasil")
	if err != nil {
		return false, nil
	}

	stdout, stderr, err := runCommand(bin, "-normaliseconf", "-json", "-useconffile", path)
	if err != nil {
		detail := strings.TrimSpace(string(stderr))
		if detail == "" {
			detail = strings.TrimSpace(string(stdout))
		}
		if detail != "" {
			return true, fmt.Errorf("yggconf: yggdrasil rejected config %s: %w: %s", path, err, detail)
		}
		return true, fmt.Errorf("yggconf: yggdrasil rejected config %s: %w", path, err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return true, sudoHint(fmt.Errorf("yggconf: reading config %s: %w", path, err))
	}
	want, err := parsePeers(data)
	if err != nil {
		return true, fmt.Errorf("yggconf: reading peers from %s: %w", path, err)
	}
	if len(want) == 0 {
		return true, nil
	}

	var normalised struct {
		Peers []string `json:"Peers"`
	}
	if err := json.Unmarshal(stdout, &normalised); err != nil {
		return true, fmt.Errorf("yggconf: decoding normalised config from yggdrasil: %w", err)
	}
	if !samePeerSet(want, normalised.Peers) {
		return true, fmt.Errorf("yggconf: yggdrasil normalised Peers to %v, want %v", normalised.Peers, want)
	}
	return true, nil
}

// samePeerSet reports whether both lists hold the same non-empty peers.
func samePeerSet(a, b []string) bool {
	set := make(map[string]struct{}, len(a))
	for _, s := range a {
		if s = strings.TrimSpace(s); s != "" {
			set[s] = struct{}{}
		}
	}
	other := make(map[string]struct{}, len(b))
	for _, s := range b {
		if s = strings.TrimSpace(s); s != "" {
			other[s] = struct{}{}
		}
	}
	if len(set) != len(other) {
		return false
	}
	for s := range other {
		if _, ok := set[s]; !ok {
			return false
		}
	}
	return true
}
