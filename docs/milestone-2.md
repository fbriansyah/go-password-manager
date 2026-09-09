# Milestone 2 — Editing and Deleting Secrets

Status: not started
Depends on: Milestone 1 (`init → create → unlock → view → copy`) working.

## Goal

Right now a Vault can only be added to, not managed. This milestone closes the
life cycle of a Secret: change one that already exists, and remove it, without
ever leaving the Vault half-written.

Done means a user can open a Vault, fix a password they just rotated, rename a
Secret, drop a Field they no longer use, and delete a retired Secret — all from
the TUI, without touching a `.gopm` file by hand.

## Scope

1. **Edit a Secret.** From the detail pane, `e` opens the same form as the
   new-Secret screen, filled in with the selected Secret. Saving writes back to
   the same slug.
2. **Rename through the title.** When the title changes the slug changes with
   it, and the old file is removed once the new one is written. If the new slug
   already belongs to another Secret, the save is refused with a message rather
   than overwriting it.
3. **Delete a Secret.** `d` from the list or the detail pane, behind one
   confirmation that has to be answered explicitly. After a delete, the
   selection moves to a neighbouring Secret.
4. **Cancelling safely.** `esc` in the edit form throws the changes away
   without touching the disk, and asks first when there are unsaved changes.

## Out of scope

- Undo, trash, or old versions. Deleted means gone.
- Changing the master password or rotating the Identity.
- Import, export, and synchronisation.
- New Field Types (TOTP and friends) — the registry is ready for them, filling
  it is another milestone's job.

## What has to change

**`internal/vault`** — no API change. `Save` and `Delete` already exist and
already carry the slug-collision rules; this milestone only uses them.

**`internal/tui/form.go`** — the form needs to be able to start from an
existing Secret rather than only from nothing: one constructor that fills in
the meta and builds a row per Field with the right type. It also needs a
"dirty" marker so `esc` knows when to ask.

**`internal/tui/tui.go`** — one new screen for the delete confirmation, two new
commands (`saveCmd`, `deleteCmd`) alongside `createCmd`, and a form that knows
which slug it is editing (empty meaning a new Secret). After a save or a
delete, the list is reloaded and the selection aimed at the right slug.

**`README.md`** — the keybinding tables and the Status section follow.

## New keybindings

| Screen | Key | Action |
| --- | --- | --- |
| List, Detail | `e` | Edit the selected Secret |
| List, Detail | `d` | Delete the selected Secret (asks first) |
| Confirmation | `y` | Go ahead and delete |
| Confirmation | `n`, `esc` | Cancel |
| Edit form | `esc` | Cancel; asks first when there are unsaved changes |

## Acceptance criteria

- Changing a Field value and saving: the file on disk changes, the title and
  the file name do not.
- Changing the title: the old file is gone, a new one appears under the new
  slug, and its contents are intact.
- Changing a title to another Secret's title: no file changes, and the TUI
  shows a collision message.
- Adding and dropping Fields on an existing Secret survives closing and
  reopening the TUI.
- Deleting a Secret: the file is gone, the list shrinks, and the selection
  stays valid even when the deleted Secret was the last row.
- Cancelling the delete confirmation touches the disk not at all.
- No failed write leaves a `.tmp` file behind in the Vault.

## Test plan

- `internal/vault`: add table cases for edit-then-delete and for deleting a
  Secret that does not exist, run against both the filesystem and the
  in-memory implementation.
- `internal/tui`: extend the existing headless harness — open the edit form,
  change one value, save, and inspect the Vault; then the delete path with the
  confirmation both accepted and cancelled.
- One manual run: a real password rotation on a Vault holding three Secrets.

## Risks

- **A rename is not atomic.** Write-new-then-remove-old means there is a very
  short window where both files exist. The order is deliberate: failing midway
  leaves a visible duplicate rather than a lost Secret.
- **A delete cannot be undone.** The confirmation is the only guard; do not let
  `d` delete outright for the sake of speed.
- **A form used in both directions** leaks state between Secrets easily. Build
  the form from scratch every time edit mode is entered; never reuse the last one.

## After this

Candidates for the next milestone, not yet decided: a TOTP Field Type, changing
the master password, and a non-interactive command (`gopm get`) for use from
scripts.
