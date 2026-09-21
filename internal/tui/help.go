package tui

import (
	"charm.land/lipgloss/v2"
)

// helpKey is one row of Help: the key as the user presses it, and what it does.
type helpKey struct{ key, desc string }

// helpSection groups the rows of Help under a heading.
type helpSection struct {
	title string
	keys  []helpKey
}

// helpView draws sections as a two-column key reference.
func helpView(sections []helpSection, width int) string {
	keyWidth := 0
	for _, s := range sections {
		for _, k := range s.keys {
			if w := lipgloss.Width(k.key); w > keyWidth {
				keyWidth = w
			}
		}
	}
	keyStyle := styleFocused.Width(keyWidth)

	lines := []string{styleTitle.Render("Help")}
	for _, s := range sections {
		lines = append(lines, "", styleLabel.Render(s.title))
		for _, k := range s.keys {
			lines = append(lines, "  "+keyStyle.Render(k.key)+"  "+k.desc)
		}
	}
	lines = append(lines, styleHelp.Render("? or esc close"))
	return lipgloss.NewStyle().Padding(1, 2).Width(width).Render(lipgloss.JoinVertical(lipgloss.Left, lines...))
}
