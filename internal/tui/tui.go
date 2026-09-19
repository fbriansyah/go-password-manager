// Package tui is the presentation layer. It calls into Vault and crypto, but
// never touches the filesystem or age directly.
package tui

import (
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

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
	screenConfirmDelete
	screenHelp
)

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

	// deleteSlug and deleteTitle name the Secret screenConfirmDelete is asking
	// about; deleteIndex is where it sat in the list, so the selection can
	// land on a neighbour once it is gone.
	deleteSlug  string
	deleteTitle string
	deleteIndex int

	// helpReturn is the screen Help was opened from, so closing it lands back
	// there — the form keeps its generator panel open underneath untouched.
	helpReturn screen

	status     string
	statusErr  bool
	clearsAt   time.Time
	ticking    bool // a tick is in flight; see Update
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
	// Quitting is handleKey's decision (q, ctrl+c). The widget's own bindings
	// — v in bubbles v2 — would otherwise close the Vault on a stray key.
	l.DisableQuitKeybindings()
	return Model{
		vaultDir: vaultDir,
		cfg:      cfg,
		screen:   screenUnlock,
		unlock:   newUnlock(vaultDir),
		list:     l,
	}
}

func (m Model) Init() tea.Cmd { return textinput.Blink }

func tick() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg { return tickMsg(t) })
}

// needsClock reports whether something on screen changes with the time: the
// clipboard countdown, or a Code in the detail pane.
func (m Model) needsClock() bool {
	if time.Now().Before(m.clearsAt) {
		return true
	}
	if s := m.current(); s != nil && m.screen == screenList {
		for _, f := range s.Fields {
			if secret.TypeFor(f.Type).Live {
				return true
			}
		}
	}
	return false
}

// Update handles one message, then makes sure a clock is running whenever
// the screen needs one and only one is ever in flight.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	next, cmd := m.update(msg)
	m = next.(Model)
	if !m.ticking && m.needsClock() {
		m.ticking = true
		cmd = tea.Batch(cmd, tick())
	}
	return m, cmd
}

func (m Model) update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
		if m.screen == screenConfirmDelete {
			m.screen = screenList
		}
		m.setStatus(msg.err.Error(), true)
		return m, nil

	case createdMsg:
		m.screen = screenList
		m.setEntries(msg.entries, msg.slug)
		m.setStatus("saved as "+msg.slug+vault.Ext, false)
		m.layout()
		return m, nil

	case savedMsg:
		m.screen = screenList
		m.setEntries(msg.entries, msg.slug)
		m.setStatus("saved as "+msg.slug+vault.Ext, false)
		m.layout()
		return m, nil

	case deletedMsg:
		m.screen = screenList
		m.setEntriesNear(msg.entries, m.deleteIndex)
		m.setStatus("deleted "+m.deleteTitle, false)
		m.deleteSlug, m.deleteTitle = "", ""
		m.layout()
		return m, nil

	case copiedMsg:
		m.clearsAt = time.Now().Add(clipboard.ClearAfter)
		m.setStatus("", false)
		return m, nil

	case policySavedMsg:
		m.setStatus("generator policy saved as default", false)
		return m, nil

	case tickMsg:
		m.ticking = false // Update restarts it if the screen still needs one
		if !time.Now().Before(m.clearsAt) {
			m.clearsAt = time.Time{}
		}
		return m, nil

	case tea.KeyPressMsg:
		return m.handleKey(msg)
	}

	return m.delegate(msg)
}

// handleKey routes a key to the screen that is showing. Each screen's keys
// live in its own file, next to its state and its view.
func (m Model) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch m.screen {
	case screenUnlock:
		return m.handleUnlockKey(msg)
	case screenForm:
		return m.handleFormKey(msg)
	case screenConfirmDelete:
		return m.handleConfirmDeleteKey(msg)
	case screenHelp:
		return m.handleHelpKey(msg)
	default: // screenList
		return m.handleListKey(msg)
	}
}

// handleFormKey lets the form answer the key, then acts on whatever it
// reports back across the seam (form_keys.go).
func (m Model) handleFormKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	var out formOutcome
	var cmd tea.Cmd
	m.form, out, cmd = m.form.handle(msg)
	// A Policy changed in the panel outlives the form (docs/milestone-3.md).
	m.cfg.Generator = m.form.policy
	switch out.kind {
	case formSubmit:
		if m.form.editSlug != "" {
			return m, saveCmd(m.store, m.form.editSlug, out.secret)
		}
		return m, createCmd(m.store, out.secret)
	case formCancelled:
		m.screen = screenList
		m.setStatus("", false)
		return m, nil
	case formSavePolicy:
		return m, saveGeneratorCmd(m.cfg.Generator)
	case formOpenHelp:
		return m.openHelp(), nil
	}
	return m, cmd
}

// handleHelpKey: Help is modal, so only the keys that close it are read; a
// user reading "d delete" cannot delete anything by trying it out.
func (m Model) handleHelpKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if k := shortcut(msg); k == "?" || k == "esc" {
		m.screen = m.helpReturn
	}
	return m, nil
}

// openHelp shows Help for the current screen and remembers where to go back.
func (m Model) openHelp() Model {
	m.helpReturn = m.screen
	m.screen = screenHelp
	return m
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

func (m Model) View() tea.View {
	v := tea.NewView(m.content())
	v.AltScreen = true
	v.WindowTitle = "gopm"
	return v
}

func (m Model) content() string {
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
	case screenConfirmDelete:
		return m.confirmDeleteView()
	case screenHelp:
		if m.helpReturn == screenForm {
			return helpView(m.form.help(), m.width)
		}
		return helpView(listHelp(), m.width)
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
		return styleHelp.Render("j/k pick field · c copy · r reveal · e edit · d delete · esc back to list · ? help · q quit")
	}
	return styleHelp.Render("↑/↓ pick · / search · tab to detail · n new · e edit · d delete · ? help · q quit")
}
