package tui

import (
	"errors"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/fbriansyah/go-password-manager/internal/generator"
	"github.com/fbriansyah/go-password-manager/internal/secret"
)

// Bounds the length knob is held to inside the panel, so a value under
// generator.ErrTooShort's floor can only ever arrive from a hand-edited
// configuration file, never from the TUI itself.
const (
	minGeneratedLength = 8
	maxGeneratedLength = 128
)

// genKnob names one row of the generator panel.
type genKnob int

const (
	genKnobLength genKnob = iota
	genKnobUpper
	genKnobDigits
	genKnobSymbols
	genKnobCount
)

// fieldRow is one Field being edited. Which editor it uses is decided by the
// Field Type's EditorKind, not by this form.
type fieldRow struct {
	typeIndex int
	label     textinput.Model
	line      textinput.Model
	area      textarea.Model
	// reveal shows a masked value in plain text while editing, so a Password
	// field can be checked against what is actually stored without leaving
	// the form. It always starts hidden again on the next row (never carried
	// from one field, or one Secret, to another).
	reveal bool
}

func newFieldRow() fieldRow {
	label := textinput.New()
	label.Placeholder = "label"
	label.CharLimit = 64

	line := textinput.New()
	line.Placeholder = "value"

	area := textarea.New()
	area.Placeholder = "note"
	area.SetHeight(3)

	return fieldRow{label: label, line: line, area: area}
}

func (r fieldRow) fieldType() secret.Type { return secret.Types()[r.typeIndex] }

func (r fieldRow) value() string {
	if r.fieldType().Editor == secret.EditorArea {
		return r.area.Value()
	}
	return r.line.Value()
}

func (r *fieldRow) setValue(v string) {
	if r.fieldType().Editor == secret.EditorArea {
		r.area.SetValue(v)
		return
	}
	r.line.SetValue(v)
}

// syncEcho makes the editor hide what is typed for masked types, unless
// reveal has been asked for on this row.
func (r *fieldRow) syncEcho() {
	if r.fieldType().Editor == secret.EditorMasked && !r.reveal {
		r.line.EchoMode = textinput.EchoPassword
		r.line.EchoCharacter = '•'
		return
	}
	r.line.EchoMode = textinput.EchoNormal
}

// formModel is the new-Secret form: Meta at the top, a series of Fields below
// it. Focus runs straight down with tab.
type formModel struct {
	title       textinput.Model
	description textinput.Model
	tags        textinput.Model
	rows        []fieldRow
	focus       int // 0 title, 1 description, 2 tags, then 3+ for field rows
	err         error

	// editSlug is the slug this form is saving back to. Empty means the form
	// is creating a new Secret rather than editing one.
	editSlug string
	// initial is the marshaled form contents at the moment it was built, so
	// dirty can tell an untouched form from one with unsaved changes without
	// keeping a second copy of every field around.
	initial []byte
	// confirmDiscard is true once esc has been pressed on a dirty form and is
	// waiting for a second esc to actually discard it.
	confirmDiscard bool

	// policy is the Generator Policy this form's session is currently using.
	// It starts from configuration, may be changed from the panel below, and
	// outlives any one field it fills (docs/milestone-3.md).
	policy generator.Options

	// The generator panel, open while genOpen is true. It always fills genRow
	// — the row that was focused when ctrl+g opened it — and never any other.
	genOpen      bool
	genRow       int
	genKnob      genKnob
	genLengthBuf string // digits typed for the length knob, reset on any other action
	candidate    string // the password on screen; empty while the length is invalid
}

const metaInputs = 3

// inputsPerRow: the label and the value are one focus stop each.
const inputsPerRow = 2

// newForm starts a blank form. policy is the Generator Policy this session
// currently holds; ctrl+g starts from it and any change to it in the panel is
// the caller's to keep for the forms that come after this one.
func newForm(policy generator.Options) formModel {
	title := textinput.New()
	title.Placeholder = "title, e.g. Facebook"
	title.CharLimit = 120
	title.Focus()

	desc := textinput.New()
	desc.Placeholder = "description (optional)"

	tags := textinput.New()
	tags.Placeholder = "tags, comma separated (optional)"

	row := newFieldRow()
	row.syncEcho()
	m := formModel{title: title, description: desc, tags: tags, rows: []fieldRow{row}, policy: policy}
	m.initial, _ = secret.Marshal(m.secretValue())
	return m
}

