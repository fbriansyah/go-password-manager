# Milestone 7 — `import-secrets`

Status: done
Depends on: nothing new. Reuses `internal/crypto.Unlock`, `internal/vault`,
`internal/secret`, and the `askPassword`/`vaultDir` helpers already in `cmd`.
Independent of Milestone 6 despite the similar name — that one installs keys,
this one installs Secrets.

## Goal

A Vault can only be filled one Secret at a time, through the form. Anyone
arriving from another password manager has to retype hundreds of logins by
hand, which in practice means they never arrive. This milestone opens the
front door: hand `gopm` the file another manager exported, get a Vault.

1Password's CSV export is the first Source Format, because it is the one in
front of us. But the seam is built for the second one: a format contributes
only the knowledge of its own file shape, never any knowledge of Vaults,
slugs, or encryption.

Done means a user can run `gopm import-secrets 1PasswordExport-*.csv`, enter
the Master Password once, and find every login from that export sitting in the
Vault — passwords, usernames, URLs, notes, tags, and TOTP seeds intact, with
duplicate titles resolved rather than refused.

## Scope

1. **A `internal/importer` package.** Shallow formats, deep planning. A
   `Format` knows two things: whether it recognises a file, and how to turn
   that file's bytes into `[]secret.Secret`. It is handed the raw bytes, so
   the package itself never assumes any file shape — a future zip or JSON
   format fits the same seam. Everything else — slug collision
   resolution, validation, writing, the report — lives in the package's
   `Plan`/`Apply` and is written once.
2. **A `onepassword` Source Format.** Recognises the header
   `Title,Url,Username,Password,OTPAuth,Favorite,Archived,Tags,Notes` and maps
   each row to one Secret per the table below. Knows nothing about `vault`.
3. **Header sniffing, with `--from` as an override.** No format claims the
   file → the command refuses and names the formats it knows, rather than
   guessing. `--from=1password` forces one, for an export whose header has
   drifted. Sniffing is strict (an exact header match) while reading is
   tolerant (columns found by name), so the override can actually read the
   drifted files it exists for.
4. **Two phases.** Phase one reads, maps, resolves collisions, and validates
   every Secret in memory; a single unusable row aborts before one file is
   written. Phase two writes. `--dry-run` runs phase one and stops.
5. **A `gopm import-secrets <file>` command.** Cobra command in
   `cmd/import_secrets.go`, sibling to `import_keys.go`. One positional
   argument (`cobra.ExactArgs(1)`); reuses `-d/--directory`; prompts for the
   Master Password with the same `askPassword` — except under `--dry-run`,
   which writes nothing and opens the Vault with `crypto.RecipientOnly`.
6. **A report.** Counts imported, names every title that was shifted to
   resolve a collision, and closes with a reminder that the source file is
   still plaintext on disk.

## The mapping

One CSV row becomes one Secret:

| CSV column | Becomes | Notes |
| --- | --- | --- |
| `Title` | `Meta.Title` | suffixed ` 2`, ` 3`… when the slug collides |
| `Tags` | `Meta.Tags` | split on `;`; `favorite`/`archived` appended when true |
| `Username` | Field `tx` "Username" | |
| `Password` | Field `ps` "Password" | |
| `OTPAuth` | Field `tp` "TOTP" | stored verbatim, per docs/adr/0008 |
| `Url` | Field `tx` "Website" | stored verbatim, never normalised |
| `Notes` | Field `ta` "Notes" | a Field, not `Meta.Description` |

An empty column produces no Field at all. Fields are ordered Username,
Password, TOTP, Website, Notes — the order a login is actually performed, so
the three values most often copied sit at the top of the detail pane.

Measured against the reference export (371 rows): 18 titles collide, 32 rows
carry no Password, 67 no Username, 85 no Url, 35 carry Notes (10 of them
multi-line), 5 carry an `OTPAuth` URI, and 33 carry more than one Tag. Every
one of those shapes is a test case.

## Out of scope

- **A TUI path.** No file picker, no preview screen, no interactive collision
  handling, no new Bindings or Help entries. Import is a once-in-a-lifetime
  event; the CLI is enough. Revisit only if a second format lands and the
  command still feels out of reach.
- **A `ur` (URL) Field Type.** Tempting — a 300-character sign-in URL is ugly
  in the detail pane, and an openable URL would be nice. But adding a Field
  Type touches the registry, the form, the detail pane, and Help. Its own
  milestone, not a side effect of an importer.
- **Any Source Format other than 1Password CSV.** The seam exists so the next
  one is cheap; building it speculatively is not.
- **Export of Secrets.** This is a one-way door for now. `export-keys` moves
  keys, not Secrets, and nothing in this milestone changes that.
- **Deleting, shredding, or moving the source file.** The report names it; the
  user disposes of it.
- **Merging into existing Secrets.** Import only ever creates. A row whose
  title matches a Secret already in the Vault becomes a new, suffixed Secret —
  never an edit of the existing one.
- **1Password's other export shapes** — the 1PUX archive, or CSVs carrying
  custom sections and non-login item types. Only the flat nine-column login
  CSV.

## Decisions

**Notes become a Field, not `Meta.Description`.** CONTEXT.md defines Meta as
describing a Secret and "never used as a credential" — and the reference
export has at least one row carrying a bare password in its Notes column. A
`ta` Field also gives the ten multi-line notes the multi-line editor they
need.

