# Importing Secrets plans in full, then only ever adds

`import-secrets` reads a file another password manager produced and fills a
Vault from it. It differs from every other write path in the application in
one way that shapes all of its rules: it acts on hundreds of Secrets at once,
from a file nobody in this project wrote, whose contents the user has never
inspected. A form saves one Secret the user is looking at; an import saves
three hundred and seventy-one they are not.

Because of that, the command is built so the user can never be surprised by
what it did, and never lose anything because of what it could not do.

**A Source Format knows nothing about the Vault.** A Format has two
responsibilities: recognising a file, and turning that file's bytes into
`[]secret.Secret`. It is handed the raw bytes rather than a parsed header, so
the package itself assumes no file shape at all and a zip or JSON export fits
the same seam.

Recognition is strict and reading is tolerant, which is why `--from` is worth
having. A Format claims a file only on an exact header match, so detection
never guesses; but a Format the user names by hand reads columns by their
names rather than their positions, so an export that gained a column, lost
one, or reordered them still imports. Without that split, the override would
exist for drifted exports and then refuse the only files it was meant for. Slugs, collisions, encryption, and the order of
writes are the importer package's business, decided once. The alternative —
each Format writing into the Vault itself — would have copied the rules below
into every new Format, and the rules are precisely the part worth getting
right once. This is why the second Format is expected to be one file with no
Vault knowledge in it at all.

**A colliding title is suffixed, not refused and not skipped.** Duplicate
titles are not an anomaly in an export; the reference 1Password export has
eighteen groups of them, three of them triples, because a user naturally has
two GitHub accounts and three logins they once called "Login". Refusing the
whole file would send that user to hand-edit a plaintext CSV containing every
password they own — the one file they should be deleting, not opening in an
editor. Skipping the colliding rows would silently drop twenty-one logins,
and the ones most likely to be duplicated are the ones most likely to matter.
So the second `Packtpub` becomes `Packtpub 2` and the third `Packtpub 3`.

The *title* is changed, not only the file name. Slug and title are already
bound together (docs/adr/0004), and letting them drift would mean the list
shows two Secrets called `Packtpub` that are distinguishable only by opening
the folder. Every shift is named in the report, so a user who wants better
names knows exactly which Secrets to rename and can do it from the TUI.

Collisions are resolved against the slugs already in the Vault as well as
against earlier rows in the same file, using `vault.List()`, which reads file
names only and needs no decryption. An import therefore never overwrites or
merges into an existing Secret; it only ever adds. A row whose title matches
something already stored becomes a new, suffixed Secret, and the original is
left untouched.

**Everything is validated before anything is written; a failure during
writing does not roll back.** The first half follows docs/adr/0010: the whole
file is read, mapped, collision-resolved, and `Validate()`d in memory, and a
single unusable row — a TOTP seed that will not parse, say — aborts the
import before one file exists. A partly-imported Vault is worse than an
unimported one, because the user cannot tell which half they still need.

The second half departs from docs/adr/0010 deliberately. `import-keys` rolls
back two files it just wrote; rolling back here means deleting up to several
hundred Secrets. Those Secrets are individually complete and correctly
encrypted — the failure was the disk, not them — and a delete path whose
worst bug destroys real Secrets is a poor trade for tidiness. So a write
failure stops the import and reports exactly how many Secrets were stored.
Re-running the command afterwards is safe by construction: because import
only ever adds, and collisions are suffixed, a second run cannot corrupt what
the first one wrote.

**The Master Password is required although the Recipient would do.**
Encrypting needs only the public key: `crypto.RecipientOnly` exists, and
CONTEXT.md says plainly that holding the Recipient alone is enough to create
Secrets. `import-secrets` asks for the Master Password anyway. Adding
hundreds of Secrets to a Vault is an act of ownership, not of access, and it
should be gated by proof that the person running it can also read what is
already there. It also keeps one story across the command line: `init`,
`export-keys`, `import-keys`, and `change-master-password` all ask, and a
command that silently did not would read as an oversight rather than a design.

`--dry-run` is the one exception, and it follows from the same reasoning
rather than bending it: a dry run adds nothing, so there is no act of
ownership to prove. It opens the Vault with `crypto.RecipientOnly`, which is
enough to list the slugs already taken — a slug needs no decryption
(docs/adr/0004) — and asks for nothing. The prompt appears exactly when
something is about to change.

## Consequences

Phase one produces a complete plan before phase two touches anything, so
`--dry-run` costs nothing to offer: it runs phase one and prints what would
happen. A user can see every renamed title and every count before committing,
which is the intended way to meet an unfamiliar export.

A Vault imported into twice holds two copies of everything, the second
suffixed. This is a real outcome, not a guarded-against one; deduplicating
would mean reading and comparing existing Secrets, which is merging, which
this command does not do.

Values cross unchanged. URLs are not normalised, `otpauth://` URIs are not
rewritten, and a scheme as broken as `httpsipotapp://…` is stored as it was
found — the same principle as docs/adr/0008, and the reason a bad value fails
loudly at its Field Type rather than quietly at import.

The source file is untouched. After a successful import the user's passwords
exist in two places, one of them plaintext, and only the user can decide when
that file goes. The report says so on its last line; it does not act.
