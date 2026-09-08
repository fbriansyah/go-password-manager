// Package tui is the presentation layer. It calls into Vault and crypto, but
// never touches the filesystem or age directly.
package tui

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/fbriansyah/go-password-manager/internal/clipboard"
	"github.com/fbriansyah/go-password-manager/internal/config"
	"github.com/fbriansyah/go-password-manager/internal/crypto"
	"github.com/fbriansyah/go-password-manager/internal/secret"
	"github.com/fbriansyah/go-password-manager/internal/vault"
)

type screen int

const (
	screenUnlock screen = iota
	screenList
	screenForm
)

// entry is one Secret, decrypted into memory during Unlock.
type entry struct {
	slug string
	data *secret.Secret
}

func (e entry) Title() string { return e.data.Meta.Title }

func (e entry) Description() string {
	if e.data.Meta.Description != "" {
		return e.data.Meta.Description
	}
	if len(e.data.Meta.Tags) > 0 {
		return "#" + strings.Join(e.data.Meta.Tags, " #")
	}
	return e.slug
}

// FilterValue lets a search reach the title, the description, and the tags.
func (e entry) FilterValue() string {
	return strings.Join(append([]string{e.data.Meta.Title, e.data.Meta.Description}, e.data.Meta.Tags...), " ")
}

// Model is the whole TUI application.
type Model struct {
	vaultDir string
	cfg      config.Config

	session *crypto.Session
	store   vault.Vault

	screen screen
	unlock unlockModel
	list   list.Model
	detail detailModel
	form   formModel

	status     string
	statusErr  bool
	clearsAt   time.Time
	width      int
	height     int
	quitting   bool
	fatalError error
}

// New prepares the application for the Vault at vaultDir.
func New(vaultDir string, cfg config.Config) Model {
	l := list.New(nil, list.NewDefaultDelegate(), 0, 0)
	l.Title = "Secret"
	l.SetShowHelp(false)
	l.SetStatusBarItemName("secret", "secret")
	return Model{
		vaultDir: vaultDir,
		cfg:      cfg,
		screen:   screenUnlock,
		unlock:   newUnlock(vaultDir),
		list:     l,
	}
}

func (m Model) Init() tea.Cmd { return tea.Batch(tea.SetWindowTitle("gopm"), textinput.Blink) }

// Internal messages.
type (
	unlockedMsg struct {
		session *crypto.Session
		store   vault.Vault
		entries []entry
		skipped []string
	}
	failedMsg  struct{ err error }
	createdMsg struct {
		slug    string
		secret  *secret.Secret
		entries []entry
	}
	copiedMsg struct{ label string }
	tickMsg   time.Time
)

// unlockCmd opens the Identity, then loads every Secret. Because the whole file
// is encrypted, the list can only be shown once this step finished
// (docs/adr/0004).
func unlockCmd(cfg config.Config, vaultDir, password string) tea.Cmd {
	return func() tea.Msg {
		session, err := crypto.Unlock(cfg.PrivateKeyPath, password)
		if err != nil {
			return failedMsg{err}
		}
		store, err := vault.Open(vaultDir, session)
		if err != nil {
			return failedMsg{err}
		}
		entries, skipped, err := loadAll(store)
		if err != nil {
			return failedMsg{err}
		}
		return unlockedMsg{session: session, store: store, entries: entries, skipped: skipped}
	}
}

// loadAll loads every Secret. One that cannot be opened — encrypted for another
// Recipient, say — is skipped and reported rather than failing the whole
// Vault.
func loadAll(store vault.Vault) ([]entry, []string, error) {
	slugs, err := store.List()
	if err != nil {
		return nil, nil, err
	}
	var entries []entry
	var skipped []string
	for _, slug := range slugs {
		s, err := store.Load(slug)
		if err != nil {
			skipped = append(skipped, slug)
			continue
		}
		entries = append(entries, entry{slug: slug, data: s})
	}
	return entries, skipped, nil
}

func createCmd(store vault.Vault, s *secret.Secret) tea.Cmd {
	return func() tea.Msg {
		slug, err := store.Create(s)
		if err != nil {
			return failedMsg{err}
		}
		entries, _, err := loadAll(store)
		if err != nil {
			return failedMsg{err}
		}
		return createdMsg{slug: slug, secret: s, entries: entries}
	}
}