**Colliding titles are suffixed, not refused and not skipped.** 18 collisions
in one ordinary export means refusing the file would send the user to hand-edit
a plaintext CSV of 371 passwords, and skipping would silently drop 21 logins.
The *title* is suffixed rather than only the file name, so what the list shows
matches what is on disk. See docs/adr/0012.

**Everything is validated before anything is written, but a mid-write failure
does not roll back.** The first half follows docs/adr/0010's rule. The second
half departs from it deliberately: `import-keys` rolls back two files, while
this rolls back up to several hundred Secrets that are individually correct —
a delete path whose worst bug destroys real Secrets, in exchange for tidiness.
Stopping and reporting is the safer failure. See docs/adr/0012.

**Collisions are resolved against the Vault's existing slugs too,** read with
`vault.List()`, which needs no decryption — not only against other rows in the
same file.

**The Master Password is required even though it is not technically needed.**
`crypto.RecipientOnly` would be enough to write Secrets, and CONTEXT.md says
so plainly. It is asked for anyway: it matches `export-keys`, `import-keys`,
and `change-master-password`, and an operation that adds hundreds of Secrets
to a Vault should be gated by proof of ownership, not merely by folder access.
`--dry-run` is the exception, from the same reasoning: it adds nothing, so
there is no ownership to prove, and it opens the Vault read-only.
See docs/adr/0012.

**Values are stored exactly as handed over.** No URL normalisation, no
trimming of odd schemes like `httpsipotapp://ipotapp#field-pro`, no rewriting
of `otpauth://` URIs. The same principle as docs/adr/0008.

## What has to change

**`internal/importer/importer.go`** — new. The seam and the planning. Roughly:

```go
type Format interface {
    Name() string                            // "1password"
    Sniff(data []byte) bool
    Read(data []byte) ([]secret.Secret, error)
}

type Plan struct {
    Secrets  []secret.Secret
    Renamed  []Rename                        // {From, To} per shifted title
}

func Names() []string
func FormatNamed(name string) (Format, error)
func Detect(data []byte) (Format, error)
func Build(f Format, data []byte, v vault.Vault) (*Plan, error)
func (p *Plan) Apply(v vault.Vault) (imported int, err error)
```

`Build` is phase one: `Read`, then resolve each title against the slugs already
in `v.List()` plus the slugs claimed by earlier rows in this same file, then
`Validate()` every Secret. Any failure returns an error naming the offending
row, and nothing has touched the Vault. `Apply` is phase two: `v.Create` per
Secret, stopping at the first error and returning how many made it.

**`internal/importer/onepassword.go`** — new. The nine-column mapping, built on
`encoding/csv` (stdlib; `LazyQuotes` off, `FieldsPerRecord` from the header).
Knows `secret` and nothing else.

**`cmd/import_secrets.go`** — new. Resolves `vaultDir()`, `config.Load`,
detects or resolves the format, reads the file, prompts with `askPassword`,
`crypto.Unlock`, `vault.Open`, `importer.Build`. On `--dry-run`, prints the
plan and returns. Otherwise `Apply`, then prints the report.

**`cmd/root.go`** — `root.AddCommand(..., importSecretsCmd())`.

**Tests** — `internal/importer/onepassword_test.go`, with short inline CSV
strings per case in the style of `buildZip` in `internal/export/import_test.go`;
`internal/importer/importer_test.go` for planning and applying, against the
in-memory Vault in `internal/vault/mem.go`. No fixture files, and no data from
a real export.

**`CONTEXT.md`** — the terms **Import** and **Source Format**.

**`docs/adr/0012-importing-secrets-plans-in-full-then-only-adds.md`** — the
decisions above that are not readable from the code.

**`README.md`** — a section after "Moving keys to another drive", and a line in
"Usage".

## Acceptance criteria

- A 1Password CSV of 371 rows imports into an empty Vault as 371 Secrets, and
  `gopm` lists all of them after one unlock.
- Three rows titled `Packtpub` become `Packtpub`, `Packtpub 2`, `Packtpub 3`,
  all three appear in the report's renamed list, and all three files exist.
- Importing the same file twice into the same Vault yields 742 Secrets, the
  second run's titles all suffixed — nothing overwritten, no `ErrSlugTaken`
  reaching the user.
- A row with an `otpauth://totp/...` URI produces a `tp` Field whose detail
  pane shows a live Code; a row with a bare base32 secret does too.
- A row with an empty `Password` produces a Secret with no Password Field, not
  one holding an empty string.
- A row whose Notes span multiple lines produces one `ta` Field containing
  every line.
- A row with `Tags` of `PaperID;PaymentGateway` and `Favorite` of `true`
  produces `Meta.Tags` of `[PaperID PaymentGateway favorite]`.
- A file containing one unparseable TOTP seed imports nothing at all, names the
  offending row, and leaves the Vault byte-for-byte unchanged.
- `--dry-run` on that same good file prints the counts and renames and leaves
  the Vault empty.
- A file whose header is not recognised is refused with a message naming the
  known formats; `--from=1password` on that same file proceeds.
- A wrong Master Password refuses before anything is written.
- The report's last line names the source file path and suggests deleting it.
- `go.mod` gains no new dependency.
