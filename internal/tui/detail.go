package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/fbriansyah/go-password-manager/internal/secret"
)

// detailModel is the right pane: the contents of the Secret highlighted in the
// list. It only displays; Model owns the Secret state.
type detailModel struct {
	fieldIndex int
	reveal     bool
	focused    bool // true when this pane is the one receiving keys
}

func (d detailModel) View(s *secret.Secret, slug string, width, height int) string {
	pane := stylePane.Width(width).Height(height)
	if s == nil {
		return pane.Render(styleMuted.Render("Empty vault.\n\nPress n to create the first secret."))
	}

	head := []string{styleTitle.Render(s.Meta.Title)}
	if s.Meta.Description != "" {
		head = append(head, styleMuted.Render(s.Meta.Description))
	}
	if len(s.Meta.Tags) > 0 {
		head = append(head, styleMuted.Render("#"+strings.Join(s.Meta.Tags, " #")))
	}
	head = append(head, styleMuted.Render(slug+".gopm"), "")

	body := head
	if len(s.Fields) == 0 {
		body = append(body, styleMuted.Render("(no fields)"))
	}
	for i, f := range s.Fields {
		t := secret.TypeFor(f.Type)
		marker := "  "
		label := styleLabel.Render(f.Label)
		if d.focused && i == d.fieldIndex {
			marker = styleFocused.Render("▸ ")
			label = styleFocused.Render(f.Label)
		}
		shown := t.Render(f.Value, d.reveal && i == d.fieldIndex)
		body = append(body, marker+label, "  "+styleValue.Render(orDash(shown)), "")
	}
	return pane.Render(lipgloss.JoinVertical(lipgloss.Left, body...))
}

func orDash(s string) string {
	if strings.TrimSpace(s) == "" {
		return "—"
	}
	return s
}
