# Milestone 4 — The TOTP Field Type

Status: done
Depends on: Milestone 2 (edit) and Milestone 3 (the form owns its rows and
validation). Touches `internal/tui/tui.go` only for the tick.

## Goal

A Secret can hold a username and a password but not the third thing most
logins now ask for. This milestone adds one Field Type, `tp`, that stores an
Authenticator Seed and turns it into a Code on demand — so the password manager
is the authenticator too, and the phone can stay in the pocket.

Done means a user can paste what a service shows under "can't scan?", save, and
from then on see the current Code and its remaining seconds in the detail pane,
and copy that Code with the same `c` that copies everything else.

## Scope

1. **A `totp` package.** Parses an Authenticator Seed — bare base32 or an
   `otpauth://totp/` URI — and derives the Code for a given instant, together
   with how long that Code is still good for. A leaf like `generator`: it
   knows nothing about Secrets or the TUI.
2. **A Field Type `tp`, named `TOTP`.** Renders the Code and a countdown;
   copies the Code; edited in a plain single-line editor; not generatable.
3. **Validation at save time.** A Seed that cannot be parsed refuses the save
   with a message, the same way an empty title does.
4. **A detail pane that keeps time.** While the highlighted Secret has a `tp`
   Field the pane redraws every second, so the Code and countdown are live.

## Out of scope

- Reading a QR code from an image or the screen. The text under "can't scan?"
  is the input.
- A `gopm totp <slug>` command for scripts. It needs a non-interactive Unlock
  that does not exist yet.
- HOTP and the non-standard schemes (Steam, Yandex). `otpauth://hotp/` is
  rejected by validation like any other malformed Seed.
- Showing a QR code to move a Seed to another device. Revealing the raw Seed
  is enough.
- Copying a Code from the list without opening the detail pane. `c` in the
  detail pane is how every Field is copied.

## Decisions

**The Seed is stored as handed over.** Base32 or URI, kept verbatim, parsed on
use. Recorded in `docs/adr/0008-authenticator-seed-is-stored-as-handed-over.md`.

**Field Types can validate.** `secret.Type` gains `Validate func(value string)
error`, nil for the three existing types, and `Secret.Validate` calls it per
Field. The form already calls `Secret.Validate` before submitting and the
Vault calls it again before writing, so no new call site is needed. The paste
is the only moment the user still has the original QR in front of them;
finding out a week later at a login prompt is too late.

**Render and Copy do not trust the file either.** A Seed that fails to parse
renders as `invalid seed` and makes `c` report an error in the status bar.
Validation stops new bad Seeds; this stops old ones from crashing a pane.

**The Code is always visible; reveal shows the Seed.** A Code is worth nothing
without the password and nothing after its period, so hiding it behind `r`
would add a keypress and protect nothing. `r` is kept for the thing that is
sensitive: the raw Seed, for moving it elsewhere. Without reveal the pane shows
`492 817  ·  12s`; six digits are grouped in threes, eight in fours.

**What is on screen is what gets copied.** `c` copies the Code currently shown,
never the next one, however few seconds remain — the countdown is there so the
user can wait. `clipboard.ClearAfter` stays at thirty seconds; a Code is never
useful longer than that.

**The Seed is edited in the open.** `EditorLine`, not `EditorMasked`. Save-time
validation can only say a Seed is wrong, not where; the user needs to see a
hundred-character URI to fix it. The Seed is hidden everywhere else.

**`Render` keeps its signature.** The current time comes from a package-level
`totp.Now` that tests replace, not from a new parameter that three other types
would have to ignore.

**TOTP is written, not imported.** RFC 6238 over RFC 4226 is a few dozen lines
of `crypto/hmac`, `encoding/base32` and `encoding/binary`, checked against the
test vectors in RFC 6238 Appendix B; the URI is `net/url`. The only crypto
dependency stays `age`.

**The ID is `tp`.** Two letters like `tx`, `ps`, `ta`. The tests that use
`"totp"` as their example of a foreign type switch to an ID that will never be
registered, so they stop depending on TOTP not existing.

## What has to change

**`internal/totp/totp.go`** — new. `Parse(value string) (Seed, error)` accepts
base32 (case-insensitive, spaces ignored, padding optional) or an
`otpauth://totp/` URI with `secret`, and optional `digits` (6 or 8), `period`,
`algorithm` (SHA1, SHA256, SHA512). `Seed.Code(at time.Time) string` and
`Seed.Remaining(at time.Time) time.Duration`. `var Now = time.Now`.

**`internal/secret/fieldtype.go`** — `Type` gains `Validate`. `Register` leaves
it nil when unset. A fourth `Register` call for `tp`, whose Render, Copy and
Validate call into `totp`; `secret` importing `totp` is fine because `totp` is
a leaf.

**`internal/secret/secret.go`** — `Secret.Validate` calls `TypeFor(f.Type)
.Validate(f.Value)` when it is set and wraps the error with the Field label.

**`internal/tui/tui.go`** — `tick()` keeps running while `clearsAt` is in the
future *or* the highlighted Secret has a `tp` Field; the `tickMsg` handler and
the places that change the highlighted Secret (selection moves, load, save,
delete) start it when needed. Nothing else: `detailModel.View` already calls
`Render` on every draw.

**`internal/tui/detail.go`** — no change; `Render` does the work.

**Tests** — `secret_test.go`, `form_test.go` and `tui_test.go` replace `"totp"`
as the foreign-type example. New tests: RFC 6238 vectors, URI parsing, the
countdown boundary at second 29→0, validation refusing a save, and the pane
ticking without a clipboard countdown running.

**`README.md`** — `TOTP` added to the list of Field Types with one sentence on
what to paste; the Status section follows.

## Acceptance criteria

- Pasting `JBSWY3DPEHPK3PXP` into a `TOTP` Field and saving: the detail pane
  shows a six-digit Code and a countdown that reaches 0 and rolls over to a
  new Code, matching what an authenticator app shows for the same seed.
- Pasting an `otpauth://totp/` URI with `digits=8&period=60&algorithm=SHA256`:
  eight digits, a sixty-second countdown, and the Code an authenticator app
  shows for that URI.
- `c` on the Field puts exactly the digits on screen in the clipboard, without
  the space or the countdown, and the clipboard clears thirty seconds later.
- Pasting `not base32!` or `otpauth://hotp/…` and pressing `ctrl+s`: no file
  is written and the form shows a message naming the Field.
- A `.gopm` written by hand with a `tp` Field holding garbage: the Vault
  opens, the pane shows `invalid seed`, and `c` reports an error instead of
  copying.
- `r` on a `tp` Field shows the stored Seed verbatim; moving to another
  Secret hides it again.
- With no `tp` Field highlighted and no clipboard countdown, the application
  sends no ticks.
- `go.mod` gains no new dependency.
- The existing foreign-type tests still pass, and still exercise a type that
  is not registered.