// editForm starts a form filled in from an existing Secret, saving back to
// slug. policy is the Generator Policy this session currently holds, same as
// newForm. Built fresh every time — the form must never be reused between
// Secrets (docs/milestone-2.md).
func editForm(policy generator.Options, slug string, s *secret.Secret) formModel {
	title := textinput.New()
	title.Placeholder = "title, e.g. Facebook"
	title.CharLimit = 120
	title.SetValue(s.Meta.Title)
	title.Focus()

	desc := textinput.New()
	desc.Placeholder = "description (optional)"
	desc.SetValue(s.Meta.Description)

	tags := textinput.New()
	tags.Placeholder = "tags, comma separated (optional)"
	tags.SetValue(strings.Join(s.Meta.Tags, ", "))

	rows := make([]fieldRow, 0, len(s.Fields))
	for _, f := range s.Fields {
		row := newFieldRow()
		row.typeIndex = typeIndexFor(f.Type)
		row.label.SetValue(f.Label)
		row.setValue(f.Value)
		row.syncEcho()
		rows = append(rows, row)
	}
	if len(rows) == 0 {
		row := newFieldRow()
		row.syncEcho()
		rows = append(rows, row)
	}

	m := formModel{title: title, description: desc, tags: tags, rows: rows, policy: policy, editSlug: slug}
	m.initial, _ = secret.Marshal(m.secretValue())
	return m
}

// typeIndexFor finds the row index secret.Types() uses for id, defaulting to
// the first type when id is not registered.
func typeIndexFor(id string) int {
	for i, t := range secret.Types() {
		if t.ID == id {
			return i
		}
	}
	return 0
}

// dirty reports whether the form no longer matches what it was built with.
func (m formModel) dirty() bool {
	cur, err := secret.Marshal(m.secretValue())
	if err != nil {
		return true
	}
	return string(cur) != string(m.initial)
}

func (m formModel) focusCount() int { return metaInputs + len(m.rows)*inputsPerRow }

// rowAt turns a focus position into a field row index and which part of that
// row currently has focus.
func (m formModel) rowAt(focus int) (row int, part int, ok bool) {
	if focus < metaInputs {
		return 0, 0, false
	}
	idx := focus - metaInputs
	return idx / inputsPerRow, idx % inputsPerRow, true
}

func (m *formModel) applyFocus() {
	m.title.Blur()
	m.description.Blur()
	m.tags.Blur()
	for i := range m.rows {
		m.rows[i].label.Blur()
		m.rows[i].line.Blur()
		m.rows[i].area.Blur()
	}
	switch m.focus {
	case 0:
		m.title.Focus()
	case 1:
		m.description.Focus()
	case 2:
		m.tags.Focus()
	default:
		row, part, _ := m.rowAt(m.focus)
		if part == 0 {
			m.rows[row].label.Focus()
			return
		}
		if m.rows[row].fieldType().Editor == secret.EditorArea {
			m.rows[row].area.Focus()
			return
		}
		m.rows[row].line.Focus()
	}
}

func (m *formModel) moveFocus(delta int) {
	n := m.focusCount()
	m.focus = (m.focus + delta + n) % n
	m.applyFocus()
}

func (m *formModel) addRow() {
	row := newFieldRow()
	row.syncEcho()
	m.rows = append(m.rows, row)
	m.focus = metaInputs + (len(m.rows)-1)*inputsPerRow
	m.applyFocus()
}

func (m *formModel) removeRow() {
	row, _, ok := m.rowAt(m.focus)
	if !ok || len(m.rows) == 1 {
		return
	}
	m.rows = append(m.rows[:row], m.rows[row+1:]...)
	if m.focus >= m.focusCount() {
		m.focus = m.focusCount() - 1
	}
	m.applyFocus()
}

// cycleType changes the Field Type of the focused row.
func (m *formModel) cycleType(delta int) {
	row, _, ok := m.rowAt(m.focus)
	if !ok {
		return
	}
	types := secret.Types()
	m.rows[row].typeIndex = (m.rows[row].typeIndex + delta + len(types)) % len(types)
	m.rows[row].reveal = false
	m.rows[row].syncEcho()
	m.applyFocus()
}

// toggleReveal shows or hides the value of the focused row in plain text —
// only meaningful on a masked field's value, so it does nothing elsewhere.
func (m *formModel) toggleReveal() {
	row, part, ok := m.rowAt(m.focus)
	if !ok || part != 1 || m.rows[row].fieldType().Editor != secret.EditorMasked {
		return
	}
	m.rows[row].reveal = !m.rows[row].reveal
	m.rows[row].syncEcho()
}

