package tui

import (
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

func (m Model) handleConfirmDeleteKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch shortcut(msg) {
	case "y":
		return m, deleteCmd(m.store, m.deleteSlug)
	case "n", "esc":
		m.screen = screenList
		m.deleteSlug, m.deleteTitle = "", ""
		return m, nil
	}
	return m, nil
}

// confirmDeleteView asks, plainly, before a Secret is gone for good — there
// is no undo (docs/milestone-2.md).
func (m Model) confirmDeleteView() string {
	lines := []string{
		styleTitle.Render("Delete secret"),
		"",
		"Delete " + styleValue.Render(m.deleteTitle) + "? This cannot be undone.",
		"",
		styleHelp.Render("y delete · n/esc cancel"),
	}
	return lipgloss.NewStyle().Padding(1, 2).Width(m.width).Render(lipgloss.JoinVertical(lipgloss.Left, lines...))
}
