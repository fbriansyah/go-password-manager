# Changing the Master Password reseals the Identity in place

`change-master-password` opens the Identity with the current Master Password
and writes it back encrypted with a new one. Nothing else moves: because the
Master Password only ever protects the Identity and never a Secret
(docs/adr/0003), every Secret in every Vault stays byte-for-byte the same, and
the Recipient — derived from the Identity, which is unchanged — stays the
same too. This is the payoff of that asymmetric design: a password change
costs one file, not a re-encryption of the whole Vault.

**The Identity is replaced atomically and only after it has been proven
good.** The resealed Identity is written to a temporary file beside the real
one, fsynced, then opened again with the new Master Password and its derived
Recipient compared to `recipient.pub`. Only when that round trip succeeds is
it renamed over the old file. At every instant there is a complete, openable
Identity on disk; a crash, a full disk, or a bug in the resealing step leaves
the old Identity untouched and at worst a stray temporary file. This is the
same verify-before-write rule `import-keys` follows (docs/adr/0010), for the
same reason: a broken Identity is discovered the day it is needed, which is
too late.

**No backup of the old Identity is kept.** It would be easy to leave
`identity.age.bak` behind as a safety net, but that file opens with the old
Master Password forever, which means the old password was never actually
retired — and the user has no way to know that. "Change" has to mean the old
password no longer opens anything on this disk. A user who wants a copy of the
previous Identity makes one deliberately with `export-keys` first.

**Earlier exports still open with the earlier password.** A zip from
`export-keys` is a plain container around the Identity as it was at export
time (docs/adr/0009), so it is sealed with whatever Master Password was
current then. The command cannot reach those zips and does not try; it says
so on success and points at `export-keys` to make a fresh one. Running
`export-keys` automatically was considered and rejected: it would write an
unasked-for file into the working directory and smuggle a second feature into
a command whose whole scope is one file.
