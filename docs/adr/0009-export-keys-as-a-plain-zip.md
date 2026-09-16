# Exporting the keypair bundles it as a plain zip, not a second encryption layer

`export-keys` bundles the Identity and the Recipient into a zip so the pair is
easy to move to another drive. The Identity file is already encrypted with the
Master Password (docs/adr/0003), through age's own scrypt-based passphrase
identity (docs/adr/0002); the Recipient is public by design. Wrapping either
in a second, independently-keyed encryption layer would buy no confidentiality
that the Master Password does not already provide, and it would cost a second
password the user has to remember and enter correctly a year later to recover
anything. The zip is a container, not a vault: what protects the Identity
inside it is the same Master Password that protects it on disk today.

Because there is no new encryption to write, there is also nothing new for
`internal/crypto` to own. The only cryptographic operation `export-keys`
performs is the `Unlock` that already exists, used here purely to verify the
Master Password is correct and the Identity is not corrupt before it is
copied anywhere — a cheap check now is better than discovering a bad export
the day it is needed for recovery.

**The Vault is out of scope.** Secrets stay where they are — a working folder
that can be kept in git (docs/adr/0001) — so this feature only ever touches
the two key files, never Secrets. A Vault export, if wanted later, is a
separate feature with a separate name.

**The zip carries a `manifest.json`** recording a format version, the export
timestamp, and a SHA-256 checksum of the Identity file. Nothing reads this
manifest yet — there is no `import` command — but a future one needs to tell
old exports from new ones and detect a corrupted or truncated Identity before
attempting to Unlock it, and neither is possible to add retroactively to a zip
that was never written with it. The checksum does not cover the Recipient
file: it is plaintext and a corrupt copy is merely inconvenient, not silently
misleading the way a corrupt Identity would be.

**The output path defaults to a timestamped file** in the current directory
(`gopm-keys-<YYYYMMDD-HHMMSS>.zip`) rather than a fixed name, and `export-keys`
refuses to overwrite an existing file at `-o`. An export is a backup; silently
clobbering the last one defeats the point of taking it.
