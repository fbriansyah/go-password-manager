# Milestone 3 — The Generator Policy

Status: done
Depends on: Milestone 1. Independent of Milestone 2, but both touch
`internal/tui/form.go`, so whichever lands second rebases onto the other.

## Goal

`ctrl+g` currently fills a Field with `generator.Default()` — twenty characters,
every class on — and nothing can change that. This milestone gives the
Generator Policy a place to live, a way to be changed, and a way to be kept.

Done means a user can set a length and switch character classes off, see the
password before accepting it, and either use that Policy once or make it their
default without leaving the TUI.

## Scope

1. **The Policy is configuration.** Four flat keys, read by the existing
   two-layer merge: `GENERATOR_LENGTH`, `GENERATOR_UPPER`, `GENERATOR_DIGITS`,
   `GENERATOR_SYMBOLS`.
2. **A panel inside the form.** `ctrl+g` opens a panel over the field rows
   showing the four knobs and a candidate password in clear text.
3. **The panel can save.** `ctrl+s` in the panel writes the current Policy to
   the global `config.yaml` as the new default.
4. **Changes stick for the session.** A Policy changed in the panel stays
   changed until the application closes, whether or not it was saved.

## Out of scope

- Making lower case optional. It is the guarantee that the pool is never empty
  (see below), and it is not a field of `generator.Options`.
- Saving a Policy to a Vault's `.gopm.yaml` from the TUI. The file is still read
  and still overrides the global one; it is only written by hand.
- Per-Secret or per-Field Policies. One Policy per session.
- Anything about passwords the generator did not make: no strength meter, no
  reuse or age checks.

## Decisions

**Three layers, merged per key.** `generator.Default()` is the starting point,
the global `config.yaml` overrides it, the Vault's `.gopm.yaml` overrides that.
Each layer speaks only about the keys it contains, so `GENERATOR_SYMBOLS: false`
alone in a Vault leaves the length inherited. Recorded in
`docs/adr/0005-configuration-keys-are-flat-and-absence-means-no-opinion.md`.

**Absence is read with `IsSet`, never with a zero test.** The existing
`merge` uses "a non-empty value wins", which cannot distinguish `false` from
unwritten. Booleans must be read as `if v.IsSet(key) { … v.GetBool(key) }`.

**Saving always goes to the global file.** A button that sometimes wrote to the
Vault folder and sometimes to the home directory would be unpredictable from
inside the TUI, and a Vault is a folder people sync or commit — a UI preference
does not belong in it.

**A bad length in a file is not a startup error.** `GENERATOR_LENGTH: 4` loads
fine, shows as 4 in the panel, and fails at generate time with the existing
`ErrTooShort`, displayed through `formModel.err`. Missing key paths stop the
application because nothing can be decrypted without them; a silly length stops
nothing. Clamping it silently is worse than failing, because the next number
the user writes with real intent would also be quietly ignored.

**The panel is state inside `formModel`, not a fourth screen.** It only ever
exists to fill the focused Field, and `m.focus` already says which row that is.
`tui.go` does not change.

**The preview is the password.** `enter` accepts exactly the candidate on
screen; it does not generate a fresh one. Any knob change, and the reroll key,
produce a new candidate.

**`config.Config` carries `generator.Options` directly.** `generator` is a leaf
package, so there is no cycle, and no parallel struct to keep in step.

## What has to change

**`internal/config/config.go`** — `Config` gains a `Generator generator.Options`
field. `Load` seeds it from `generator.Default()` before reading any file.
`merge` gains the four keys, read through `IsSet`. A new function alongside
`Write` — `Update(path string, values map[string]string) error` — rewrites
matching lines in place and appends the ones that are missing, touching nothing
else in the file. `Write` keeps its current job: creating a file from nothing
for `gopm init`, still with the two key paths only.

**`internal/generator/generator.go`** — no change. `Options`, `Default` and
`Generate` already do everything asked of them.

**`internal/tui/form.go`** — `formModel` gains the session Policy, the panel's
open/closed state, the focused knob, and the current candidate. `newForm` takes
the Policy from configuration as an argument. While the panel is open,
`formModel.Update` handles its keys and `View` draws it in place of the field
rows. `generate()` becomes "open the panel"; on a Field Type that is not
`Generatable` it now sets `err` instead of doing nothing at all.

**`internal/tui/tui.go`** — one line: `newForm` is called with
`m.cfg.Generator`.

**`README.md`** — the four keys documented in the configuration section, the
panel keys in the keybinding table.

## Panel keys

| Key | Action |
| --- | --- |
| `↑`, `↓` | Move between knobs |
| `←`, `→` | Change the focused knob — length by one, a class on or off |
| digits | Type a length directly |
| `r` | Reroll: a new candidate, same Policy |
| `enter` | Accept the candidate shown and close |
| `ctrl+s` | Save this Policy to the global `config.yaml` |
| `esc` | Close without touching the Field |

Length is held between 8 and 128 inside the panel, so a length below
`ErrTooShort` can only ever come from a hand-edited file.

## Acceptance criteria

- No configuration file mentions the generator: `ctrl+g` still produces twenty
  characters with every class present.
- `GENERATOR_SYMBOLS: false` in the global file: the panel opens with symbols
  off and no candidate contains one.
- Global sets `GENERATOR_LENGTH: 32`, a Vault's `.gopm.yaml` sets only
  `GENERATOR_SYMBOLS: false`: candidates in that Vault are 32 characters and
  symbol-free.
- `GENERATOR_SYMBOLS: false` written in a file is honoured — proof the merge
  reads absence, not falsity.
- Turning symbols off in the panel, accepting, then pressing `ctrl+g` on the
  next Field: the panel still has symbols off.
- `ctrl+s` on a `config.yaml` that carries a comment and an unrelated key: the
  four generator keys are written or updated, the comment and the unrelated key
  survive, and reopening the application starts from the saved Policy.
- `ctrl+s` twice in a row does not append a second copy of any key.
- `GENERATOR_LENGTH: 4`: the Vault opens, the panel shows 4, and generating
  reports "a password must be at least 8 characters long".
- `esc` in the panel leaves the Field exactly as it was.
- `ctrl+g` on a Field Type that is not `Generatable`: no panel, and a message
  saying so.
