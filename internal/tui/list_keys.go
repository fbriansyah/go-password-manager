package tui

import (
	"slices"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
)

// listBinding is a Binding of the list screen: it acts on the whole Model,
// because its keys open other screens, and reports the Cmd to run, if any.
type listBinding = binding[Model, tea.Cmd]

// act wraps an action that changes the Model and runs nothing.
func act(f func(m *Model)) func(*Model, string) tea.Cmd {
	return func(m *Model, _ string) tea.Cmd { f(m); return nil }
}

// withCurrent runs f only when a Secret is selected; an empty Vault has
// nothing to edit or delete.
func withCurrent(f func(m *Model)) func(*Model, string) tea.Cmd {
	return act(func(m *Model) {
		if m.current() != nil {
			f(m)
		}
	})
}

func (m *Model) openNew() {
	m.form = newForm(m.loc.Config.Generator)
	m.screen = screenForm
	m.setStatus("", false)
}

func (m *Model) openEdit() {
	m.form = editForm(m.loc.Config.Generator, m.currentSlug(), m.current())
	m.screen = screenForm
	m.setStatus("", false)
}

func (m *Model) askDelete() {
	m.deleteSlug = m.currentSlug()
	m.deleteTitle = m.current().Meta.Title
	m.deleteIndex = m.list.Index()
	m.screen = screenConfirmDelete
}

func (m *Model) toggleFocus() {
	m.detail.focused = !m.detail.focused
	m.detail.reveal = false
}

func (m *Model) leaveDetail() {
	m.detail.focused = false
	m.detail.reveal = false
}

func (m *Model) toggleReveal() { m.detail.reveal = !m.detail.reveal }

func (m *Model) quit() { m.quitting = true }

// listFocusBindings are the keys while the list has the focus. Picking and
// searching are the widget's own; they are listed here with no action so the
// footer and Help can name them.
var listFocusBindings = []listBinding{
	{keys: []string{"up", "down", "j", "k"}, label: "↑/↓, j/k", hint: "pick", desc: "pick a secret"},
	{keys: []string{"/"}, hint: "search", desc: "search by title, description, or tag"},
	{keys: []string{"tab"}, hint: "to detail", desc: "move the focus to the detail pane",
		do: act((*Model).toggleFocus)},
	{keys: []string{"n"}, hint: "new", desc: "new secret",
		do: act((*Model).openNew)},
	{keys: []string{"e"}, hint: "edit", desc: "edit the selected secret",
		do: withCurrent((*Model).openEdit)},
	{keys: []string{"d"}, hint: "delete", desc: "delete the selected secret (asks first)",
		do: withCurrent((*Model).askDelete)},
}

// detailFocusBindings are the keys while the detail pane has the focus.
var detailFocusBindings = []listBinding{
	{keys: []string{"j", "k", "up", "down"}, label: "j/k, ↑/↓", hint: "pick field", desc: "pick a field",
		do: func(m *Model, k string) tea.Cmd {
			if k == "j" || k == "down" {
				m.moveField(1)
			} else {
				m.moveField(-1)
			}
			return nil
		}},
	{keys: []string{"c"}, hint: "copy", desc: "copy the field's value to the clipboard",
		do: func(m *Model, _ string) tea.Cmd {
			f, ok := m.focusedField()
			if !ok {
				return nil
			}
			return copyCmd(f)
		}},
	{keys: []string{"r"}, hint: "reveal", desc: "reveal or hide a masked value (the seed, for a TOTP field)",
		do: act((*Model).toggleReveal)},
	{keys: []string{"e"}, hint: "edit", desc: "edit this secret",
		do: withCurrent((*Model).openEdit)},
	{keys: []string{"d"}, hint: "delete", desc: "delete this secret (asks first)",
		do: withCurrent((*Model).askDelete)},
	{keys: []string{"tab"}, desc: "move the focus back to the list",
		do: act((*Model).toggleFocus)},
	{keys: []string{"esc"}, hint: "back to list", desc: "back to the list",
		do: act((*Model).leaveDetail)},
}

// generalBindings hold in both focus modes.
var generalBindings = []listBinding{
	{keys: []string{"?"}, hint: "help", desc: "this help",
		do: func(m *Model, _ string) tea.Cmd { *m = m.openHelp(); return nil }},
	{keys: []string{"q", "ctrl+c"}, label: "q, ctrl+c", hint: "quit", desc: "quit",
		do: func(m *Model, _ string) tea.Cmd { m.quit(); return tea.Quit }},
}

// listBindings are the Bindings in force right now: those of the pane with
// the focus, then the general ones.
func (m Model) listBindings() []listBinding {
	if m.detail.focused {
		return slices.Concat(detailFocusBindings, generalBindings)
	}
	return slices.Concat(listFocusBindings, generalBindings)
}

// handleListKey answers a key on the list screen. While a search is being
// typed every key is the search's; otherwise the active Bindings are walked
// first, and anything they do not claim falls through to the list widget.
func (m Model) handleListKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if m.list.FilterState() == list.Filtering {
		var cmd tea.Cmd
		m.list, cmd = m.list.Update(msg)
		return m, cmd
	}
	k := shortcut(msg)
	for _, b := range m.listBindings() {
		if b.matches(&m, k) && b.do != nil {
			return m, b.do(&m, k)
		}
	}
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	m.clampField()
	return m, cmd
}

// listFooter is the one-line hint under the list screen.
func (m Model) listFooter() string { return footer(m.listBindings()) }

// listHelp is Help for the list screen. Both focus modes are shown at once,
// because the thing a footer cannot explain is how they relate: c and r only
// work once tab has moved the focus to the detail pane.
func listHelp() []helpSection {
	return []helpSection{
		{"Secret list", helpRows(listFocusBindings)},
		{"Detail", helpRows(detailFocusBindings)},
		{"General", helpRows(generalBindings)},
	}
}
