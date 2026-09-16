# Milestone 5 — `export-keys`

Status: done
Depends on: nothing new; reuses `internal/crypto.Unlock`, `internal/config`,
and the `askPassword`/vault-dir helpers already in `cmd`.

## Goal

The Identity and the Recipient live under `os.UserConfigDir()`, one copy per
machine. Moving to another machine, or keeping a backup on another drive,
today means finding and copying two files by hand. `export-keys` bundles both
into one zip a user can drag anywhere.

Done means a user can run `gopm export-keys`, enter the Master Password once
to prove the Identity is not corrupt, and get a single timestamped zip file
containing the Identity, the Recipient, and a manifest — ready to copy to a
USB drive, another machine, or a cloud folder.

## Scope

1. **A `internal/export` package.** One function that takes a `config.Config`
   and an `io.Writer`, verifies the Identity by unlocking it, and writes a
   zip archive containing `identity.age`, `recipient.pub`, and
   `manifest.json` to the writer. Knows nothing about Cobra, flags, or stdout.
2. **A `manifest.json` inside the zip.** `format_version` (starts at `1`),
   `app` (`"gopm"`), `exported_at` (RFC 3339), and `identity_sha256` (hex
   SHA-256 of the Identity file's bytes, taken before encryption is touched —
   it hashes the file on disk, not any decrypted content).
3. **A `gopm export-keys` command.** Cobra command in `cmd/export_keys.go`,
   sibling to `init`. Reuses `-d/--directory` (so the Vault-folder override
   in `.gopm.yaml`, docs/adr/0001, is honored) and adds `-o/--output` for the
   destination path.
4. **A timestamped default output path.** `gopm-keys-<YYYYMMDD-HHMMSS>.zip` in
   the current working directory when `-o` is not given.
5. **A password prompt.** The same `askPassword` used by `init`: interactive,
   no echo, refuses a non-TTY. Success unlocks the Identity, which is only a
   correctness check here — the exported Identity file is copied as-is,
   still encrypted with the Master Password.

## Out of scope

- **Importing a zip back.** A separate milestone. This one only produces the
  file; nothing yet reads `manifest.json` or an exported zip.
- **Exporting the Vault.** Secrets stay on disk / in git (docs/adr/0001);
  `export-keys` never touches a `.gopm` file.
- **A second password for the zip itself.** Decided against in
  docs/adr/0009-export-keys-as-a-plain-zip.md — the Identity is already
  protected by the Master Password, and a second password would only add a
  second thing to forget.
- **Non-interactive password input** (`--password-stdin` or similar). Nothing
  today needs `export-keys` in a script; add it when something does.
- **Overwrite confirmation or `--force`.** `-o` pointing at an existing file
  is a hard error; the user removes or renames it themselves.

## Decisions

**The zip is a container, not a second vault.** No new encryption is added;
see docs/adr/0009-export-keys-as-a-plain-zip.md for the full reasoning.

**`export-keys` still needs `-d`.** `config.Load` resolves the Identity and
Recipient paths from the Vault folder's `.gopm.yaml` override when present
(docs/adr/0001), so which keys get exported can depend on which Vault folder
is selected. `export-keys` resolves its config exactly the way `runTUI` does.

**Missing keys is a plain error.** If `config.Load` reports no Identity at the
resolved path, `export-keys` fails with a message pointing at `gopm init`,
the same way `runTUI` already refuses to start without one.

**The output file is written `0600`.** The zip carries the same Identity that
is sensitive on disk; the copy should not be more exposed than the original.

## What has to change

**`internal/export/export.go`** — new. `Keys(cfg config.Config, masterPassword string, w io.Writer) error`:
calls `crypto.Unlock(cfg.PrivateKeyPath, masterPassword)` to verify, reads
both key files, computes the Identity's SHA-256, and writes a
`archive/zip.Writer` over `w` with the three entries.

**`cmd/export_keys.go`** — new. Parses `-o/--output` (default: timestamped
name in the current directory via `time.Now().Format("20060102-150405")`),
resolves `vaultDir()` and `config.Load()` as `runTUI` does, calls
`askPassword`, opens the output file with `os.O_CREATE|os.O_EXCL` and mode
`0600` (so an existing file is refused, not overwritten), calls
`export.Keys`, and on success prints the output path.

**`cmd/root.go`** — `root.AddCommand(exportKeysCmd())` alongside the existing
`initCmd()`/`clipboardClearCmd()`.

**Tests** — `internal/export/export_test.go` exercises `Keys` against a
`bytes.Buffer`: correct password produces a zip with all three entries and a
manifest whose checksum matches; wrong password returns an error and writes
nothing.

**`README.md`** — a line under wherever `init` is documented, describing
`export-keys` and what the zip contains.

## Acceptance criteria

- `gopm export-keys` with the correct Master Password produces
  `gopm-keys-<timestamp>.zip` in the current directory, mode `0600`,
  containing `identity.age`, `recipient.pub`, and `manifest.json`.
- `manifest.json`'s `identity_sha256` matches the SHA-256 of the exported
  `identity.age`.
- A wrong Master Password: `export-keys` exits non-zero, prints an error, and
  writes no zip file.
- `gopm export-keys -o already-exists.zip` where the file exists: exits
  non-zero without touching the existing file's contents.
- `gopm export-keys -d some/other/vault` picks up that Vault folder's
  `.gopm.yaml` override, if any, the same way `gopm -d some/other/vault`
  would when unlocking the TUI.
- Running `export-keys` before `gopm init`: a clear error naming `gopm init`,
  no partial output file left behind.
- `go.mod` gains no new dependency (`archive/zip` and `crypto/sha256` are
  stdlib).