// openGenerator opens the panel over the focused row, only for a Field Type
// the generator is allowed to fill.
func (m *formModel) openGenerator() {
	row, _, ok := m.rowAt(m.focus)
	if !ok || !m.rows[row].fieldType().Generatable {
		m.err = errors.New("this field type cannot be generated")
		return
	}
	m.genRow = row
	m.genOpen = true
	m.genKnob = genKnobLength
	m.genLengthBuf = ""
	m.err = nil
	m.reroll()
}

// closeGenerator closes the panel without touching the Field it was open
// over. The Policy it leaves behind — including a length that was mid-typed —
// is still clamped, so the session never carries an invalid one forward.
func (m *formModel) closeGenerator() {
	m.genOpen = false
	m.genLengthBuf = ""
	m.candidate = ""
	m.clampLength()
}

// acceptGenerator writes the candidate on screen into the Field the panel was
// opened for, then closes the panel the same way esc would.
func (m *formModel) acceptGenerator() {
	if m.candidate == "" {
		return
	}
	m.rows[m.genRow].setValue(m.candidate)
	m.closeGenerator()
}

func (m *formModel) clampLength() {
	if m.policy.Length < minGeneratedLength {
		m.policy.Length = minGeneratedLength
	}
	if m.policy.Length > maxGeneratedLength {
		m.policy.Length = maxGeneratedLength
	}
}

// reroll produces a new candidate from the current Policy. The length may be
// mid-typed and momentarily too short to generate from; the candidate is left
// empty rather than shown stale until it is valid again.
func (m *formModel) reroll() {
	if m.policy.Length < minGeneratedLength {
		m.candidate = ""
		return
	}
	pw, err := generator.Generate(m.policy)
	if err != nil {
		m.candidate = ""
		return
	}
	m.candidate = pw
}

// moveGenKnob changes which knob has focus in the panel.
func (m *formModel) moveGenKnob(delta int) {
	m.genLengthBuf = ""
	m.genKnob = (m.genKnob + genKnob(delta) + genKnobCount) % genKnobCount
}

// adjustGenKnob changes the value of the focused knob: the length by one step,
// a character class on or off either direction.
func (m *formModel) adjustGenKnob(delta int) {
	m.genLengthBuf = ""
	switch m.genKnob {
	case genKnobLength:
		m.policy.Length += delta
		m.clampLength()
	case genKnobUpper:
		m.policy.Upper = !m.policy.Upper
	case genKnobDigits:
		m.policy.Digits = !m.policy.Digits
	case genKnobSymbols:
		m.policy.Symbols = !m.policy.Symbols
	}
	m.reroll()
}

// rerollGenerator produces a fresh candidate from the same Policy — for
// picking a different password without changing any knob.
func (m *formModel) rerollGenerator() {
	m.genLengthBuf = ""
	m.reroll()
}

// typeLength feeds one typed digit into the length knob. Digits accumulate
// until any other panel action resets the buffer (moveGenKnob, adjustGenKnob,
// reroll, accept, or close).
func (m *formModel) typeLength(d rune) {
	if m.genKnob != genKnobLength || len(m.genLengthBuf) >= 3 {
		return
	}
	m.genLengthBuf += string(d)
	n, err := strconv.Atoi(m.genLengthBuf)
	if err != nil {
		return
	}
	if n > maxGeneratedLength {
		n = maxGeneratedLength
		m.genLengthBuf = strconv.Itoa(n)
	}
	m.policy.Length = n
	m.reroll()
}

// secretValue builds a Secret from the form. A Field whose label and value are
// both empty is ignored, so leftover rows are not saved.
func (m formModel) secretValue() *secret.Secret {
	s := &secret.Secret{
		Meta: secret.Meta{
			Title:       strings.TrimSpace(m.title.Value()),
			Description: strings.TrimSpace(m.description.Value()),
		},
	}
	for _, t := range strings.Split(m.tags.Value(), ",") {
		if t = strings.TrimSpace(t); t != "" {
			s.Meta.Tags = append(s.Meta.Tags, t)
		}
	}
	for _, r := range m.rows {
		label, value := strings.TrimSpace(r.label.Value()), r.value()
		if label == "" && strings.TrimSpace(value) == "" {
			continue
		}
		s.Fields = append(s.Fields, secret.Field{
			Type:  r.fieldType().ID,
			Label: label,
			Value: value,
		})
	}
	return s
}

