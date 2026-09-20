package tui

import (
	"strings"

	"charm.land/bubbles/v2/list"

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
