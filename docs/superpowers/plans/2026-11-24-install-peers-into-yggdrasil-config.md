# Install Peers into Yggdrasil Config — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `-install` to PeerYgg so the selected fastest peers are written into the yggdrasil config's `Peers` array, always with a timestamped backup.

**Architecture:** A new `internal/yggconf` package owns path detection, a byte-level HJSON patch of only the `Peers` value, backup creation, optional validation through `yggdrasil -normaliseconf`, and the atomic write. `cmd/peerygg/main.go` calls it after normal output when `-install` is set.

**Tech Stack:** Go 1.20, standard library only (no new dependencies). Tests use `testing` and `t.TempDir()`.

**Spec:** `docs/superpowers/specs/2026-11-24-install-peers-into-yggdrasil-config-design.md`

## Global Constraints

- No new module dependencies: `go.mod` stays dependency-free.
- Only the value of the top-level `Peers` key changes. Comments, key order, indentation style, CRLF endings, `PrivateKey`, `PrivateKeyPath`, `Listen`, `AdminListen`, `InterfacePeers`, `MulticastInterfaces` must all survive byte-for-byte.
- A backup is created before any write; an existing backup is never overwritten.
- No automatic service restart; only a printed command.
- OpenWrt/netifd is unsupported: detect and error out, never write `/tmp/yggdrasil/*.conf`.
- Every peer URI is written double-quoted.
- Config paths: `/etc/yggdrasil/yggdrasil.conf`, `/etc/yggdrasil.conf`, `/opt/etc/yggdrasil.conf`, `/usr/local/etc/yggdrasil.conf`; Windows `%ProgramData%\Yggdrasil\yggdrasil.conf`.
- Restart commands: linux (systemd) `systemctl restart yggdrasil`; darwin `launchctl kickstart -k system/yggdrasil`; freebsd `service yggdrasil restart`; windows `Restart-Service Yggdrasil`; other `rc-service yggdrasil restart` fallback.

## Review Focus

- A `Peers` array written over several lines with a trailing comma and a comment inside it — all comments and the rest of the file must survive the patch.
- `InterfacePeers: { "eth0": [ "tls://..." ] }` sits at depth > 0 and must never be treated as the target array.
- Writing without root (file `0640 root:yggdrasil`) — must fail with a sudo hint, leave the config untouched, and leave no temp file behind.
- `yggdrasil` not installed locally — install must still succeed with a warning, not fail.
- `-dry-run` — must not create a backup and must not touch the config.
- Windows CRLF config — line endings preserved.
- Peer URIs with IPv6 brackets and `?key=` queries — quoting keeps them valid.

---

### Task 1: HJSON `Peers` patcher

**Files:**
- Create: `internal/yggconf/patch.go`
- Test: `internal/yggconf/patch_test.go`

**Interfaces:**
- Produces: `func ReplacePeers(data []byte, peers []string) (out []byte, old []string, err error)`

- [ ] **Step 1: Write the failing tests**

Create `internal/yggconf/patch_test.go` with these cases (table-driven, one `t.Run` each):

