package yggconf

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReplacePeers(t *testing.T) {
	t.Run("multiline_hjson", func(t *testing.T) {
		input := "PrivateKey: abc\n# my note\nPeers: [\n  tcp://old1:443 # fast one\n  tls://old2:443,\n]\nListen: [\"tcp://[::]:0\"]\n"

		out, old, err := ReplacePeers([]byte(input), []string{"tcp://new1:443", "tls://[2001:db8::1]:443"})
		if err != nil {
			t.Fatalf("ReplacePeers() error = %v", err)
		}

		if got, want := strings.Join(old, ","), "tcp://old1:443,tls://old2:443"; got != want {
			t.Errorf("old peers = %q, want %q", got, want)
		}

		for _, want := range []string{
			"# my note",
			"# fast one",
			`Listen: ["tcp://[::]:0"]`,
			"PrivateKey: abc",
			`"tcp://new1:443"`,
			`"tls://[2001:db8::1]:443"`,
		} {
			if !strings.Contains(string(out), want) {
				t.Errorf("out does not contain %q:\n%s", want, out)
			}
		}
		for _, gone := range []string{"old1", "old2"} {
			if strings.Contains(string(out), gone) {
				t.Errorf("out still contains old peer %q:\n%s", gone, out)
			}
		}
		// The source body contains a comma, so every entry must be terminated
		// with one (commaEach), not silently switched to newline style.
		assertPeerLinesEndWithComma(t, out, true)
	})

	t.Run("missing_key", func(t *testing.T) {
		input := "PrivateKey: abc\nListen: []\n"

		out, _, err := ReplacePeers([]byte(input), []string{"tcp://new1:443", "tcp://new2:443"})
		if err != nil {
			t.Fatalf("ReplacePeers() error = %v", err)
		}
		if !strings.HasPrefix(string(out), input) {
			t.Errorf("existing prefix changed:\n%s", out)
		}
		if appended := string(out[len(input):]); !strings.HasPrefix(appended, "Peers: [") {
			t.Errorf("appended block does not start with a Peers key:\n%s", out)
		}
		for _, want := range []string{`"tcp://new1:443"`, `"tcp://new2:443"`} {
			if !strings.Contains(string(out), want) {
				t.Errorf("out does not contain %q:\n%s", want, out)
			}
		}
		if !strings.HasSuffix(strings.TrimRight(string(out), "\n"), "]") {
			t.Errorf("out does not end with a Peers array:\n%s", out)
		}
	})

	t.Run("json_input", func(t *testing.T) {
		input := `{"PrivateKey":"abc","Peers":["tcp://old:443"],"Listen":["tcp://[::]:0"]}`

		out, old, err := ReplacePeers([]byte(input), []string{"tcp://new1:443", "tcp://new2:443"})
		if err != nil {
			t.Fatalf("ReplacePeers() error = %v", err)
		}
		if len(old) != 1 || old[0] != "tcp://old:443" {
			t.Errorf("old peers = %v, want [tcp://old:443]", old)
		}

		var parsed map[string]any
		if err := json.Unmarshal(out, &parsed); err != nil {
			t.Fatalf("out is not valid JSON: %v\n%s", err, out)
		}
		if parsed["PrivateKey"] != "abc" {
			t.Errorf("PrivateKey = %v, want abc", parsed["PrivateKey"])
		}
		peers, ok := parsed["Peers"].([]any)
		if !ok || len(peers) != 2 || peers[0] != "tcp://new1:443" || peers[1] != "tcp://new2:443" {
			t.Errorf("Peers = %v, want the two new URIs", parsed["Peers"])
		}
	})

	t.Run("crlf", func(t *testing.T) {
		input := "Peers: [\r\n  tcp://old:443\r\n]\r\n"

		out, _, err := ReplacePeers([]byte(input), []string{"tcp://new:443"})
		if err != nil {
			t.Fatalf("ReplacePeers() error = %v", err)
		}
		if !bytes.Contains(out, []byte("\r\n")) {
			t.Errorf("out lost CRLF line endings:\n%q", out)
		}
		for i, b := range out {
			if b == '\n' && (i == 0 || out[i-1] != '\r') {
				t.Fatalf("out contains a bare LF at offset %d:\n%q", i, out)
			}
		}
	})

	t.Run("empty_array", func(t *testing.T) {
		input := "Peers: []\n"

		out, _, err := ReplacePeers([]byte(input), []string{"tcp://a:443"})
		if err != nil {
			t.Fatalf("ReplacePeers() error = %v", err)
		}
		if got := strings.Count(string(out), "Peers"); got != 1 {
			t.Errorf("out contains %d occurrences of Peers, want 1:\n%s", got, out)
		}
		if !strings.Contains(string(out), `"tcp://a:443"`) {
			t.Errorf("out does not contain the new peer:\n%s", out)
		}
		// "Peers: []" has no comma in its body, so entries stay comma-free.
		assertPeerLinesEndWithComma(t, out, false)
	})

	t.Run("commented_key_is_not_a_key", func(t *testing.T) {
		input := "# Peers: [\"tcp://old:443\"]\nListen: []\n"

		out, _, err := ReplacePeers([]byte(input), []string{"tcp://new:443"})
		if err != nil {
			t.Fatalf("ReplacePeers() error = %v", err)
		}
		if !strings.Contains(string(out), "# Peers: [\"tcp://old:443\"]") {
			t.Errorf("commented Peers line was modified:\n%s", out)
		}
	})

	t.Run("interface_peers_untouched", func(t *testing.T) {
		input := "Peers: [\"tcp://old:443\"]\nInterfacePeers: {\"eth0\": [\"tls://if1:443\"]}\n"

		out, _, err := ReplacePeers([]byte(input), []string{"tcp://new:443"})
		if err != nil {
			t.Fatalf("ReplacePeers() error = %v", err)
		}
		if want := `InterfacePeers: {"eth0": ["tls://if1:443"]}`; !strings.Contains(string(out), want) {
			t.Errorf("out does not contain %q verbatim:\n%s", want, out)
		}
	})

	t.Run("duplicate_key_errors", func(t *testing.T) {
		input := "Peers: [\"tcp://a:443\"]\nPeers: [\"tcp://b:443\"]\n"

		out, old, err := ReplacePeers([]byte(input), []string{"tcp://new:443"})
		if err == nil {
			t.Fatalf("ReplacePeers() error = nil, want error for duplicate keys")
		}
		if out != nil {
			t.Errorf("out = %q, want nil on error", out)
		}
		if old != nil {
			t.Errorf("old = %v, want nil on error", old)
		}
	})

	t.Run("empty_peers_slice", func(t *testing.T) {
		input := "Peers: [\"tcp://a:443\"]\n"

		out, old, err := ReplacePeers([]byte(input), nil)
		if err != nil {
			t.Fatalf("ReplacePeers() error = %v", err)
		}
		if len(old) != 1 || old[0] != "tcp://a:443" {
			t.Errorf("old peers = %v, want [tcp://a:443]", old)
		}
		if !strings.Contains(string(out), "Peers: [") {
			t.Errorf("out does not contain a Peers array:\n%s", out)
		}
		if strings.Contains(string(out), "tcp://a:443") {
			t.Errorf("out still contains the old peer:\n%s", out)
		}
	})

	t.Run("json_root_without_peers_errors", func(t *testing.T) {
		input := `{"PrivateKey":"abc"}`

		out, old, err := ReplacePeers([]byte(input), []string{"tcp://new:443"})
		if err == nil {
			t.Fatalf("ReplacePeers() error = nil, want error for a braced root without Peers")
		}
		if out != nil {
			t.Errorf("out = %q, want nil on error", out)
		}
		if old != nil {
			t.Errorf("old = %v, want nil on error", old)
		}
	})

	t.Run("non_array_value_errors", func(t *testing.T) {
		input := "Peers: \"tcp://x:443\"\n"

		out, old, err := ReplacePeers([]byte(input), []string{"tcp://new:443"})
		if err == nil {
			t.Fatalf("ReplacePeers() error = nil, want error for a non-array Peers value")
		}
		if out != nil {
			t.Errorf("out = %q, want nil on error", out)
		}
		if old != nil {
			t.Errorf("old = %v, want nil on error", old)
		}
	})

	t.Run("unterminated_array_errors", func(t *testing.T) {
		input := "Peers: [\n  \"tcp://x:443\"\n"

		out, old, err := ReplacePeers([]byte(input), []string{"tcp://new:443"})
		if err == nil {
			t.Fatalf("ReplacePeers() error = nil, want error for an unterminated Peers array")
		}
		if out != nil {
			t.Errorf("out = %q, want nil on error", out)
		}
		if old != nil {
			t.Errorf("old = %v, want nil on error", old)
		}
	})
}

