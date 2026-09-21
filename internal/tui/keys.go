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

// binding is one key a screen answers to (CONTEXT.md: Binding). M is the
// model the key acts on and O what the action reports back. keys are the
// canonical names shortcut() produces; label is how they are written on
// screen when the keys themselves do not read well (arrows, digit ranges).
// when, if set, must hold for the key to count. do performs the action for
// the key that matched and reports the outcome; a nil do lists the key for
// the footer and Help only, and leaves the press to whatever sits beneath the
// Bindings. hint is the footer's word for it (empty keeps it out of the
// footer); desc is the Help line.
type binding[M, O any] struct {
	keys  []string
	label string
	hint  string
	desc  string
	when  func(m *M) bool
	do    func(m *M, k string) O
}

func (b binding[M, O]) matches(m *M, k string) bool {
	for _, want := range b.keys {
		if want == k {
			return b.when == nil || b.when(m)
		}
	}
	return false
}

// keyLabel is the Binding's key as the user reads it: the label as written,
// or the keys joined with ctrl written the way this OS offers it (ADR-0007).
// An explicit label is left alone, so "ctrl+c" can stay ctrl+c on macOS,
// where Command-C is copy and never reaches it.
func (b binding[M, O]) keyLabel() string {
	if b.label != "" {
		return b.label
	}
	return strings.ReplaceAll(strings.Join(b.keys, "/"), "ctrl+", modLabel())
}

// footer is the one-line hint under a screen: every Binding with a hint.
func footer[M, O any](bindings []binding[M, O]) string {
	var parts []string
	for _, b := range bindings {
		if b.hint != "" {
			parts = append(parts, b.keyLabel()+" "+b.hint)
		}
	}
	return strings.Join(parts, " · ")
}

// helpRows lists every Binding as a row of Help.
func helpRows[M, O any](bindings []binding[M, O]) []helpKey {
	keys := make([]helpKey, 0, len(bindings))
	for _, b := range bindings {
		keys = append(keys, helpKey{b.keyLabel(), b.desc})
	}
	return keys
}
