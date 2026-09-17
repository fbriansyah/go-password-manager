# Milestone 6 — `import-keys`

Status: done
Depends on: Milestone 5 (`export-keys`, docs/milestone-5.md), whose zip layout
this reads. Reuses `internal/crypto.Unlock`, `internal/config`, and the
`askPassword`/`vaultDir` helpers already in `cmd`.

## Goal

`export-keys` produces a zip; nothing reads it back yet. `import-keys` closes
the loop: given that zip, install the Identity and Recipient it carries on a
machine that either has never run `init`, or already has a config pointing at
key paths that are simply empty.

Done means a user can copy a zip from `export-keys` to a new machine, run
`gopm import-keys <zip>`, enter the Master Password once, and immediately run
`gopm` against a Vault that Identity can already decrypt.

## Scope

1. **A `Keys` function in `internal/export`** (or a sibling function in the
   same package — it is the mirror of `Keys` that writes the zip) taking the
   zip's bytes, a `config.Config` to write into, and the Master Password;
   returns the paths it wrote or an error. Verifies before writing, per
   docs/adr/0010.
2. **A `gopm import-keys <zip>` command.** Cobra command in
   `cmd/import_keys.go`, sibling to `export_keys.go`. Positional argument for
   the zip path (`cobra.ExactArgs(1)`); reuses `-d/--directory`.
3. **Manifest and checksum validation.** `format_version` must be `1`;
   `identity_sha256` must match the extracted Identity before the password
   prompt.
4. **A password prompt and an Unlock.** Same `askPassword` as `init` and
   `export-keys`. The unlocked Identity's derived Recipient must match the
   zip's `recipient.pub` exactly.
5. **A fresh-machine path.** If `config.Load(vaultDir)` reports
   `config.ErrNotConfigured`, fall back to `config.Defaults()` and write a new
   `config.yaml`, the same paths `gopm init` would have chosen.
6. **Refuse-and-rollback.** Refuses outright if a keypair already exists at
   the resolved paths; on any later failure, deletes whatever this run wrote.

## Out of scope

- **`--force` / overwriting an existing keypair.** Decided against in
  docs/adr/0010; a user who means to replace a keypair removes the old one
  themselves first.
- **Importing a Vault or Secrets.** Only the two key files and the config
  they need; Secrets are unaffected by this command, exactly as they are
  unaffected by `export-keys`.
- **Downloading or fetching the zip from anywhere** (a URL, a cloud API). The
  zip is a local file path the user already has.
- **Reading an older or newer `format_version`.** `1` is the only version
  `export-keys` has ever produced; support for another version is added when
  one exists.

## Decisions

**Verify in order, before any destination write.** Manifest → checksum →
password → Recipient match. See docs/adr/0010 for the full reasoning.

**Zip entries are read by exact name, never used as paths.** Prevents
zip-slip; the write destinations always come from `config.Config`, never from
the zip's own entry names.

**Fresh-machine import writes its own `config.yaml`,** the same defaults
`gopm init` uses, so `import-keys` alone is enough to set up a new machine —
no throwaway keypair generation needed first.

## What has to change

**`internal/export/import.go`** — new. Something like:

```go
func Keys(zipData []byte, cfg config.Config, masterPassword string) error
```

Opens `zipData` as a `zip.Reader`, looks up the three named entries (missing
`identity.age` or `recipient.pub` is an error regardless of the manifest),
parses and validates `manifest.json`, checks the Identity's SHA-256, calls
`crypto.Unlock(extracted identity bytes via a temp check, masterPassword)` —
likely by writing the Identity to a temp file first since `crypto.Unlock`
reads from a path, or by adding a byte-slice-based verification helper to
`internal/crypto` if that reads more cleanly — compares the derived Recipient
against the extracted `recipient.pub`, then writes both files to
`cfg.PrivateKeyPath`/`cfg.PublicKeyPath` with `os.OpenFile(..., O_CREATE|O_EXCL, 0o600)`
(mirroring `crypto.writeNew`), rolling back on any write failure.

**`cmd/import_keys.go`** — new. Reads the zip file from the positional arg,
resolves `vaultDir()`, tries `config.Load(dir)`, falls back to
`config.Defaults()` on `config.ErrNotConfigured`, refuses if either key path
already exists, prompts for the password, calls into `internal/export`,
writes `config.yaml` when it was freshly resolved via `Defaults`, and prints
the same closing summary `init` prints.

**`cmd/root.go`** — `root.AddCommand(..., importKeysCmd())`.

**Tests** — `internal/export/import_test.go`: round-trips a zip from `Keys`
back through the new import function into a fresh temp config and checks the
files match; wrong password; tampered/truncated Identity (checksum mismatch);
mismatched Recipient (swapped from a different export); missing manifest;
unknown `format_version`; existing keypair at destination refuses without
touching it.

**`README.md`** — a line under "Moving keys to another drive" documenting
`import-keys`.

## Acceptance criteria

- A zip from `gopm export-keys` on machine A, copied to a machine with no
  config at all: `gopm import-keys the.zip` with the correct password creates
  `config.yaml`, `identity.age`, and `recipient.pub` at the default paths,
  and `gopm -d <vault>` against a Vault created on machine A opens normally.
- The same zip against a machine that already has a keypair: `import-keys`
  refuses without touching the existing files.
- A zip with `identity.age` truncated by one byte: refused at the checksum
  step, before any password prompt.
- The correct zip with a wrong Master Password: refused, nothing written.
- A zip assembled by hand from two different exports' `identity.age` and
  `recipient.pub`: refused at the Recipient-match step.
- A zip missing `manifest.json`, or with `format_version: 2`: refused with a
  message naming the reason, nothing written.
- Killing the process (simulated by a forced error) between writing
  `identity.age` and `recipient.pub`: a rerun finds no leftover `identity.age`
  from the failed attempt.
- `go.mod` gains no new dependency.
