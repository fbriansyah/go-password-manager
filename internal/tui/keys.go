package tui

import (
	"runtime"
	"strings"

	tea "charm.land/bubbletea/v2"
)

// keyOS decides whether Command counts as a modifier. It is a var so tests can
// exercise the macOS path from any machine; nothing changes it at runtime.
var keyOS = runtime.GOOS

// shortcut names the binding a key press should be matched against. On macOS a
// leading "super+" — Command — is rewritten to "ctrl+", so ⌘S and ctrl+S both
// reach the same case and ctrl keeps working in the terminals (Terminal.app,
// iTerm2) that swallow Command before it ever arrives. See ADR 0007.
//
// Only the string used for matching is rewritten; msg itself travels on to the
// text inputs untouched.
//
// super+c is the one press left alone: ctrl+c quits, but Command-C means copy
// on macOS, and a user copying text off the screen must not close the Vault.
func shortcut(msg tea.KeyPressMsg) string {
	s := msg.String()
	if keyOS != "darwin" || !strings.HasPrefix(s, "super+") || s == "super+c" {
		return s
	}
	return "ctrl+" + strings.TrimPrefix(s, "super+")
}

// modLabel is how the ctrl modifier is written in the help lines: on macOS both
// keys are offered, because Command is the one that may never arrive.
func modLabel() string {
	if keyOS == "darwin" {
		return "⌘/^"
	}
	return "ctrl+"
}