func copyCmd(f secret.Field) tea.Cmd {
	return func() tea.Msg {
		value, err := secret.TypeFor(f.Type).Copy(f.Value)
		if err != nil {
			return failedMsg{err}
		}
		if err := clipboard.Copy(value); err != nil {
			return failedMsg{err}
		}
		return copiedMsg{label: f.Label}
	}
}

func tick() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.layout()
		return m, nil

	case unlockedMsg:
		m.session, m.store = msg.session, msg.store
		m.screen = screenList
		m.setEntries(msg.entries, "")
		if len(msg.skipped) > 0 {
			m.setStatus(fmt.Sprintf("%d secret(s) cannot be opened with this identity: %s",
				len(msg.skipped), strings.Join(msg.skipped, ", ")), true)
		}
		m.layout()
		return m, nil

	case failedMsg:
		if m.screen == screenUnlock {
			m.unlock.busy = false
			m.unlock.err = msg.err
			return m, nil
		}
		if m.screen == screenForm {
			m.form.err = msg.err
			return m, nil
		}
		m.setStatus(msg.err.Error(), true)
		return m, nil

	case createdMsg:
		m.screen = screenList
		m.setEntries(msg.entries, msg.slug)
		m.setStatus("saved as "+msg.slug+vault.Ext, false)
		m.layout()
		return m, nil

	case copiedMsg:
		m.clearsAt = time.Now().Add(clipboard.ClearAfter)
		m.setStatus("", false)
		return m, tick()

	case tickMsg:
		if time.Now().Before(m.clearsAt) {
			return m, tick()
		}
		m.clearsAt = time.Time{}
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	return m.delegate(msg)
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.screen {
	case screenUnlock:
		switch msg.Type {
		case tea.KeyEsc, tea.KeyCtrlC:
			m.quitting = true
			return m, tea.Quit
		case tea.KeyEnter:
			if m.unlock.busy {
				return m, nil
			}
			password := m.unlock.input.Value()
			if password == "" {
				m.unlock.err = errors.New("the master password cannot be empty")
				return m, nil
			}
			m.unlock.busy, m.unlock.err = true, nil
			m.unlock.input.SetValue("")
			return m, unlockCmd(m.cfg, m.vaultDir, password)
		}
		var cmd tea.Cmd
		m.unlock, cmd = m.unlock.Update(msg)
		return m, cmd

	case screenForm:
		switch {
		case msg.Type == tea.KeyEsc:
			m.screen = screenList
			m.setStatus("", false)
			return m, nil
		case msg.Type == tea.KeyCtrlS:
			s := m.form.secretValue()
			if err := s.Validate(); err != nil {
				m.form.err = err
				return m, nil
			}
			return m, createCmd(m.store, s)
		case msg.Type == tea.KeyCtrlN:
			m.form.addRow()
			return m, nil
		case msg.Type == tea.KeyCtrlD:
			m.form.removeRow()
			return m, nil
		case msg.Type == tea.KeyCtrlG:
			m.form.generate()
			return m, nil
		case msg.Type == tea.KeyTab, msg.Type == tea.KeyDown && m.formNavigable():
			m.form.moveFocus(1)
			return m, nil
		case msg.Type == tea.KeyShiftTab, msg.Type == tea.KeyUp && m.formNavigable():
			m.form.moveFocus(-1)
			return m, nil
		case msg.Type == tea.KeyLeft && m.form.focus >= metaInputs && m.formTypeCycle():
			m.form.cycleType(-1)
			return m, nil
		case msg.Type == tea.KeyRight && m.form.focus >= metaInputs && m.formTypeCycle():
			m.form.cycleType(1)
			return m, nil
		}
		var cmd tea.Cmd
		m.form, cmd = m.form.Update(msg)
		return m, cmd

	default: // screenList
		if m.list.FilterState() == list.Filtering {
			var cmd tea.Cmd
			m.list, cmd = m.list.Update(msg)
			return m, cmd
		}
		switch msg.String() {
		case "q", "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		case "n":
			m.form = newForm()
			m.screen = screenForm
			m.setStatus("", false)
			return m, nil
		case "tab":
			m.detail.focused = !m.detail.focused
			m.detail.reveal = false
			return m, nil
		case "esc":
			if m.detail.focused {
				m.detail.focused = false
				m.detail.reveal = false
				return m, nil
			}
		case "r":
			if m.detail.focused {
				m.detail.reveal = !m.detail.reveal
				return m, nil
			}
		case "c":
			if m.detail.focused {
				f, ok := m.focusedField()
				if !ok {
					return m, nil
				}
				return m, copyCmd(f)
			}
		case "j", "down":
			if m.detail.focused {
				m.moveField(1)
				return m, nil
			}
		case "k", "up":
			if m.detail.focused {
				m.moveField(-1)
				return m, nil
			}
		}
		var cmd tea.Cmd
		m.list, cmd = m.list.Update(msg)
		m.clampField()
		return m, cmd
	}
}

