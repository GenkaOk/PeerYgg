// Package yggconf rewrites parts of a yggdrasil daemon configuration file
// in place. The file is HJSON (of which JSON is a subset), so the raw bytes
// are patched instead of decoding and re-encoding the whole document:
// comments, key order and formatting outside the patched value stay intact.
package yggconf

import (
	"bytes"
	"fmt"
	"strings"
)

// ReplacePeers replaces the value of the top-level Peers array with peers,
// writing every URI double-quoted, and returns the resulting document together
// with the peer URIs that were in the array before.
//
// Everything outside the array value is copied verbatim. When the document has
// no top-level Peers key, a Peers block is appended; a document wrapped in a
// root object ({...}) cannot take that trailing block and is an error instead.
// More than one top-level Peers key, or a Peers value that is not an array, is
// also an error; out is nil on any error.
func ReplacePeers(data []byte, peers []string) (out []byte, old []string, err error) {
	rootBrace := rootDepth(data)

	occurrences, err := findPeerArrays(data, rootBrace)
	if err != nil {
		return nil, nil, err
	}

	switch len(occurrences) {
	case 0:
		if rootBrace == 1 {
			// Appending after the root object would leave trailing content
			// outside {} and produce a document that parses as neither JSON
			// nor HJSON. Fail closed instead of writing an invalid config.
			return nil, nil, fmt.Errorf("yggconf: braced root document has no top-level Peers key; refusing to append outside the root object")
		}

		nl := lineEnding(data)
		encoded := append([]byte("Peers: "), buildPeersArray("", peers, nil, commaNone, nl)...)

		buf := make([]byte, 0, len(data)+len(encoded)+len(nl))
		buf = append(buf, data...)
		if !bytes.HasSuffix(buf, []byte("\n")) {
			buf = append(buf, nl...)
		}
		buf = append(buf, encoded...)
		buf = append(buf, nl...)
		return buf, []string{}, nil
	case 1:
		// handled below
	default:
		return nil, nil, fmt.Errorf("yggconf: %d top-level Peers keys found, want at most one", len(occurrences))
	}

	occ := occurrences[0]
	_, comments, hadComma := splitPeersBody(data[occ.open+1 : occ.close])

	old, err = parsePeers(data)
	if err != nil {
		return nil, nil, err
	}

	style := commaNone
	if hadComma {
		style = commaEach
	}
	if rootBrace == 1 {
		// JSON has no newline-separated values and forbids a trailing comma,
		// so separators are required but never follow the last entry.
		style = commaBetween
	}

	encoded := buildPeersArray(occ.indent, peers, comments, style, lineEnding(data))

	buf := make([]byte, 0, len(data)+len(encoded))
	buf = append(buf, data[:occ.open]...)
	buf = append(buf, encoded...)
	buf = append(buf, data[occ.close+1:]...)
	return buf, old, nil
}

// rootDepth returns the brace depth at which top-level keys live: 1 for a JSON
// document wrapped in the root object, 0 for bare HJSON.
func rootDepth(data []byte) int {
	if firstSignificant(data) == '{' {
		return 1
	}
	return 0
}

// parsePeers returns the peer URIs of the document's top-level Peers array,
// unquoted, or an empty list when it has none. It applies the same structural
// rules as ReplacePeers — a non-array value and duplicate top-level Peers keys
// are errors — so peers read here belong to an array ReplacePeers can rewrite.
func parsePeers(data []byte) ([]string, error) {
	occurrences, err := findPeerArrays(data, rootDepth(data))
	if err != nil {
		return nil, err
	}
	switch len(occurrences) {
	case 0:
		return []string{}, nil
	case 1:
		occ := occurrences[0]
		tokens, _, _ := splitPeersBody(data[occ.open+1 : occ.close])
		peers := make([]string, 0, len(tokens))
		for _, tok := range tokens {
			if v := cleanPeerToken(tok); v != "" {
				peers = append(peers, v)
			}
		}
		return peers, nil
	default:
		return nil, fmt.Errorf("yggconf: %d top-level Peers keys found, want at most one", len(occurrences))
	}
}

// peerArray is a top-level Peers key together with the byte range of its
// array value in the source document.
type peerArray struct {
	open   int // offset of '['
	close  int // offset of the matching ']'
	indent string
}

