package tui

import (
	"errors"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
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
	in.SetWidth(inputWidth)
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

// handleUnlockKey answers a key on the unlock screen: enter tries the Master
// Password, esc quits, anything else goes to the input.
func (m Model) handleUnlockKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch shortcut(msg) {
	case "esc", "ctrl+c":
		m.quitting = true
		return m, tea.Quit
	case "enter":
		if m.unlockUI.busy {
			return m, nil
		}
		password := m.unlockUI.input.Value()
		if password == "" {
			m.unlockUI.err = errors.New("the master password cannot be empty")
			return m, nil
		}
		m.unlockUI.busy, m.unlockUI.err = true, nil
		m.unlockUI.input.SetValue("")
		return m, unlockCmd(m.unlock, password)
	}
	var cmd tea.Cmd
	m.unlockUI, cmd = m.unlockUI.Update(msg)
	return m, cmd
}
