package tui

import (
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/fbriansyah/go-password-manager/internal/generator"
	"github.com/fbriansyah/go-password-manager/internal/secret"
)

// fieldRow adalah satu Field yang sedang disunting. Editor yang dipakai
// ditentukan oleh EditorKind milik Field Type, bukan oleh form ini.
type fieldRow struct {
	typeIndex int
	label     textinput.Model
	line      textinput.Model
	area      textarea.Model
}

func newFieldRow() fieldRow {
	label := textinput.New()
	label.Placeholder = "label"
	label.CharLimit = 64

	line := textinput.New()
	line.Placeholder = "nilai"

	area := textarea.New()
	area.Placeholder = "catatan"
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

// syncEcho membuat editor menyembunyikan ketikan untuk tipe bertopeng.
func (r *fieldRow) syncEcho() {
	if r.fieldType().Editor == secret.EditorMasked {
		r.line.EchoMode = textinput.EchoPassword
		r.line.EchoCharacter = '•'
		return
	}
	r.line.EchoMode = textinput.EchoNormal
}

// formModel adalah form pembuatan Secret baru: Meta di atas, sederet Field di
// bawahnya. Fokus berjalan lurus dari atas ke bawah dengan tab.
type formModel struct {
	title       textinput.Model
	description textinput.Model
	tags        textinput.Model
	rows        []fieldRow
	focus       int // 0 judul, 1 deskripsi, 2 tag, lalu 3+ untuk baris field
	err         error
}

const metaInputs = 3

// inputsPerRow: label dan nilai masing-masing satu titik fokus.
const inputsPerRow = 2

func newForm() formModel {
	title := textinput.New()
	title.Placeholder = "judul, mis. Facebook"
	title.CharLimit = 120
	title.Focus()

	desc := textinput.New()
	desc.Placeholder = "deskripsi (opsional)"

	tags := textinput.New()
	tags.Placeholder = "tag, dipisah koma (opsional)"

	row := newFieldRow()
	row.syncEcho()
	return formModel{title: title, description: desc, tags: tags, rows: []fieldRow{row}}
}

func (m formModel) focusCount() int { return metaInputs + len(m.rows)*inputsPerRow }

// rowAt menerjemahkan posisi fokus menjadi indeks baris field dan bagian mana
// dari baris itu yang sedang difokus.
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

// cycleType mengganti Field Type baris yang sedang difokus.
func (m *formModel) cycleType(delta int) {
	row, _, ok := m.rowAt(m.focus)
	if !ok {
		return
	}
	types := secret.Types()
	m.rows[row].typeIndex = (m.rows[row].typeIndex + delta + len(types)) % len(types)
	m.rows[row].syncEcho()
	m.applyFocus()
}

// generate mengisi baris yang sedang difokus dengan password acak, hanya untuk
// tipe yang memang boleh dibangkitkan.
func (m *formModel) generate() {
	row, _, ok := m.rowAt(m.focus)
	if !ok || !m.rows[row].fieldType().Generatable {
		return
	}
	pw, err := generator.Generate(generator.Default())
	if err != nil {
		m.err = err
		return
	}
	m.rows[row].setValue(pw)
	m.err = nil
}

// secretValue menyusun Secret dari isi form. Field yang label dan nilainya
// sama-sama kosong diabaikan, sehingga baris sisa tidak ikut tersimpan.
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
	lines := []string{
		styleTitle.Render("Secret baru"),
		"",
		labeled("Judul", m.title.View(), m.focus == 0),
		labeled("Deskripsi", m.description.View(), m.focus == 1),
		labeled("Tag", m.tags.View(), m.focus == 2),
		"",
	}
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
		lines = append(lines, labeled("Nilai", editor, active && part == 1))
		lines = append(lines, "")
	}
	if m.err != nil {
		lines = append(lines, styleErr.Render(m.err.Error()))
	}
	lines = append(lines, styleHelp.Render(
		"tab pindah · ←/→ ganti tipe · ctrl+n tambah field · ctrl+d hapus field · ctrl+g generate · ctrl+s simpan · esc batal"))
	return lipgloss.NewStyle().Padding(1, 2).Width(width).Render(lipgloss.JoinVertical(lipgloss.Left, lines...))
}

func labeled(label, body string, focused bool) string {
	l := styleLabel.Render(label)
	if focused {
		l = styleFocused.Render(label)
	}
	return lipgloss.JoinVertical(lipgloss.Left, l, body)
}