// TestReplacePeersGenconf patches a verbatim `yggdrasil -genconf` document: a
// braced root, // comments, Peers: [] and a PrivateKey. Everything outside the
// array must survive byte for byte.
func TestReplacePeersGenconf(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "yggdrasil-genconf.conf"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	before, err := findPeerArrays(data, rootDepth(data))
	if err != nil {
		t.Fatalf("find original Peers: %v", err)
	}
	if len(before) != 1 {
		t.Fatalf("fixture has %d Peers arrays, want 1", len(before))
	}

	peers := []string{"tls://new1.example:443", "tls://new2.example:443"}
	out, old, err := ReplacePeers(data, peers)
	if err != nil {
		t.Fatalf("ReplacePeers() error = %v", err)
	}
	if len(old) != 0 {
		t.Errorf("old peers = %v, want none (fixture starts with Peers: [])", old)
	}

	after, err := findPeerArrays(out, rootDepth(out))
	if err != nil {
		t.Fatalf("find patched Peers: %v", err)
	}
	if len(after) != 1 {
		t.Fatalf("patched document has %d Peers arrays, want 1:\n%s", len(after), out)
	}
	if !bytes.Equal(data[:before[0].open], out[:after[0].open]) {
		t.Error("bytes before the Peers array changed")
	}
	if !bytes.Equal(data[before[0].close+1:], out[after[0].close+1:]) {
		t.Error("bytes after the Peers array changed")
	}

	got, err := parsePeers(out)
	if err != nil {
		t.Fatalf("parsePeers(patched): %v", err)
	}
	if strings.Join(got, ",") != strings.Join(peers, ",") {
		t.Errorf("patched peers = %v, want %v", got, peers)
	}
	for _, want := range []string{"PrivateKey:", "Listen: []", "InterfacePeers: {}", "NodeInfo: {}", "# List of outbound peer connection strings"} {
		if !strings.Contains(string(out), want) {
			t.Errorf("patched document lost %q", want)
		}
	}
}

