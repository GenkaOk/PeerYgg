package main

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/genkaok/PeerYgg/internal/config"
	"github.com/genkaok/PeerYgg/internal/network"
	"github.com/genkaok/PeerYgg/internal/yggconf"
)

func TestSelectedPeers(t *testing.T) {
	t.Run("preserves_order", func(t *testing.T) {
		results := []network.Result{
			{Peer: "tcp://first.example:443"},
			{Peer: "tcp://second.example:443"},
		}
		want := []string{"tcp://first.example:443", "tcp://second.example:443"}

		got := selectedPeers(results)
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("selectedPeers(%v) = %v, want %v", results, got, want)
		}
	})

	t.Run("empty", func(t *testing.T) {
		got := selectedPeers(nil)
		if len(got) != 0 {
			t.Fatalf("selectedPeers(nil) = %v, want empty", got)
		}
	})
}

// TestInstallPeersDryRunEmptySelection covers the preview of clearing the peer
// list: a dry run reports the diff instead of refusing, and writes nothing.
func TestInstallPeersDryRunEmptySelection(t *testing.T) {
	path, original := writeTestConfig(t)
	dir := filepath.Dir(path)

	err := installPeers(&config.Config{ConfPath: path, DryRun: true}, nil)
	if err != nil {
		t.Fatalf("installPeers(dry run, no peers) = %v, want nil", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if string(got) != original {
		t.Fatalf("dry run changed the config:\n got: %q\nwant: %q", got, original)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("dry run left %d entries in %s, want only the config: %v", len(entries), dir, entries)
	}
}

// TestInstallPeersRefusesEmptyWrite keeps the refusal for a real write.
func TestInstallPeersRefusesEmptyWrite(t *testing.T) {
	path, original := writeTestConfig(t)

	err := installPeers(&config.Config{ConfPath: path}, nil)
	if err == nil {
		t.Fatal("installPeers(no peers, no dry run) = nil error, want a refusal")
	}

	got, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatalf("read config: %v", readErr)
	}
	if string(got) != original {
		t.Fatalf("refused install changed the config:\n got: %q\nwant: %q", got, original)
	}
}

func TestWriteInstallReport(t *testing.T) {
	t.Run("write", func(t *testing.T) {
		rep := &yggconf.Report{
			Path:       "/etc/yggdrasil.conf",
			BackupPath: "/etc/yggdrasil.conf.bak-20260102-030405",
			Added:      []string{"tcp://a:443", "tcp://b:443"},
			Removed:    []string{"tcp://old:443"},
			Validated:  true,
		}

		var buf bytes.Buffer
		writeInstallReport(&buf, rep)
		got := buf.String()

		restart, _ := yggconf.RestartCommand(runtime.GOOS)
		for _, want := range []string{
			"yggdrasil config: /etc/yggdrasil.conf\n",
			"backup: /etc/yggdrasil.conf.bak-20260102-030405\n",
			"+2 / -1 peers\n",
			"restart: " + restart + "\n",
		} {
			if !strings.Contains(got, want) {
				t.Errorf("report does not contain %q:\n%s", want, got)
			}
		}
		if strings.Contains(got, "dry run") {
			t.Errorf("write report mentions a dry run:\n%s", got)
		}
		if strings.Contains(got, "already up to date") {
			t.Errorf("write report says it was unchanged:\n%s", got)
		}
	})

	t.Run("unchanged", func(t *testing.T) {
		rep := &yggconf.Report{Path: "/etc/yggdrasil.conf", Unchanged: true}

		var buf bytes.Buffer
		writeInstallReport(&buf, rep)
		got := buf.String()

		want := "yggdrasil config: /etc/yggdrasil.conf\nalready up to date\n"
		if got != want {
			t.Errorf("report = %q, want %q", got, want)
		}
		if strings.Contains(got, "restart") {
			t.Errorf("unchanged report prints a restart command:\n%s", got)
		}
	})

	t.Run("dry_run", func(t *testing.T) {
		rep := &yggconf.Report{
			Path:   "/etc/yggdrasil.conf",
			Added:  []string{"tcp://a:443"},
			DryRun: true,
		}

		var buf bytes.Buffer
		writeInstallReport(&buf, rep)
		got := buf.String()

		want := "yggdrasil config: /etc/yggdrasil.conf\n+1 / -0 peers\ndry run: nothing written\n"
		if got != want {
			t.Errorf("report = %q, want %q", got, want)
		}
		if strings.Contains(got, "restart") {
			t.Errorf("dry run prints a restart command:\n%s", got)
		}
	})
}

func TestPrintInstallFailure(t *testing.T) {
	t.Run("names_backup", func(t *testing.T) {
		var buf bytes.Buffer
		printInstallFailure(&buf, &yggconf.Report{BackupPath: "/etc/yggdrasil.conf.bak-1"})
		if got, want := buf.String(), "backup: /etc/yggdrasil.conf.bak-1\n"; got != want {
			t.Errorf("output = %q, want %q", got, want)
		}
	})

	t.Run("without_backup_is_silent", func(t *testing.T) {
		var buf bytes.Buffer
		printInstallFailure(&buf, &yggconf.Report{})
		printInstallFailure(&buf, nil)
		if got := buf.String(); got != "" {
			t.Errorf("output = %q, want empty", got)
		}
	})
}

// writeTestConfig puts a throwaway config with one peer in a temp dir and
// returns its path and contents.
func writeTestConfig(t *testing.T) (path, content string) {
	t.Helper()

	path = filepath.Join(t.TempDir(), "yggdrasil.conf")
	content = "Peers: [\n  tcp://old.example:443,\n]\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path, content
}