```go
// multiline_hjson: comments before and inside the array survive, other keys untouched
input := "PrivateKey: abc\n# my note\nPeers: [\n  tcp://old1:443 # fast one\n  tls://old2:443,\n]\nListen: [\"tcp://[::]:0\"]\n"
out, old, err := ReplacePeers([]byte(input), []string{"tcp://new1:443", "tls://[2001:db8::1]:443"})
// err == nil; old == {"tcp://old1:443", "tls://old2:443"}
// out contains "# my note", "# fast one", `Listen: ["tcp://[::]:0"]`, "PrivateKey: abc"
// out contains "\"tcp://new1:443\"" and "\"tls://[2001:db8::1]:443\"" and NOT "old1"

// missing_key: appended at end of file
input := "PrivateKey: abc\nListen: []\n"            // no Peers key
// out ends with a Peers block holding both URIs; prefix "PrivateKey: abc\nListen: []\n" unchanged

// json_input: quoted key, single-line array
input := `{"PrivateKey":"abc","Peers":["tcp://old:443"],"Listen":["tcp://[::]:0"]}`
// old == {"tcp://old:443"}; out is valid JSON with Peers replaced and PrivateKey intact
// (assert by json.Unmarshal into map[string]any)

// crlf: line endings preserved
input := "Peers: [\r\n  tcp://old:443\r\n]\r\n"
// out contains "\r\n"; out does not contain a bare "\n" that replaces it

// empty_array: "Peers: []" is replaced, not duplicated
input := "Peers: []\n"
// out has exactly one occurrence of "Peers"

// commented_key_is_not_a_key
input := "# Peers: [\"tcp://old:443\"]\nListen: []\n"
// err == nil and out still contains the commented line verbatim (Peers is appended, not substituted)

// interface_peers_untouched
input := "Peers: [\"tcp://old:443\"]\nInterfacePeers: {\"eth0\": [\"tls://if1:443\"]}\n"
// out contains `InterfacePeers: {"eth0": ["tls://if1:443"]}` verbatim

// duplicate_key_errors
input := "Peers: [\"tcp://a:443\"]\nPeers: [\"tcp://b:443\"]\n"
// err != nil; out is nil

// empty_peers_slice: writes an empty array, old values returned
input := "Peers: [\"tcp://a:443\"]\n"
// old == {"tcp://a:443"}; out contains "Peers: ["
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/yggconf/ -run TestReplacePeers -v`
Expected: FAIL — `undefined: ReplacePeers`.

- [ ] **Step 3: Implement `ReplacePeers` in `internal/yggconf/patch.go`**

Write a single-pass byte scanner. Track mode (`code`, `dqString`, `sqString`, `lineComment`, `blockComment`) and bracket depth for `{`/`}` and `[`/`]`; `#` and `//` start line comments only in `code`, `/*` starts a block comment. Record top-level (both brace and bracket depth zero) occurrences of a `Peers` key — quoted or bare, matched case-insensitively, followed by optional whitespace and `:`. For each occurrence, find the value's `[` and its matching `]` at equal bracket nesting.

Rules the body must satisfy:
- Zero occurrences: append `\n` if the file does not end with one, then a `Peers: [` block.
- More than one occurrence: return an error naming the count; do not produce output.
- Value is not an array: return an error.
- Old values: split the array body at top-level commas *or* newlines, trim whitespace, strip one layer of matching quotes, drop empty tokens and comment-only lines.
- New block: quote every URI with `"`, indent entries by the indentation of the key's line plus two spaces, closing `]` at the key line's indentation. Terminate entries with `,` if the replaced array body contained a comma outside strings/comments, otherwise newline-separated without commas.
- Preserve the file's line ending: if the input contains `\r\n`, emit `\r\n` inside the block.
- Return `out == nil` on any error.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/yggconf/ -run TestReplacePeers -v`
Expected: PASS, all subtests.

- [ ] **Step 5: Commit**

```bash
git add internal/yggconf/patch.go internal/yggconf/patch_test.go
git commit -m "feat(yggconf): patch Peers array in HJSON config"
```

---

### Task 2: Backup creation

**Files:**
- Create: `internal/yggconf/backup.go`
- Test: `internal/yggconf/backup_test.go`

**Interfaces:**
- Produces: `func CreateBackup(path string) (backupPath string, err error)`

- [ ] **Step 1: Write the failing tests**

```go
// creates_timestamped_sibling: file "yggdrasil.conf" -> "yggdrasil.conf.bak-<ts>" in the same dir,
// content byte-identical to the original, mode equal to the original's permission bits.

// never_overwrites: call CreateBackup twice in the same second (freeze via injecting an
// unexported nowFunc = func() time.Time — set it in the test and restore with t.Cleanup);
// second name ends in "-2"; both files exist and hold their own content.

// preserves_mode: original 0640 -> backup 0640.

