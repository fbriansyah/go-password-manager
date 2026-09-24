package tui

import (
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/fbriansyah/go-password-manager/internal/clipboard"
	"github.com/fbriansyah/go-password-manager/internal/config"
	"github.com/fbriansyah/go-password-manager/internal/generator"
	"github.com/fbriansyah/go-password-manager/internal/secret"
	"github.com/fbriansyah/go-password-manager/internal/vault"
)

// Internal messages.
type (
	unlockedMsg struct {
		store   vault.Vault
		entries []entry
		skipped []string
	}
	failedMsg  struct{ err error }
	createdMsg struct {
		slug    string
		secret  *secret.Secret
		entries []entry
	}
	savedMsg struct {
		slug    string
		entries []entry
	}
	deletedMsg struct {
		entries []entry
	}
	copiedMsg      struct{ label string }
	policySavedMsg struct{}
	tickMsg        time.Time
)

// unlockCmd performs the Unlock through the seam Model holds, then loads every
// Secret. Because the whole file is encrypted, the list can only be shown once
// this step finished (docs/adr/0004).
func unlockCmd(unlock func(string) (vault.Vault, error), password string) tea.Cmd {
	return func() tea.Msg {
		store, err := unlock(password)
		if err != nil {
			return failedMsg{err}
		}
		entries, skipped, err := loadAll(store)
		if err != nil {
			return failedMsg{err}
		}
		return unlockedMsg{store: store, entries: entries, skipped: skipped}
	}
}

// loadAll loads every Secret. One that cannot be opened — encrypted for another
// Recipient, say — is skipped and reported rather than failing the whole
// Vault.
func loadAll(store vault.Vault) ([]entry, []string, error) {
	slugs, err := store.List()
	if err != nil {
		return nil, nil, err
	}
	var entries []entry
	var skipped []string
	for _, slug := range slugs {
		s, err := store.Load(slug)
		if err != nil {
			skipped = append(skipped, slug)
			continue
		}
		entries = append(entries, entry{slug: slug, data: s})
	}
	return entries, skipped, nil
}

func createCmd(store vault.Vault, s *secret.Secret) tea.Cmd {
	return func() tea.Msg {
		slug, err := store.Create(s)
		if err != nil {
			return failedMsg{err}
		}
		entries, _, err := loadAll(store)
		if err != nil {
			return failedMsg{err}
		}
		return createdMsg{slug: slug, secret: s, entries: entries}
	}
}

// saveCmd writes changes back to an existing Secret. If the title changed
// the slug changes with it; Vault.Save handles the rename and the
// slug-collision check (docs/milestone-2.md).
func saveCmd(store vault.Vault, slug string, s *secret.Secret) tea.Cmd {
	return func() tea.Msg {
		newSlug, err := store.Save(slug, s)
		if err != nil {
			return failedMsg{err}
		}
		entries, _, err := loadAll(store)
		if err != nil {
			return failedMsg{err}
		}
		return savedMsg{slug: newSlug, entries: entries}
	}
}

// deleteCmd removes a Secret outright; there is no undo, so this only ever
// runs once the confirmation screen has been answered explicitly.
func deleteCmd(store vault.Vault, slug string) tea.Cmd {
	return func() tea.Msg {
		if err := store.Delete(slug); err != nil {
			return failedMsg{err}
		}
		entries, _, err := loadAll(store)
		if err != nil {
			return failedMsg{err}
		}
		return deletedMsg{entries: entries}
	}
}

func copyCmd(f secret.Field) tea.Cmd {
	return func() tea.Msg {
		value, err := secret.TypeFor(f.Type).Copy(f.Value)
		if err != nil {
			return failedMsg{err}
		}
		if err := clipboard.Copy(value); err != nil {
			return failedMsg{err}
		}
		return copiedMsg{label: f.Label}
	}
}

// saveGeneratorCmd writes o to the global configuration as the new default
// Generator Policy. It always targets the global file, never a Vault's
// override — a UI preference does not belong in a folder people sync or
// commit (docs/milestone-3.md).
func saveGeneratorCmd(o generator.Options) tea.Cmd {
	return func() tea.Msg {
		_, path, err := config.Defaults()
		if err != nil {
			return failedMsg{err}
		}
		if err := config.UpdateGenerator(path, o); err != nil {
			return failedMsg{err}
		}
		return policySavedMsg{}
	}
}
