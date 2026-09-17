# Importing a keypair verifies everything before writing anything

`import-keys` is the reverse of `export-keys` (docs/adr/0009): it takes a zip
produced by `export-keys` and installs the Identity and Recipient it carries.
Because installing the wrong Identity means every existing Secret in a Vault
becomes unreadable, and installing a corrupt one is discovered only the day
recovery is needed, the whole command is built around failing before writing
rather than after.

**An existing keypair is never overwritten.** `crypto.GenerateKeypair` already
refuses to overwrite an Identity or Recipient (crypto.go), and `import-keys`
follows the same rule: if a keypair already exists at the resolved config
paths, import is refused outright. There is no `--force`; a user who really
means to replace a keypair removes the old one deliberately first.

**Verification happens in order, cheapest first, before any file is written
at its destination.** Each step can reject the whole import:

1. `manifest.json` must exist inside the zip and its `format_version` must be
   one this build understands (`1` today). A missing or unrecognized manifest
   means either a zip this application never wrote, or a future export
   version — either way, guessing at the layout is refused rather than risked
   (this is the reason `manifest.json` exists at all, per docs/adr/0009).
2. The extracted Identity's SHA-256 must match `manifest.json`'s
   `identity_sha256`. This catches a truncated or corrupted zip before the
   Master Password is even asked for.
3. The Master Password must open the Identity (`crypto.Unlock`). This is the
   same verify-before-write the export side already performs, run again here
   because a zip can travel a long time between the two.
4. The Recipient extracted from the zip must match the Recipient the unlocked
   Identity itself derives. A zip's Identity and Recipient could in principle
   come from two different exports; this check makes sure what gets installed
   is one real keypair, not two halves of different ones.

**Entries are read by name, never used as write paths.** `import-keys` looks
up `identity.age`, `recipient.pub`, and `manifest.json` by exact name inside
the zip and ignores anything else it contains; the destination paths always
come from the resolved `config.Config`, never from the zip's own entry names.
This is what keeps a zip-slip entry (`../../something`) from ever being
capable of writing outside the config folder.

**A machine with no configuration yet is a supported starting point.**
`import-keys` resolves its target config the same way `export-keys` does —
`config.Load(vaultDir)`, honoring a Vault folder's `.gopm.yaml` override
(docs/adr/0001) — but where `export-keys` requires a config to already exist,
`import-keys` falls back to `config.Defaults()` when none does, mirroring what
`gopm init` would create. This makes `import-keys` a full substitute for
`init` when moving to a new machine: no keypair needs to be generated only to
be immediately thrown away.

**A partial failure rolls back.** Because import only ever starts from a
clean slate (no existing keypair, per the first rule above), any file this
run wrote before a later step failed is safe to delete: the Identity, the
Recipient, and a freshly-written `config.yaml` when import created one. This
avoids leaving a half-installed keypair that later commands would treat as
real.
