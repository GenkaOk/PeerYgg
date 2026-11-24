package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/genkaok/PeerYgg/internal/config"
	"github.com/genkaok/PeerYgg/internal/network"
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
