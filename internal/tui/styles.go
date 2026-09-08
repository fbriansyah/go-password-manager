package tui

import "github.com/charmbracelet/lipgloss"

var (
	colorAccent = lipgloss.AdaptiveColor{Light: "#5A3FD4", Dark: "#B4A2FF"}
	colorMuted  = lipgloss.AdaptiveColor{Light: "#6B6B76", Dark: "#8A8A96"}
	colorErr    = lipgloss.AdaptiveColor{Light: "#B3261E", Dark: "#FF8A80"}
	colorOK     = lipgloss.AdaptiveColor{Light: "#1B6B3A", Dark: "#7BE3A0"}
	colorLine   = lipgloss.AdaptiveColor{Light: "#D5D5DD", Dark: "#3C3C46"}

	styleTitle   = lipgloss.NewStyle().Bold(true).Foreground(colorAccent)
	styleMuted   = lipgloss.NewStyle().Foreground(colorMuted)
	styleErr     = lipgloss.NewStyle().Foreground(colorErr)
	styleOK      = lipgloss.NewStyle().Foreground(colorOK)
	styleLabel   = lipgloss.NewStyle().Foreground(colorMuted)
	styleValue   = lipgloss.NewStyle().Bold(true)
	styleFocused = lipgloss.NewStyle().Foreground(colorAccent).Bold(true)

	stylePane = lipgloss.NewStyle().
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(colorLine).
			Padding(0, 1)

	styleHelp = lipgloss.NewStyle().Foreground(colorMuted).MarginTop(1)
)
