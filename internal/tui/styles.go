package tui

import (
	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/compat"
)

var (
	colorAccent = compat.AdaptiveColor{Light: lipgloss.Color("#5A3FD4"), Dark: lipgloss.Color("#B4A2FF")}
	colorMuted  = compat.AdaptiveColor{Light: lipgloss.Color("#6B6B76"), Dark: lipgloss.Color("#8A8A96")}
	colorErr    = compat.AdaptiveColor{Light: lipgloss.Color("#B3261E"), Dark: lipgloss.Color("#FF8A80")}
	colorOK     = compat.AdaptiveColor{Light: lipgloss.Color("#1B6B3A"), Dark: lipgloss.Color("#7BE3A0")}
	colorLine   = compat.AdaptiveColor{Light: lipgloss.Color("#D5D5DD"), Dark: lipgloss.Color("#3C3C46")}

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
