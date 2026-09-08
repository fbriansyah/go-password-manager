package tui

import (
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// unlockModel asks for the Master Password. It does not keep the password after
// use; only the Session that Unlock returns survives.
type unlockModel struct {
	input    textinput.Model
	vaultDir string
	err      error
	busy     bool
}

func newUnlock(vaultDir string) unlockModel {
	in := textinput.New()
	in.Placeholder = "master password"
	in.EchoMode = textinput.EchoPassword
	in.EchoCharacter = '•'
	in.Focus()
	in.CharLimit = 256
	return unlockModel{input: in, vaultDir: vaultDir}
}

func (m unlockModel) Update(msg tea.Msg) (unlockModel, tea.Cmd) {
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m unlockModel) View(width int) string {
	var b lipgloss.Style = lipgloss.NewStyle().Padding(1, 2)
	lines := []string{
		styleTitle.Render("gopm") + styleMuted.Render("  "+m.vaultDir),
		"",
		"Open the vault with the master password:",
		m.input.View(),
	}
	switch {
	case m.busy:
		lines = append(lines, "", styleMuted.Render("opening the identity…"))
	case m.err != nil:
		lines = append(lines, "", styleErr.Render(m.err.Error()))
	}
	lines = append(lines, styleHelp.Render("enter opens · esc quits"))
	return b.Width(width).Render(lipgloss.JoinVertical(lipgloss.Left, lines...))
}
