package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/fbriansyah/go-password-manager/internal/secret"
)

// formOutcomeKind says what the form wants from Model after a key, when it
// wants anything at all. Everything the form can do to itself it does
// itself; only leaving the form, or reaching past it, is reported.
type formOutcomeKind int

const (
	formNone formOutcomeKind = iota
	formSubmit
	formCancelled
	formSavePolicy // write the form's Policy to configuration as the default
	formOpenHelp
)

// formOutcome is what crosses the seam: the kind, plus the Secret to save
// when the kind is formSubmit.
type formOutcome struct {
	kind   formOutcomeKind
	secret *secret.Secret
}

// formBinding is a Binding of the form: it acts on the formModel and reports
// a formOutcome.
type formBinding = binding[formModel, formOutcome]

func none(f func(m *formModel)) func(*formModel, string) formOutcome {
	return func(m *formModel, _ string) formOutcome { f(m); return formOutcome{} }
}

func report(kind formOutcomeKind) func(*formModel, string) formOutcome {
	return func(*formModel, string) formOutcome { return formOutcome{kind: kind} }
}

// dir turns an arrow key into a direction: up and left step back.
func dir(k string) int {
	if k == "up" || k == "left" {
		return -1
	}
	return 1
}

var digitKeys = []string{"0", "1", "2", "3", "4", "5", "6", "7", "8", "9"}

// formBindings are the keys of the form itself, in the order the footer and
// Help show them.
var formBindings = []formBinding{
	{keys: []string{"tab"}, hint: "move", desc: "move to the next input",
		do: none(func(m *formModel) { m.moveFocus(1) })},
	{keys: []string{"shift+tab"}, desc: "move to the previous input",
		do: none(func(m *formModel) { m.moveFocus(-1) })},
	{keys: []string{"up", "down"}, label: "↑/↓", desc: "move between inputs (inside a note: between lines)",
		when: (*formModel).navigable,
		do: func(m *formModel, k string) formOutcome {
			m.moveFocus(dir(k))
			return formOutcome{}
		}},
	{keys: []string{"left", "right"}, label: "←/→", hint: "change type", desc: "change the field type (on a field's label)",
		when: (*formModel).onTypeCell,
		do: func(m *formModel, k string) formOutcome {
			m.cycleType(dir(k))
			return formOutcome{}
		}},
	{keys: []string{"ctrl+n"}, hint: "add field", desc: "add a field below the last one",
		do: none((*formModel).addRow)},
	{keys: []string{"ctrl+d"}, hint: "remove field", desc: "remove the focused field (the last one stays)",
		do: none((*formModel).removeRow)},
	{keys: []string{"ctrl+g"}, hint: "generate", desc: "generate a password into the focused field",
		do: none((*formModel).openGenerator)},
	{keys: []string{"ctrl+r"}, hint: "reveal value", desc: "reveal or hide a masked value while editing it",
		do: none((*formModel).toggleReveal)},
	{keys: []string{"ctrl+s"}, hint: "save", desc: "save the secret",
		do: func(m *formModel, _ string) formOutcome {
			s := m.secretValue()
			if err := s.Validate(); err != nil {
				m.err = err
				return formOutcome{}
			}
			return formOutcome{kind: formSubmit, secret: s}
		}},
	{keys: []string{"esc"}, hint: "cancel", desc: "leave the form (asks first when there are unsaved changes)",
		do: func(m *formModel, _ string) formOutcome {
			if m.dirty() {
				m.confirmDiscard = true
				return formOutcome{}
			}
			return formOutcome{kind: formCancelled}
		}},
}

// panelBindings are the keys of the generator panel. While it is open they
// replace formBindings entirely: ctrl+s here saves the Policy, not the
// Secret, and nothing falls through to the field rows underneath.
var panelBindings = []formBinding{
	{keys: []string{"up", "down"}, label: "↑/↓", hint: "knob", desc: "pick a knob",
		do: func(m *formModel, k string) formOutcome {
			m.moveGenKnob(dir(k))
			return formOutcome{}
		}},
	{keys: []string{"left", "right"}, label: "←/→", hint: "change", desc: "change the knob",
		do: func(m *formModel, k string) formOutcome {
			m.adjustGenKnob(dir(k))
			return formOutcome{}
		}},
	{keys: digitKeys, label: "0-9", hint: "length", desc: "type a length",
		do: func(m *formModel, k string) formOutcome { m.typeLength(rune(k[0])); return formOutcome{} }},
	{keys: []string{"r"}, hint: "reroll", desc: "reroll the candidate",
		do: none((*formModel).rerollGenerator)},
	{keys: []string{"enter"}, hint: "accept", desc: "accept the candidate into the field",
		do: none((*formModel).acceptGenerator)},
	{keys: []string{"ctrl+s"}, hint: "save default", desc: "save these knobs as the default generator policy",
		do: report(formSavePolicy)},
	{keys: []string{"esc"}, hint: "cancel", desc: "close the panel, leaving the field untouched",
		do: none((*formModel).closeGenerator)},
	{keys: []string{"?"}, hint: "help", desc: "this help",
		do: report(formOpenHelp)},
}

// bindings are the Bindings in force right now: the panel's while it is open.
func (m formModel) bindings() []formBinding {
	if m.genOpen {
		return panelBindings
	}
	return formBindings
}

// handle answers one key. It walks the active Bindings first; in the form
// anything they do not claim falls through to the focused input, in the
// panel it is dropped.
func (m formModel) handle(msg tea.KeyPressMsg) (formModel, formOutcome, tea.Cmd) {
	k := shortcut(msg)
	if m.confirmDiscard {
		// Waiting for a second esc: anything else keeps editing, unread.
		if k == "esc" {
			return m, formOutcome{kind: formCancelled}, nil
		}
		m.confirmDiscard = false
		return m, formOutcome{}, nil
	}
	for _, b := range m.bindings() {
		if b.matches(&m, k) {
			return m, b.do(&m, k), nil
		}
	}
	if m.genOpen {
		return m, formOutcome{}, nil
	}
	var cmd tea.Cmd
	m, cmd = m.Update(msg)
	return m, formOutcome{}, cmd
}

// footer is the one-line hint under the form.
func (m formModel) footer() string { return footer(m.bindings()) }

// help is Help for whichever of the form or the panel is showing.
func (m formModel) help() []helpSection {
	title := "Secret form"
	if m.genOpen {
		title = "Generate password"
	}
	return []helpSection{{title, helpRows(m.bindings())}}
}

// navigable reports whether up/down move between inputs rather than lines:
// true everywhere but inside a multi-line note.
func (m *formModel) navigable() bool {
	row, part, ok := m.rowAt(m.focus)
	if !ok || part == 0 {
		return true
	}
	return m.rows[row].fieldType().Editor != secret.EditorArea
}

// onTypeCell reports whether the focus is on a field's label, where the
// Field Type is changed.
func (m *formModel) onTypeCell() bool {
	_, part, ok := m.rowAt(m.focus)
	return ok && part == 0
}