// missing_source_errors: CreateBackup on a nonexistent path returns an error and creates nothing.
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/yggconf/ -run TestCreateBackup -v`
Expected: FAIL — `undefined: CreateBackup`.

- [ ] **Step 3: Implement `CreateBackup`**

Read the source bytes and `os.Stat` it. Build `path + ".bak-" + nowFunc().Format("20060102-150405")`; while the name exists, retry with `-2`, `-3`, … `os.WriteFile` with the source's `Perm()` followed by a best-effort `os.Chown` to the source's uid/gid (ignore `EPERM` when not root). Declare `var nowFunc = time.Now` at package level for the test hook. Wrap write errors with the path.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/yggconf/ -run TestCreateBackup -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/yggconf/backup.go internal/yggconf/backup_test.go
git commit -m "feat(yggconf): timestamped config backups"
```

---

### Task 3: Path detection and restart hints

**Files:**
- Create: `internal/yggconf/detect.go`, `internal/yggconf/restart.go`
- Test: `internal/yggconf/detect_test.go`

**Interfaces:**
- Produces:
  - `func Candidates(goos string) []string`
  - `func Detect(explicit string) (string, error)`
  - `func RestartCommand(goos string) (cmd string, known bool)`

- [ ] **Step 1: Write the failing tests**

```go
// candidates_linux: contains "/etc/yggdrasil/yggdrasil.conf" first, then "/etc/yggdrasil.conf";
// also contains "/opt/etc/yggdrasil.conf" and "/usr/local/etc/yggdrasil.conf".
// candidates_windows: single element built from %ProgramData% (set t.Setenv("ProgramData", ...)).
// detect_explicit_wins: explicit path that exists is returned even when a candidate also exists
//   (use a t.TempDir() file and temporarily prepend the temp dir to candidates via
//    an unexported candidateRoots override, or by testing Detect with explicit only).
// detect_missing: explicit nonexistent path -> error mentioning the path.
// detect_none_found: with an override making candidates empty -> error listing probed paths.
// restart_linux/darwin/freebsd/windows: expected command strings from Global Constraints, known == true.
// restart_unknown: "openbsd" -> known == false and a non-empty generic hint.
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/yggconf/ -run 'TestCandidates|TestDetect|TestRestart' -v`
Expected: FAIL — undefined symbols.

- [ ] **Step 3: Implement detection and restart hints**

`Candidates(goos)` switches on `goos`: `windows` → `filepath.Join(os.Getenv("ProgramData"), "Yggdrasil", "yggdrasil.conf")`; `freebsd` → `/usr/local/etc/yggdrasil.conf` plus the generic list; everything else → the four Linux paths in the order listed in Global Constraints. `Detect(explicit)` returns `explicit` when non-empty and existing (error with the path and a `-config` hint when it is not), otherwise the first existing candidate, otherwise an error that lists every probed path and says to pass `-config`. Detect OpenWrt before that error: if `/lib/netifd/proto/yggdrasil.sh` exists, return an error saying yggdrasil is managed through UCI in `/etc/config/network` and PeerYgg will not write it. Keep candidates/`os.Stat` behind unexported package vars (`candidatesFor = Candidates`, `statFile = os.Stat`, `openWrtMarker = "/lib/netifd/proto/yggdrasil.sh"`) so tests can override them.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/yggconf/ -run 'TestCandidates|TestDetect|TestRestart' -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/yggconf/detect.go internal/yggconf/restart.go internal/yggconf/detect_test.go
git commit -m "feat(yggconf): detect yggdrasil config path and restart command"
```

---

### Task 4: Validation and atomic install

**Files:**
- Create: `internal/yggconf/validate.go`, `internal/yggconf/install.go`
- Test: `internal/yggconf/install_test.go`

**Interfaces:**
- Consumes: `ReplacePeers`, `CreateBackup`, `RestartCommand`
- Produces:
  - `func Validate(path string) (checked bool, err error)`
  - `type Options struct { Path string; Peers []string; DryRun bool }`
  - `type Report struct { Path, BackupPath string; Old, Added, Removed []string; Unchanged, DryRun, Validated bool }`
  - `func Install(opts Options) (*Report, error)`

- [ ] **Step 1: Write the failing tests**

Use `t.Setenv("PATH", ...)` and unexported hooks (`lookPath = exec.LookPath`, `runCommand`) so no real daemon is needed.

```go
// install_replaces_and_backs_up: temp config with comments + Listen + InterfacePeers;
// Install(Options{Path: conf, Peers: []string{"tcp://new:443"}}) returns Report with
// BackupPath != "", Old == {"tcp://old:443"}, Added == {"tcp://new:443"},
// Removed == {"tcp://old:443"}; file now holds the new peer; comments and InterfacePeers intact;
// backup content equals the original bytes; backup mode equals original mode.