func TestReplacePeersRejectsNonUTF8(t *testing.T) {
	tests := []struct {
		name   string
		input  []byte
		detail string
	}{
		{
			name:   "utf16le_bom",
			input:  []byte{0xFF, 0xFE, 'P', 0, 'e', 0, 'e', 0, 'r', 0, 's', 0, ':', 0, ' ', 0, '[', 0, ']', 0},
			detail: "UTF-16",
		},
		{
			name:   "utf16be_bom",
			input:  []byte{0xFE, 0xFF, 0, 'P', 0, 'e', 0, 'e', 0, 'r', 0, 's'},
			detail: "UTF-16",
		},
		{
			name:   "nul_byte",
			input:  []byte("Peers: []\x00\n"),
			detail: "NUL",
		},
		{
			name:   "invalid_utf8",
			input:  []byte{'P', 'e', 'e', 'r', 's', ':', ' ', '[', 0xC3, 0x28, ']'},
			detail: "UTF-8",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, old, err := ReplacePeers(tt.input, []string{"tcp://new:443"})
			if err == nil {
				t.Fatal("ReplacePeers() error = nil, want an encoding error")
			}
			if out != nil {
				t.Errorf("out = %q, want nil on error", out)
			}
			if old != nil {
				t.Errorf("old = %v, want nil on error", old)
			}
			if !strings.Contains(err.Error(), tt.detail) {
				t.Errorf("error %q does not name %q", err, tt.detail)
			}
		})
	}
}