// findPeerArrays locates every top-level Peers key whose value is an array.
// rootBrace is the brace depth at which top-level keys live (0 for HJSON,
// 1 for a JSON document wrapped in the root object).
func findPeerArrays(data []byte, rootBrace int) ([]peerArray, error) {
	var found []peerArray
	brace, bracket := 0, 0

	for i := 0; i < len(data); {
		c := data[i]

		if c == '"' || c == '\'' {
			end := skipString(data, i)
			if brace == rootBrace && bracket == 0 {
				if name, after, ok := readKey(data, i); ok && strings.EqualFold(name, "Peers") {
					occ, next, err := peerArrayAt(data, i, after)
					if err != nil {
						return nil, err
					}
					found = append(found, occ)
					i = next
					continue
				}
			}
			// Skip the whole string so its contents are never read as keys.
			i = end
			continue
		}

		if c == '#' || isLineCommentStart(data, i) {
			i = skipLineComment(data, i)
			continue
		}
		if c == '/' && i+1 < len(data) && data[i+1] == '*' {
			i = skipBlockComment(data, i)
			continue
		}

		switch c {
		case '{':
			brace++
			i++
			continue
		case '}':
			brace--
			i++
			continue
		case '[':
			bracket++
			i++
			continue
		case ']':
			bracket--
			i++
			continue
		}

		if brace == rootBrace && bracket == 0 && isBareKeyStart(c) {
			name, after, ok := readKey(data, i)
			if !ok {
				for i < len(data) && isBareKeyChar(data[i]) {
					i++
				}
				continue
			}
			if !strings.EqualFold(name, "Peers") {
				i = after
				continue
			}
			occ, next, err := peerArrayAt(data, i, after)
			if err != nil {
				return nil, err
			}
			found = append(found, occ)
			i = next
			continue
		}

		i++
	}

	return found, nil
}

// peerArrayAt validates that the value starting at valueStart is an array and
// returns its byte range. next is the offset just past the closing ']'.
func peerArrayAt(data []byte, keyStart, valueStart int) (peerArray, int, error) {
	open := skipSpaceAndComments(data, valueStart)
	if open >= len(data) || data[open] != '[' {
		return peerArray{}, 0, fmt.Errorf("yggconf: top-level Peers value is not an array")
	}
	closeIdx, err := matchBracket(data, open)
	if err != nil {
		return peerArray{}, 0, err
	}
	return peerArray{
		open:   open,
		close:  closeIdx,
		indent: lineIndent(data, keyStart),
	}, closeIdx + 1, nil
}

// matchBracket returns the offset of the ']' matching the '[' at open.
func matchBracket(data []byte, open int) (int, error) {
	depth := 0
	for i := open; i < len(data); {
		c := data[i]
		switch {
		case c == '"' || c == '\'':
			i = skipString(data, i)
		case c == '#' || isLineCommentStart(data, i):
			i = skipLineComment(data, i)
		case c == '/' && i+1 < len(data) && data[i+1] == '*':
			i = skipBlockComment(data, i)
		case c == '[':
			depth++
			i++
		case c == ']':
			depth--
			if depth == 0 {
				return i, nil
			}
			i++
		default:
			i++
		}
	}
	return 0, fmt.Errorf("yggconf: unterminated top-level Peers array")
}

// commaStyle controls how entries of the rewritten array are separated.
type commaStyle int

const (
	commaNone    commaStyle = iota // newline-separated, as in bare HJSON
	commaBetween                   // ',' between entries, never trailing
	commaEach                      // ',' after every entry, as in the source
)

// buildPeersArray renders the replacement array value. Comments recovered from
// the replaced array are kept above the new entries so no user note is lost.
func buildPeersArray(indent string, peers, comments []string, style commaStyle, nl string) []byte {
	if len(peers) == 0 && len(comments) == 0 {
		return []byte("[]")
	}

	entryIndent := indent + "  "
	var buf bytes.Buffer
	buf.WriteString("[")
	buf.WriteString(nl)
	for _, comment := range comments {
		buf.WriteString(entryIndent)
		buf.WriteString(strings.TrimSpace(comment))
		buf.WriteString(nl)
	}
	for idx, peer := range peers {
		buf.WriteString(entryIndent)
		buf.WriteByte('"')
		buf.WriteString(escapePeer(peer))
		buf.WriteByte('"')
		if style == commaEach || (style == commaBetween && idx < len(peers)-1) {
			buf.WriteByte(',')
		}
		buf.WriteString(nl)
	}
	buf.WriteString(indent)
	buf.WriteByte(']')
	return buf.Bytes()
}

// splitPeersBody splits the array body at top-level commas and newlines.
// Line comments inside the body are dropped from the tokens (a trailing
// comment such as "tcp://old:443 # fast" must not leak into the URI) but are
// returned separately so they can be preserved.
func splitPeersBody(body []byte) (tokens [][]byte, comments []string, hadComma bool) {
	var cur []byte
	depth := 0
	flush := func() {
		tokens = append(tokens, append([]byte(nil), cur...))
		cur = cur[:0]
	}

	for i := 0; i < len(body); {
		c := body[i]
		switch {
		case c == '"' || c == '\'':
			end := skipString(body, i)
			cur = append(cur, body[i:end]...)
			i = end
		case c == '#' || isLineCommentStart(body, i):
			end := skipLineComment(body, i)
			if text := strings.TrimSpace(string(body[i:end])); text != "" {
				comments = append(comments, text)
			}
			i = end
		case c == '/' && i+1 < len(body) && body[i+1] == '*':
			end := skipBlockComment(body, i)
			comments = append(comments, strings.TrimSpace(string(body[i:end])))
			i = end
		case c == '[' || c == '{':
			depth++
			cur = append(cur, c)
			i++
		case c == ']' || c == '}':
			depth--
			cur = append(cur, c)
			i++
		case depth == 0 && (c == ',' || c == '\n' || c == '\r'):
			if c == ',' {
				hadComma = true
			}
			flush()
			i++
		default:
			cur = append(cur, c)
			i++
		}
	}
	flush()

	return tokens, comments, hadComma
}

