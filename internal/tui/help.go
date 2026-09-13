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

// listHelp below is written by hand, mirroring the cases in handleKey — the
// same arrangement the list footer lives with. The form's Help, by contrast,
// is drawn from its Bindings (form_keys.go).

// listHelp is Help for the list screen. Both focus modes are shown at once,
// because the thing a footer cannot explain is how they relate: c and r only
// work once tab has moved the focus to the detail pane.
func listHelp() []helpSection {
	return []helpSection{
		{"Secret list", []helpKey{
			{"↑/↓, j/k", "pick a secret"},
			{"/", "search by title, description, or tag"},
			{"tab", "move the focus to the detail pane"},
			{"n", "new secret"},
			{"e", "edit the selected secret"},
			{"d", "delete the selected secret (asks first)"},
		}},
		{"Detail", []helpKey{
			{"j/k, ↑/↓", "pick a field"},
			{"c", "copy the field's value to the clipboard"},
			{"r", "reveal or hide a masked value"},
			{"e", "edit this secret"},
			{"d", "delete this secret (asks first)"},
			{"esc", "back to the list"},
		}},
		{"General", []helpKey{
			{"?", "this help"},
			{"q, ctrl+c", "quit"},
		}},
	}
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