func TestReplacePeersUTF8BOM(t *testing.T) {
	bom := []byte{0xEF, 0xBB, 0xBF}
	body := `{"PrivateKey":"abc","Peers":["tcp://old:443"],"Listen":["tcp://[::]:0"]}`
	input := append(append([]byte(nil), bom...), body...)

	out, old, err := ReplacePeers(input, []string{"tcp://new:443"})
	if err != nil {
		t.Fatalf("ReplacePeers() error = %v", err)
	}
	if !bytes.HasPrefix(out, bom) {
		t.Error("out lost the UTF-8 BOM")
	}
	if want := []string{"tcp://old:443"}; strings.Join(old, ",") != strings.Join(want, ",") {
		t.Errorf("old peers = %v, want %v", old, want)
	}
	if got := strings.Count(string(out), "Peers"); got != 1 {
		t.Errorf("out contains %d Peers keys, want 1:\n%s", got, out)
	}

	var parsed map[string]any
	if err := json.Unmarshal(out[len(bom):], &parsed); err != nil {
		t.Fatalf("out without BOM is not valid JSON: %v\n%s", err, out)
	}
	if parsed["PrivateKey"] != "abc" {
		t.Errorf("PrivateKey = %v, want abc", parsed["PrivateKey"])
	}
}

// A single apostrophe in a quoteless HJSON value used to start a "string" that
// ran to EOF, hiding the real Peers array and appending a duplicate block.
func TestReplacePeersApostropheBeforePeers(t *testing.T) {
	input := "NodeInfo: { name: O'Brien }\nPeers: [\n  tcp://old:443\n]\nListen: []\n"

	out, old, err := ReplacePeers([]byte(input), []string{"tcp://new:443"})
	if err != nil {
		t.Fatalf("ReplacePeers() error = %v", err)
	}
	if want := []string{"tcp://old:443"}; strings.Join(old, ",") != strings.Join(want, ",") {
		t.Errorf("old peers = %v, want %v", old, want)
	}
	if got := strings.Count(string(out), "Peers:"); got != 1 {
		t.Errorf("out contains %d Peers keys, want 1:\n%s", got, out)
	}
	if !strings.Contains(string(out), "O'Brien") {
		t.Errorf("out lost the NodeInfo value:\n%s", out)
	}
	if !strings.Contains(string(out), `"tcp://new:443"`) {
		t.Errorf("out does not contain the new peer:\n%s", out)
	}
	if strings.Contains(string(out), "tcp://old:443") {
		t.Errorf("out still contains the old peer:\n%s", out)
	}
}

// TestReplacePeersSafetyNet covers a document the scanner cannot reach: a Peers
// key exists, but only inside a nested map, so the top-level scan finds none.
// Appending a second block would be invisible damage; failing closed is safer.
func TestReplacePeersSafetyNet(t *testing.T) {
	input := "Tunnel: { Peers: [] }\nListen: []\n"

	out, old, err := ReplacePeers([]byte(input), []string{"tcp://new:443"})
	if err == nil {
		t.Fatalf("ReplacePeers() error = nil, want a refusal to append next to a Peers key")
	}
	if out != nil {
		t.Errorf("out = %q, want nil on error", out)
	}
	if old != nil {
		t.Errorf("old = %v, want nil on error", old)
	}
	if !strings.Contains(err.Error(), "Peers") {
		t.Errorf("error %q does not name Peers", err)
	}
}

// assertPeerLinesEndWithComma checks every quoted URI entry line of out for the
// expected comma style.
func assertPeerLinesEndWithComma(t *testing.T, out []byte, want bool) {
	t.Helper()

	checked := 0
	for _, line := range strings.Split(string(out), "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, `"`) {
			continue
		}
		checked++
		if got := strings.HasSuffix(trimmed, ","); got != want {
			t.Errorf("entry line %q ends with comma = %v, want %v:\n%s", line, got, want, out)
		}
	}
	if checked == 0 {
		t.Errorf("no quoted entry lines found in output:\n%s", out)
	}
}