func (m formModel) Update(msg tea.Msg) (formModel, tea.Cmd) {
	var cmd tea.Cmd
	switch m.focus {
	case 0:
		m.title, cmd = m.title.Update(msg)
	case 1:
		m.description, cmd = m.description.Update(msg)
	case 2:
		m.tags, cmd = m.tags.Update(msg)
	default:
		row, part, _ := m.rowAt(m.focus)
		if part == 0 {
			m.rows[row].label, cmd = m.rows[row].label.Update(msg)
			break
		}
		if m.rows[row].fieldType().Editor == secret.EditorArea {
			m.rows[row].area, cmd = m.rows[row].area.Update(msg)
			break
		}
		m.rows[row].line, cmd = m.rows[row].line.Update(msg)
	}
	return m, cmd
}

func (m formModel) View(width int) string {
	title := "New secret"
	if m.editSlug != "" {
		title = "Edit secret"
	}
	lines := []string{
		styleTitle.Render(title),
		"",
		labeled("Title", m.title.View(), m.focus == 0),
		labeled("Description", m.description.View(), m.focus == 1),
		labeled("Tags", m.tags.View(), m.focus == 2),
		"",
	}
	if m.genOpen {
		lines = append(lines, m.generatorView()...)
	} else {
		for i, r := range m.rows {
			focusRow, part, _ := m.rowAt(m.focus)
			active := focusRow == i && m.focus >= metaInputs

			head := styleMuted.Render("field " + strconv.Itoa(i+1) + "  ")
			typeName := r.fieldType().Name
			if active {
				head += styleFocused.Render("‹ " + typeName + " ›")
			} else {
				head += styleMuted.Render("  " + typeName)
			}
			lines = append(lines, head)
			lines = append(lines, labeled("Label", r.label.View(), active && part == 0))

			editor := r.line.View()
			if r.fieldType().Editor == secret.EditorArea {
				editor = r.area.View()
			}
			valueLabel := "Value"
			if r.fieldType().Editor == secret.EditorMasked && r.reveal {
				valueLabel = "Value (revealed)"
			}
			lines = append(lines, labeled(valueLabel, editor, active && part == 1))
			lines = append(lines, "")
		}
	}
	if m.confirmDiscard {
		lines = append(lines, styleErr.Render("unsaved changes — press esc again to discard, any other key to keep editing"))
	} else if m.err != nil {
		lines = append(lines, styleErr.Render(m.err.Error()))
	}
	if m.genOpen {
		lines = append(lines, styleHelp.Render(
			"↑/↓ knob · ←/→ change · digits length · r reroll · enter accept · ctrl+s save default · esc cancel"))
	} else {
		lines = append(lines, styleHelp.Render(
			"tab move · ←/→ change type · ctrl+n add field · ctrl+d remove field · ctrl+g generate · ctrl+r reveal value · ctrl+s save · esc cancel"))
	}
	return lipgloss.NewStyle().Padding(1, 2).Width(width).Render(lipgloss.JoinVertical(lipgloss.Left, lines...))
}

// generatorView draws the generator panel: the four knobs, then the candidate
// on screen — or a hint that the length typed so far is too short to generate
// from, rather than showing a stale password.
func (m formModel) generatorView() []string {
	lines := []string{styleTitle.Render("Generate password — field " + strconv.Itoa(m.genRow+1)), ""}

	knob := func(k genKnob, label, value string) string {
		row := styleLabel.Render(label) + "  " + value
		if m.genKnob == k {
			row = styleFocused.Render(label) + "  " + styleFocused.Render("‹ "+value+" ›")
		}
		return row
	}
	boolValue := func(on bool) string {
		if on {
			return "on"
		}
		return "off"
	}

	lengthValue := strconv.Itoa(m.policy.Length)
	if m.genLengthBuf != "" {
		lengthValue = m.genLengthBuf
	}
	lines = append(lines, knob(genKnobLength, "Length ", lengthValue))
	lines = append(lines, knob(genKnobUpper, "Upper  ", boolValue(m.policy.Upper)))
	lines = append(lines, knob(genKnobDigits, "Digits ", boolValue(m.policy.Digits)))
	lines = append(lines, knob(genKnobSymbols, "Symbols", boolValue(m.policy.Symbols)))
	lines = append(lines, "")

	if m.candidate == "" {
		lines = append(lines, styleMuted.Render("a password needs at least "+strconv.Itoa(minGeneratedLength)+" characters"))
	} else {
		lines = append(lines, styleLabel.Render("Candidate"), styleValue.Render(m.candidate))
	}
	lines = append(lines, "")
	return lines
}

func labeled(label, body string, focused bool) string {
	l := styleLabel.Render(label)
	if focused {
		l = styleFocused.Render(label)
	}
	return lipgloss.JoinVertical(lipgloss.Left, l, body)
}
