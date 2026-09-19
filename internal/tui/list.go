package tui

import (
	"strings"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"

	"github.com/fbriansyah/go-password-manager/internal/secret"
)

// entry is one Secret, decrypted into memory during Unlock.
type entry struct {
	slug string
	data *secret.Secret
}

func (e entry) Title() string { return e.data.Meta.Title }

func (e entry) Description() string {
	if e.data.Meta.Description != "" {
		return e.data.Meta.Description
	}
	if len(e.data.Meta.Tags) > 0 {
		return "#" + strings.Join(e.data.Meta.Tags, " #")
	}
	return e.slug
}

// FilterValue lets a search reach the title, the description, and the tags.
func (e entry) FilterValue() string {
	return strings.Join(append([]string{e.data.Meta.Title, e.data.Meta.Description}, e.data.Meta.Tags...), " ")
}

// handleListKey answers a key on the list screen, in either focus mode: the
// list itself, or the detail pane once tab has moved the focus there.
func (m Model) handleListKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	k := shortcut(msg)
	if m.list.FilterState() == list.Filtering {
		var cmd tea.Cmd
		m.list, cmd = m.list.Update(msg)
		return m, cmd
	}
	switch k {
	case "q", "ctrl+c":
		m.quitting = true
		return m, tea.Quit
	case "?":
		return m.openHelp(), nil
	case "n":
		m.form = newForm(m.cfg.Generator)
		m.screen = screenForm
		m.setStatus("", false)
		return m, nil
	case "e":
		s := m.current()
		if s == nil {
			return m, nil
		}
		m.form = editForm(m.cfg.Generator, m.currentSlug(), s)
		m.screen = screenForm
		m.setStatus("", false)
		return m, nil
	case "d":
		s := m.current()
		if s == nil {
			return m, nil
		}
		m.deleteSlug = m.currentSlug()
		m.deleteTitle = s.Meta.Title
		m.deleteIndex = m.list.Index()
		m.screen = screenConfirmDelete
		return m, nil
	case "tab":
		m.detail.focused = !m.detail.focused
		m.detail.reveal = false
		return m, nil
	case "esc":
		if m.detail.focused {
			m.detail.focused = false
			m.detail.reveal = false
			return m, nil
		}
	case "r":
		if m.detail.focused {
			m.detail.reveal = !m.detail.reveal
			return m, nil
		}
	case "c":
		if m.detail.focused {
			f, ok := m.focusedField()
			if !ok {
				return m, nil
			}
			return m, copyCmd(f)
		}
	case "j", "down":
		if m.detail.focused {
			m.moveField(1)
			return m, nil
		}
	case "k", "up":
		if m.detail.focused {
			m.moveField(-1)
			return m, nil
		}
	}
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	m.clampField()
	return m, cmd
}

func (m *Model) setEntries(entries []entry, selectSlug string) {
	items := make([]list.Item, 0, len(entries))
	selected := 0
	for i, e := range entries {
		items = append(items, e)
		if e.slug == selectSlug {
			selected = i
		}
	}
	m.list.SetItems(items)
	if len(items) > 0 {
		m.list.Select(selected)
	}
	m.detail.fieldIndex = 0
}

// setEntriesNear rebuilds the list after a delete and selects whatever now
// sits at idx — the row the deleted Secret used to occupy — clamped to the
// new, shorter list.
func (m *Model) setEntriesNear(entries []entry, idx int) {
	items := make([]list.Item, 0, len(entries))
	for _, e := range entries {
		items = append(items, e)
	}
	m.list.SetItems(items)
	if len(items) > 0 {
		if idx >= len(items) {
			idx = len(items) - 1
		}
		if idx < 0 {
			idx = 0
		}
		m.list.Select(idx)
	}
	m.detail.fieldIndex = 0
}

func (m Model) current() *secret.Secret {
	if it, ok := m.list.SelectedItem().(entry); ok {
		return it.data
	}
	return nil
}

func (m Model) currentSlug() string {
	if it, ok := m.list.SelectedItem().(entry); ok {
		return it.slug
	}
	return ""
}

func (m Model) focusedField() (secret.Field, bool) {
	s := m.current()
	if s == nil || m.detail.fieldIndex >= len(s.Fields) {
		return secret.Field{}, false
	}
	return s.Fields[m.detail.fieldIndex], true
}

func (m *Model) moveField(delta int) {
	s := m.current()
	if s == nil || len(s.Fields) == 0 {
		return
	}
	m.detail.fieldIndex = (m.detail.fieldIndex + delta + len(s.Fields)) % len(s.Fields)
	m.detail.reveal = false
}

func (m *Model) clampField() {
	s := m.current()
	if s == nil || m.detail.fieldIndex >= len(s.Fields) {
		m.detail.fieldIndex = 0
	}
}