// dry_run_touches_nothing: DryRun true -> Report.DryRun true, BackupPath == "",
// file bytes unchanged, and no ".bak-" file exists in the dir.

// no_change_skips_write: peers already equal to the config's Peers -> Report.Unchanged true,
// BackupPath == "", file mtime/content unchanged, no backup created.

// validation_failure_aborts: runCommand hook returns an error -> Install returns error,
// config bytes unchanged, and no temp file ("*.tmp-*") remains in the dir.

// validation_skipped_without_binary: lookPath hook returns error -> Install succeeds,
// Report.Validated == false.

// permission_error_has_sudo_hint: make the config read-only (chmod 0444) and make the temp
// rename/write fail (use a directory without write permission) -> error text contains "sudo"
// and the original config is unchanged.

// validate_compares_peers: Validate writes a temp copy, runs the hook, parses the hook's
// JSON output ({"Peers":[...]}) and reports an error when the parsed Peers differ from the
// file's Peers.
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/yggconf/ -run 'TestInstall|TestValidate' -v`
Expected: FAIL — undefined `Install`, `Options`, `Report`, `Validate`.

- [ ] **Step 3: Implement `Validate` in `internal/yggconf/validate.go`**

`LookPath("yggdrasil")` → when missing return `(false, nil)`. Otherwise run `yggdrasil -normaliseconf -json -useconffile <path>`, capture stdout/stderr with a 10s timeout; non-zero exit → error including trimmed stderr. Unmarshal stdout into `struct{ Peers []string \`json:"Peers"\` }` and compare as sets with the peers parsed from `path` via `ReplacePeers`'s old-value extraction (extract a small `parsePeers(data []byte) ([]string, error)` helper in `patch.go` and use it in both places); mismatch → error. Expose `lookPath` and `runCommand` as package vars for tests.

- [ ] **Step 4: Implement `Install` in `internal/yggconf/install.go`**

Flow: read `opts.Path` → `ReplacePeers` → `peer.Diff(old, opts.Peers)` for Added/Removed → if `Added` and `Removed` are empty, return `Report{Unchanged: true}` without writing → if `DryRun`, return the report → `CreateBackup` (error aborts) → write `path + ".tmp-" + strconv.Itoa(os.Getpid())` in the same directory → `Chmod`/`Chown` to match the original stat → `Validate(tmp)` (on failure remove tmp and return the error) → `os.Rename(tmp, path)`; if rename fails on Windows with an existing destination, `os.Remove(path)` then retry once. Any `EACCES`/`EPERM` from read, write, or rename gets wrapped with the hint `run as root (sudo)`. Remove the temp file on every error path. Return the populated `Report` with `Path` and `Validated` set.

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/yggconf/... -v`
Expected: PASS, all tests in the package.

- [ ] **Step 6: Commit**

```bash
git add internal/yggconf/validate.go internal/yggconf/install.go internal/yggconf/patch.go internal/yggconf/install_test.go
git commit -m "feat(yggconf): validate and atomically install peers"
```

---

### Task 5: CLI wiring

**Files:**
- Modify: `internal/config/config.go` (add `Install bool`, `ConfPath string`; flags `-install`, `-config`, `-dry-run`)
- Modify: `cmd/peerygg/main.go` (call `yggconf.Install` after output)
- Test: `cmd/peerygg/main_test.go` (new, for the peer-list helper only)

**Interfaces:**
- Consumes: `yggconf.Detect`, `yggconf.Install`, `yggconf.Options`, `yggconf.Report`, `yggconf.RestartCommand`
- Produces: `func selectedPeers(selected []network.Result) []string` in `package main`

- [ ] **Step 1: Write the failing test**

```go
// selected_peers_preserves_order: two Results -> []string{r0.Peer, r1.Peer} in input order.
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./cmd/peerygg/ -run TestSelectedPeers -v`
Expected: FAIL — undefined `selectedPeers`.

- [ ] **Step 3: Implement the flags and the install step**

In `config.Load`: `install := flag.Bool("install", false, "write selected peers into the yggdrasil config")`, `confPath := flag.String("config", "", "path to the yggdrasil config (default: autodetect)")`, `dryRun := flag.Bool("dry-run", false, "show config changes without writing")`; set `Install: *install`, `ConfPath: getEnv("PEERS_CONFIG", *confPath)`, `DryRun: *dryRun || getEnv("DRY_RUN", "") == "1"`.

In `main`, after `storage.SaveLocal`, when `cfg.Install` is set: resolve the path with `yggconf.Detect(cfg.ConfPath)`, call `yggconf.Install`, then print to stderr — the config path, `backup: <path>` when non-empty, `+N / -M peers`, `already up to date` when `Unchanged`, a `not validated (yggdrasil not found)` line when `!Validated`, and `restart: <cmd>` from `RestartCommand(runtime.GOOS)` (fall back to a generic sentence when `known` is false). On any error print it and `os.Exit(1)` — the already-printed results stay valid.

- [ ] **Step 4: Run the tests and build**

Run: `go test ./... && go build ./...`
Expected: PASS, build succeeds.

- [ ] **Step 5: Manual end-to-end check on a throwaway config**

```bash
printf 'PrivateKey: abc\n# keep me\nPeers: [\n  tcp://127.0.0.1:1,\n]\nListen: ["tcp://[::]:0"]\n' > /tmp/yggtest.conf
./peerygg -n 2 -install -config /tmp/yggtest.conf -output json > /dev/null
cat /tmp/yggtest.conf; ls /tmp/yggtest.conf.bak-*
```

Expected: `# keep me` still present, `Listen` untouched, `Peers` holds the new peers, one `.bak-` file whose content matches the original. Then `-dry-run` leaves the file unchanged and creates no backup.

- [ ] **Step 6: Commit**

```bash
git add internal/config/config.go cmd/peerygg/main.go cmd/peerygg/main_test.go
git commit -m "feat: -install writes selected peers into yggdrasil config"
```

---

### Task 6: Documentation

**Files:**
- Modify: `README.md`, `README.ru.md`

- [ ] **Step 1: Add the flag to both usage blocks**

Insert into the `Usage of PeerYgg` block of each README, in alphabetical order with the existing flags:

```
  -config string
        path to the yggdrasil config (default: autodetect)
  -dry-run
        show config changes without writing
  -install
        write selected peers into the yggdrasil config
```

- [ ] **Step 2: Add an `Install into yggdrasil` section to both READMEs**

English (mirror the structure in Russian): state that `-install` replaces the `Peers` array and nothing else, that a `<config>.bak-<timestamp>` backup is always written first, that the config path is autodetected (listing the per-platform paths from Global Constraints) and overridable with `-config`/`PEERS_CONFIG`, that `-dry-run` previews, that root/sudo is required on packaged installs (`0640`), that yggdrasil is restarted separately with the printed command, and that OpenWrt with the netifd package is not supported because it has no `yggdrasil.conf`.

- [ ] **Step 3: Verify**

Run: `grep -c -- "-install" README.md README.ru.md`
Expected: at least 2 in each file (usage block plus section).

- [ ] **Step 4: Commit**

```bash
git add README.md README.ru.md
git commit -m "docs: document -install for yggdrasil config"
```