// formNavigable keeps up/down arrows moving the cursor inside a multi-line note
// instead of jumping between fields.
func (m Model) formNavigable() bool {
	row, part, ok := m.form.rowAt(m.form.focus)
	if !ok || part == 0 {
		return true
	}
	return m.form.rows[row].fieldType().Editor != secret.EditorArea
}

func (m Model) formTypeCycle() bool {
	_, part, ok := m.form.rowAt(m.form.focus)
	return ok && part == 0
}

func (m Model) delegate(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch m.screen {
	case screenUnlock:
		m.unlock, cmd = m.unlock.Update(msg)
	case screenForm:
		m.form, cmd = m.form.Update(msg)
	default:
		m.list, cmd = m.list.Update(msg)
	}
	return m, cmd
}

func (m *Model) setEntries(entries []entry, selectSlug string) {
	items := make([]list.Item, 0, len(entries))
	selected := 0
	for i, e := range entries {
		items = append(items, e)
		if e.slug == selectSlug {
			selected = i
		}
	}
	m.list.SetItems(items)
	if len(items) > 0 {
		m.list.Select(selected)
	}
	m.detail.fieldIndex = 0
}

func (m Model) current() *secret.Secret {
	if it, ok := m.list.SelectedItem().(entry); ok {
		return it.data
	}
	return nil
}

func (m Model) currentSlug() string {
	if it, ok := m.list.SelectedItem().(entry); ok {
		return it.slug
	}
	return ""
}

func (m Model) focusedField() (secret.Field, bool) {
	s := m.current()
	if s == nil || m.detail.fieldIndex >= len(s.Fields) {
		return secret.Field{}, false
	}
	return s.Fields[m.detail.fieldIndex], true
}

func (m *Model) moveField(delta int) {
	s := m.current()
	if s == nil || len(s.Fields) == 0 {
		return
	}
	m.detail.fieldIndex = (m.detail.fieldIndex + delta + len(s.Fields)) % len(s.Fields)
	m.detail.reveal = false
}

func (m *Model) clampField() {
	s := m.current()
	if s == nil || m.detail.fieldIndex >= len(s.Fields) {
		m.detail.fieldIndex = 0
	}
}

func (m *Model) setStatus(text string, isErr bool) {
	m.status, m.statusErr = text, isErr
}

// layout splits the screen width: the list on the left, the detail on the right.
func (m *Model) layout() {
	if m.width == 0 {
		return
	}
	listWidth := m.width * 2 / 5
	if listWidth < 24 {
		listWidth = 24
	}
	if listWidth > 48 {
		listWidth = 48
	}
	bodyHeight := m.height - 3
	if bodyHeight < 5 {
		bodyHeight = 5
	}
	m.list.SetSize(listWidth, bodyHeight)
}

func (m Model) View() string {
	if m.quitting {
		return ""
	}
	if m.width == 0 {
		return "loading…"
	}
	switch m.screen {
	case screenUnlock:
		return m.unlock.View(m.width)
	case screenForm:
		return m.form.View(m.width)
	}

	listWidth := m.list.Width()
	detailWidth := m.width - listWidth - 4
	if detailWidth < 20 {
		detailWidth = 20
	}
	listPane := stylePane.Width(listWidth).Height(m.list.Height()).Render(m.list.View())
	detailPane := m.detail.View(m.current(), m.currentSlug(), detailWidth, m.list.Height())
	body := lipgloss.JoinHorizontal(lipgloss.Top, listPane, detailPane)
	return lipgloss.JoinVertical(lipgloss.Left, body, m.statusLine())
}

func (m Model) statusLine() string {
	if m.status != "" {
		if m.statusErr {
			return styleErr.Render(m.status)
		}
		return styleOK.Render(m.status)
	}
	if left := time.Until(m.clearsAt); left > 0 {
		return styleOK.Render(fmt.Sprintf("copied to clipboard · cleared in %ds", int(left.Seconds()+0.5)))
	}
	if m.detail.focused {
		return styleHelp.Render("j/k pick field · c copy · r reveal · esc back to list · q quit")
	}
	return styleHelp.Render("↑/↓ pick · / search · tab to detail · n new · q quit")
}
