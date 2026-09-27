# A Vault has one implementation, and tests open a real folder

For a long time `internal/vault` held two Vaults: `FS`, which stores Secrets as
files in a folder, and `Mem`, which stores them in a map. `Mem` had no
production caller. It existed so that tests in three packages could have a
Vault without touching a disk, and it was kept honest by a shared test table
that ran the same cases against both.

The problem was that every rule about storing a Secret was written twice.
`FS.prepare` and `Mem.prepare` were byte-identical — validate, turn the title
into a slug, refuse a title that leaves no file name, marshal, encrypt — and so
were the collision rules: `ErrSlugTaken` on a taken slug, `ErrNotFound` on a
missing one, write the new file before removing the old one on a rename. The
shared table existed because nothing else stopped the two copies drifting, and
a shared table is a poor substitute for a rule that can only be written once.

**`Mem` is deleted. `FS` is the only Vault.** A test that needs one opens a
real Vault on a `t.TempDir()`, which is what every test inside
`internal/vault` already did. That costs a few microseconds of real file I/O
per test and buys the guarantee that the rules a test exercises are the rules
that ship.

**The obvious alternative was rejected: splitting the rules out over a seam,
with a folder adapter and a memory adapter beneath them.** That removes the
duplication too, and it is the design an architecture review will propose,
because one was proposed and graded `Strong` before this decision was taken.
It was turned down because a Vault *is* the working folder (docs/adr/0001).
There is no second real way to store Secrets and there is not going to be one,
so the only thing that would ever cross that seam a second time is a fake that
exists to serve tests — and tests are served just as well by a temporary
folder. A seam with one real adapter is a hypothetical seam, and it would have
been paid for in three new types where there is now one.

**`vault.Vault` stays, even with one implementation.** It is not there to hold
two Vaults apart; it is the substitution point consumers use. `internal/tui`
types its store and its Unlock seam through it, and
`importer_test.go`'s `failingVault` wraps it to make a write fail part way
through — the only test in the repository that exercises a disk giving out.
Deleting the interface would mean re-cutting five files' signatures, and
`internal/importer` asking for six methods when it uses two is a separate
shallowness to address on its own.

## Consequences

`internal/vault` went from two types to one, and its coverage from 80.3% to
78.8% — `Mem` was thoroughly covered by the shared table, and deleting covered
code lowers the ratio without making anything less tested. The number moved in
the wrong direction for the right reason.

Three test packages now build their Vaults the same way, and a fake Vault that
disagrees with the rules is no longer possible to write by accident. Each of
those packages keeps its own three-line `nopCipher`; a shared one would be a
smaller version of the same mistake.
