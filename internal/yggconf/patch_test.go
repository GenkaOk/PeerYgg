package yggconf

import (
	"bytes"
	"encoding/json"
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
