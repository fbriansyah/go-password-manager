package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

// withKeyOS pretends the application is running on the named OS for the rest of
// the test, so the macOS key path can be exercised from any machine.
func withKeyOS(t *testing.T, goos string) {
	t.Helper()
	previous := keyOS
	keyOS = goos
	t.Cleanup(func() { keyOS = previous })
}

func TestShortcutTranslatesCommandOnlyOnMacOS(t *testing.T) {
	cmdS := tea.KeyPressMsg{Code: 's', Mod: tea.ModSuper}
	ctrlS := tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl}
	cmdC := tea.KeyPressMsg{Code: 'c', Mod: tea.ModSuper}

	withKeyOS(t, "darwin")
	if got := shortcut(cmdS); got != "ctrl+s" {
		t.Fatalf("shortcut(⌘s) on darwin = %q, want ctrl+s", got)
	}
	if got := shortcut(ctrlS); got != "ctrl+s" {
		t.Fatalf("shortcut(ctrl+s) on darwin = %q, want it left alone", got)
	}
	// ⌘C is copy on macOS; it must never reach the ctrl+c that quits.
	if got := shortcut(cmdC); got != "super+c" {
		t.Fatalf("shortcut(⌘c) on darwin = %q, want it left alone", got)
	}

	withKeyOS(t, "linux")
	if got := shortcut(cmdS); got != "super+s" {
		t.Fatalf("shortcut(⌘s) on linux = %q, want it left alone", got)
	}
}

func TestCommandSavesTheFormOnMacOS(t *testing.T) {
	withKeyOS(t, "darwin")
	m := unlocked(t)
	m = update(t, m, text("e"))
	if m.screen != screenForm {
		t.Fatalf("screen = %v, want screenForm", m.screen)
	}
	m = run(t, m, key("super+s"))
	if m.screen != screenList {
		t.Fatalf("screen = %v after ⌘s, want screenList (err: %v)", m.screen, m.form.err)
	}
}

func TestCommandDoesNothingOffMacOS(t *testing.T) {
	withKeyOS(t, "linux")
	m := unlocked(t)
	m = update(t, m, text("e"))
	m = run(t, m, key("super+s"))
	if m.screen != screenForm {
		t.Fatalf("screen = %v after ⌘s on linux, want to still be on the form", m.screen)
	}
}

func TestHelpNamesCommandOnlyOnMacOS(t *testing.T) {
	// The help line is wrapped by the renderer, so the assertions look for the
	// modifier alone rather than a whole "<mod>s save" phrase.
	withKeyOS(t, "darwin")
	if got := modLabel(); got != "⌘/^" {
		t.Fatalf("modLabel() on darwin = %q, want both modifiers", got)
	}
	m := unlocked(t)
	m = update(t, m, text("e"))
	view := m.View().Content
	if !strings.Contains(view, "⌘/^") {
		t.Fatalf("the help line does not offer Command on macOS:\n%s", view)
	}
	if !strings.Contains(view, "^s") {
		t.Fatalf("the help line drops the ctrl fallback macOS users may need:\n%s", view)
	}

	withKeyOS(t, "linux")
	if got := modLabel(); got != "ctrl+" {
		t.Fatalf("modLabel() on linux = %q, want ctrl+", got)
	}
	view = m.View().Content
	if strings.Contains(view, "⌘") {
		t.Fatalf("the help line names Command off macOS:\n%s", view)
	}
	if !strings.Contains(view, "ctrl+") {
		t.Fatalf("the help line lost ctrl:\n%s", view)
	}
}