// cleanPeerToken trims a raw array element, strips one layer of matching
// quotes and drops tokens that carry no value.
func cleanPeerToken(tok []byte) string {
	s := strings.TrimSpace(string(tok))
	if len(s) >= 2 {
		q := s[0]
		if (q == '"' || q == '\'') && s[len(s)-1] == q {
			s = strings.TrimSpace(s[1 : len(s)-1])
		}
	}
	return s
}

func escapePeer(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	return strings.ReplaceAll(s, `"`, `\"`)
}

// readKey parses a bare or quoted key at i followed by a colon. valueStart is
// the offset just past the colon.
func readKey(data []byte, i int) (name string, valueStart int, ok bool) {
	if data[i] == '"' || data[i] == '\'' {
		end := skipString(data, i)
		if end <= i+1 || end > len(data) || data[end-1] != data[i] {
			return "", 0, false
		}
		k := skipHorizontalSpace(data, end)
		if k < len(data) && data[k] == ':' {
			return string(data[i+1 : end-1]), k + 1, true
		}
		return "", 0, false
	}

	if !isBareKeyStart(data[i]) {
		return "", 0, false
	}
	j := i
	for j < len(data) && isBareKeyChar(data[j]) {
		j++
	}
	k := skipHorizontalSpace(data, j)
	if k < len(data) && data[k] == ':' {
		return string(data[i:j]), k + 1, true
	}
	return "", 0, false
}

func isBareKeyStart(c byte) bool {
	return c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

func isBareKeyChar(c byte) bool {
	return isBareKeyStart(c) || (c >= '0' && c <= '9') || c == '-' || c == '.'
}

// isLineCommentStart reports whether a "//" comment starts at i. In HJSON "//"
// may start anywhere outside a string, but a URI such as "tls://host" also
// contains it; requiring a separator before the marker keeps unquoted URIs
// (the common case in a Peers array) from being truncated.
func isLineCommentStart(data []byte, i int) bool {
	if i+1 >= len(data) || data[i] != '/' || data[i+1] != '/' {
		return false
	}
	if i == 0 {
		return true
	}
	switch data[i-1] {
	case ' ', '\t', '\r', '\n', ',', '{', '[', '(', '=':
		return true
	}
	return false
}

func skipString(data []byte, i int) int {
	quote := data[i]
	for j := i + 1; j < len(data); j++ {
		switch {
		case quote == '"' && data[j] == '\\':
			j++
		case data[j] == quote:
			return j + 1
		}
	}
	return len(data)
}

func skipLineComment(data []byte, i int) int {
	for i < len(data) && data[i] != '\n' && data[i] != '\r' {
		i++
	}
	return i
}

func skipBlockComment(data []byte, i int) int {
	for j := i + 2; j+1 < len(data); j++ {
		if data[j] == '*' && data[j+1] == '/' {
			return j + 2
		}
	}
	return len(data)
}

func skipSpaceAndComments(data []byte, i int) int {
	for i < len(data) {
		switch {
		case data[i] == ' ' || data[i] == '\t' || data[i] == '\r' || data[i] == '\n':
			i++
		case data[i] == '#' || isLineCommentStart(data, i):
			i = skipLineComment(data, i)
		case data[i] == '/' && i+1 < len(data) && data[i+1] == '*':
			i = skipBlockComment(data, i)
		default:
			return i
		}
	}
	return i
}

func skipHorizontalSpace(data []byte, i int) int {
	for i < len(data) && (data[i] == ' ' || data[i] == '\t') {
		i++
	}
	return i
}

// firstSignificant returns the first byte that is not whitespace or part of a
// comment, or 0 when the document has none.
func firstSignificant(data []byte) byte {
	for i := 0; i < len(data); {
		switch {
		case data[i] == ' ' || data[i] == '\t' || data[i] == '\r' || data[i] == '\n':
			i++
		case data[i] == '#' || isLineCommentStart(data, i):
			i = skipLineComment(data, i)
		case data[i] == '/' && i+1 < len(data) && data[i+1] == '*':
			i = skipBlockComment(data, i)
		default:
			return data[i]
		}
	}
	return 0
}

// lineIndent returns the whitespace prefix of the line containing pos.
func lineIndent(data []byte, pos int) string {
	start := bytes.LastIndexByte(data[:pos], '\n') + 1
	end := start
	for end < pos && (data[end] == ' ' || data[end] == '\t') {
		end++
	}
	return string(data[start:end])
}

func lineEnding(data []byte) string {
	if bytes.Contains(data, []byte("\r\n")) {
		return "\r\n"
	}
	return "\n"
}
