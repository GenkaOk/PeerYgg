package yggconf

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
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
	// CommandContext kills the process on timeout; report the deadline itself
	// so the caller can tell a hang from a config the daemon rejected.
	if ctxErr := ctx.Err(); ctxErr != nil && err != nil {
		err = ctxErr
	}
	return out.Bytes(), errOut.Bytes(), err
}

// maxOutputLines bounds the daemon output quoted in an error. yggdrasil panics
// on a bad config with a goroutine dump; echoing all of it floods a terminal.
const maxOutputLines = 10

// truncateOutput keeps at most maxOutputLines lines and marks what it dropped.
func truncateOutput(s string) string {
	if s == "" {
		return ""
	}
	lines := strings.Split(s, "\n")
	if len(lines) <= maxOutputLines {
		return s
	}
	return strings.Join(lines[:maxOutputLines], "\n") + "\n... (output truncated)"
}

// daemonFailedToRun reports whether err means the daemon never got as far as
// reading the config: exec could not start it, or it was killed by a signal.
// A plain non-zero exit (an *exec.ExitError with a real status) is a config
// rejection instead.
func daemonFailedToRun(err error) bool {
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return true
	}
	var execErr *exec.Error
	if errors.As(err, &execErr) {
		return true
	}
	var pathErr *os.PathError
	if errors.As(err, &pathErr) {
		return true
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode() < 0
	}
	return false
}

// Validate asks the yggdrasil daemon to normalise the config at path. checked
// is false when no yggdrasil binary is on PATH: a machine that cannot run the
// daemon has nothing to validate with, which is not an install failure.
// checked is also false when the daemon failed to run at all (missing binary,
// timeout, killed by a signal); only a daemon that ran and exited non-zero is
// reported as rejecting the config.
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
		detail := truncateOutput(strings.TrimSpace(string(stderr)))
		if detail == "" {
			detail = truncateOutput(strings.TrimSpace(string(stdout)))
		}
		if errors.Is(err, context.DeadlineExceeded) {
			return false, fmt.Errorf("yggconf: yggdrasil did not finish validating config %s within %s: %w", path, validateTimeout, err)
		}
		if daemonFailedToRun(err) {
			if detail != "" {
				return false, fmt.Errorf("yggconf: yggdrasil failed to run while validating config %s: %w: %s", path, err, detail)
			}
			return false, fmt.Errorf("yggconf: yggdrasil failed to run while validating config %s: %w", path, err)
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
