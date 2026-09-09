# gopm

A file-based password manager with a terminal UI. Secrets live as encrypted
files in the folder you run it from, so they travel with the project they
belong to and can be committed to git.

Each secret is encrypted with [age](https://github.com/FiloSottile/age)
(X25519 + ChaCha20-Poly1305) to your public key. Your private key is itself
encrypted with a master password, so reading a secret needs two things: the
key file and the password. Writing needs only the public key.

## Install

Requires Go 1.26 or newer.

```sh
go install github.com/fbriansyah/go-password-manager@latest
```

That installs the binary under the module name, so rename it if you want the
short command: `mv "$(go env GOPATH)/bin/go-password-manager" "$(go env GOPATH)/bin/gopm"`.

Or build from a clone:

```sh
git clone https://github.com/fbriansyah/go-password-manager
cd go-password-manager
go build -o gopm .
```

To copy values to the clipboard, install a clipboard tool — `wl-clipboard` on
Wayland, `xclip` on X11. macOS uses the built-in `pbcopy`. Without one, gopm
still runs; copying reports that no tool is installed.

## First run

Create your keypair and config. This is a one-time step:

```sh
gopm init
```

It asks for a master password twice, then writes:

| File | What it is |
|---|---|
| `~/.config/gopm/identity.age` | your private key, encrypted with the master password |
| `~/.config/gopm/recipient.pub` | your public key |
| `~/.config/gopm/config.yaml` | paths to both of the above |

**Back up the identity file.** There is no recovery: lose that file or forget
the master password and every secret encrypted to it is gone for good.

## Usage

Run `gopm` in any folder to open the secrets stored there:

```sh
cd ~/projects/acme
gopm
```

Point it at another folder without changing directory:

```sh
gopm -d ~/projects/acme      # or --directory
```

The folder must already exist — a mistyped path is rejected rather than
silently treated as an empty vault.

gopm asks for your master password, then shows the secrets in that folder:
the list on the left, the selected secret's fields on the right.

### Keys

**List**

| Key | Action |
|---|---|
| `↑` `↓` | move through secrets |
| `/` | search title, description and tags |
| `tab` | move focus to the detail pane |
| `n` | new secret |
| `e` | edit the selected secret |
| `d` | delete the selected secret (asks first) |
| `q` | quit |

**Detail pane**

| Key | Action |
|---|---|
| `j` `k` | select a field |
| `c` | copy the field to the clipboard (cleared after 30 seconds) |
| `r` | reveal a hidden value |
| `e` | edit the selected secret |
| `d` | delete the selected secret (asks first) |
| `esc` | back to the list |

**Delete confirmation**

| Key | Action |
|---|---|
| `y` | go ahead and delete |
| `n` `esc` | cancel |

Deleting is permanent — there is no undo, trash, or recovery.

**New / edit secret form**

`e` opens the same form `n` does, filled in with the selected secret; saving
writes back to the same file. Changing the title renames the file — the old
one is removed once the new one is written — and is refused with a message if
another secret already has that title.

| Key | Action |
|---|---|
| `tab` `shift+tab` | move between inputs |
| `←` `→` | change the field type (on a field's label) |
| `ctrl+n` | add a field |
| `ctrl+d` | remove the focused field |
| `ctrl+g` | open the generator panel (password fields only) |
| `ctrl+s` | save |
| `esc` | cancel; asks first when there are unsaved changes |

Field types are `Text` (single line), `Password` (masked, generatable) and
`Note` (multi-line).

**Generator panel**

`ctrl+g` opens a panel over the field rows, showing a candidate password next
to the four knobs that shape it — this is the Generator Policy.

| Key | Action |
|---|---|
| `↑` `↓` | move between knobs (length, upper, digits, symbols) |
| `←` `→` | change the focused knob — length by one, a class on or off |
| digits | type a length directly |
| `r` | reroll: a new candidate, same knobs |
| `enter` | accept the candidate shown |
| `ctrl+s` | save the current knobs to `config.yaml` as the default |
| `esc` | close without touching the field |

A Policy changed in the panel stays in effect for the rest of the session even
without saving; `ctrl+s` is only for making it the default the next time gopm
starts.

Copying starts a short-lived helper process that clears the clipboard 30
seconds later, and only if the clipboard still holds what gopm put there — so
anything you copy in the meantime is left alone. The helper outlives the TUI,
so the value survives quitting gopm before you paste it.

## Files and config

Secrets are stored as `<slug>.gopm`, an ASCII-armored age file. The slug comes
from the title, so "Facebook" becomes `facebook.gopm`, and renaming the title
renames the file. Everything inside — title, description, tags and all fields —
is encrypted; the filename is the one part that stays readable.

`config.yaml` holds the key paths, and optionally the Generator Policy:

```yaml
PUBLIC_KEY_PATH: "/home/you/.config/gopm/recipient.pub"
PRIVATE_KEY_PATH: "/home/you/.config/gopm/identity.age"
GENERATOR_LENGTH: 24
GENERATOR_UPPER: true
GENERATOR_DIGITS: true
GENERATOR_SYMBOLS: false
```

Both key paths accept `~` and environment variables. Drop a `.gopm.yaml` in a
vault folder to override any of these keys for that folder only — useful when
a project should use a separate key, or a client's site rejects symbols. The
override is read from the vault folder, so it follows `-d`.

Every key is optional and merges independently: a key a file does not mention
is inherited from the file below it, and ultimately from the built-in default
(20 characters, every class on) if no file mentions it at all. `gopm init`
never writes the generator keys itself — they only appear once you save a
Policy from the generator panel, or add them by hand.

Since only the public key is needed to write, a machine that holds
`recipient.pub` alone can add secrets to a vault without ever being able to
read it.

## Development

```sh
go test ./...
```

The domain layer is independent of the TUI: `internal/secret` (model, codec,
field types), `internal/crypto` (age), `internal/vault` (storage, with a
filesystem and an in-memory implementation), `internal/tui` (bubbletea),
`cmd/` (cobra). The vocabulary is in [CONTEXT.md](./CONTEXT.md) and the design
decisions behind it in [docs/adr/](./docs/adr/).

## Status

Working today: `init`, unlock, list and search, view and copy fields, create,
edit and delete secrets, all with a configurable password generator.
