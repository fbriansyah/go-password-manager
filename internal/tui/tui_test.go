package tui

import (
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/fbriansyah/go-password-manager/internal/config"
	"github.com/fbriansyah/go-password-manager/internal/crypto"
	"github.com/fbriansyah/go-password-manager/internal/secret"
	"github.com/fbriansyah/go-password-manager/internal/vault"
)

const testPassword = "master-password"

// unlocked prepares a Vault holding one Secret, then runs the Unlock flow the
// way a user does: type the password, press enter.
func unlocked(t *testing.T) Model {
	t.Helper()
	keyDir, vaultDir := t.TempDir(), t.TempDir()
	idPath := filepath.Join(keyDir, "identity.age")
	recPath := filepath.Join(keyDir, "recipient.pub")
	if err := crypto.GenerateKeypair(idPath, recPath, testPassword); err != nil {
		t.Fatalf("GenerateKeypair: %v", err)
	}
	session, err := crypto.Unlock(idPath, testPassword)
	if err != nil {
		t.Fatalf("Unlock: %v", err)
	}
	store, err := vault.Open(vaultDir, session)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if _, err := store.Create(&secret.Secret{
		Meta: secret.Meta{Title: "Facebook", Description: "Facebook Credential", Tags: []string{"app"}},
		Fields: []secret.Field{
			{Type: "tx", Label: "Username", Value: "user@mail.com"},
			{Type: "ps", Label: "Password", Value: "s3cret-value"},
		},
	}); err != nil {
		t.Fatalf("Create: %v", err)
	}

	m := New(vaultDir, config.Config{PrivateKeyPath: idPath, PublicKeyPath: recPath})
	m = update(t, m, tea.WindowSizeMsg{Width: 120, Height: 32})
	m = update(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(testPassword)})
	m = run(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.screen != screenList {
		t.Fatalf("screen = %v, want screenList", m.screen)
	}
	return m
}

// update sends one message and ignores the command it produces.
func update(t *testing.T, m Model, msg tea.Msg) Model {
	t.Helper()
	next, _ := m.Update(msg)
	return next.(Model)
}

// run sends one message, runs the command it produces, and feeds the result
// back in — standing in for the bubbletea loop in tests.
func run(t *testing.T, m Model, msg tea.Msg) Model {
	t.Helper()
	next, cmd := m.Update(msg)
	m = next.(Model)
	if cmd == nil {
		return m
	}
	out := cmd()
	if out == nil {
		return m
	}
	next, _ = m.Update(out)
	return next.(Model)
}

func TestUnlockLoadsTheSecretList(t *testing.T) {
	m := unlocked(t)
	if len(m.list.Items()) != 1 {
		t.Fatalf("item count = %d, want 1", len(m.list.Items()))
	}
	view := m.View()
	if !strings.Contains(view, "Facebook") {
		t.Fatalf("the title is not on screen:\n%s", view)
	}
	if !strings.Contains(view, "Username") {
		t.Fatalf("the detail pane does not show the field:\n%s", view)
	}
}

func TestWrongPasswordStaysOnUnlockScreen(t *testing.T) {
	m := unlocked(t)
	m.screen = screenUnlock
	m.unlock = newUnlock(m.vaultDir)
	m = update(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("very-wrong")})
	m = run(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.screen != screenUnlock {
		t.Fatal("a wrong password must not open the vault")
	}
	if m.unlock.err == nil {
		t.Fatal("want an error message on the unlock screen")
	}
	if strings.Contains(m.View(), "very-wrong") {
		t.Fatal("the typed password is visible on screen")
	}
}

func TestPasswordStaysHiddenUntilR(t *testing.T) {
	m := unlocked(t)
	m = update(t, m, tea.KeyMsg{Type: tea.KeyTab})                       // focus the detail pane
	m = update(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")}) // move to the Password field
	if strings.Contains(m.View(), "s3cret-value") {
		t.Fatal("the password is shown without being asked for")
	}
	m = update(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")})
	if !strings.Contains(m.View(), "s3cret-value") {
		t.Fatalf("r did not reveal the value:\n%s", m.View())
	}
	// Moving to another field hides the value that was revealed.
	m = update(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	if strings.Contains(m.View(), "s3cret-value") {
		t.Fatal("the value stayed revealed after moving to another field")
	}
}

func TestCreateNewSecretThroughTheForm(t *testing.T) {
	m := unlocked(t)
	m = update(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	if m.screen != screenForm {
		t.Fatalf("screen = %v, want screenForm", m.screen)
	}
	m = update(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("Gmail")})
	m = update(t, m, tea.KeyMsg{Type: tea.KeyTab}) // description
	m = update(t, m, tea.KeyMsg{Type: tea.KeyTab}) // tags
	m = update(t, m, tea.KeyMsg{Type: tea.KeyTab}) // label of the first field
	m = update(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("Password")})
	m = update(t, m, tea.KeyMsg{Type: tea.KeyRight}) // tx -> ps
	m = update(t, m, tea.KeyMsg{Type: tea.KeyTab})   // value
	m = update(t, m, tea.KeyMsg{Type: tea.KeyCtrlG}) // generate

	m = run(t, m, tea.KeyMsg{Type: tea.KeyCtrlS})
	if m.screen != screenList {
		t.Fatalf("screen = %v after saving, want screenList (err: %v)", m.screen, m.form.err)
	}
	if len(m.list.Items()) != 2 {
		t.Fatalf("item count = %d, want 2", len(m.list.Items()))
	}
	s, err := m.store.Load("gmail")
	if err != nil {
		t.Fatalf("Load gmail: %v", err)
	}
	if len(s.Fields) != 1 || s.Fields[0].Type != "ps" || s.Fields[0].Label != "Password" {
		t.Fatalf("the stored field is wrong: %+v", s.Fields)
	}
	if len(s.Fields[0].Value) != 20 {
		t.Fatalf("the generator did not fill the value: %q", s.Fields[0].Value)
	}
}

func TestFormRefusesASecretWithoutATitle(t *testing.T) {
	m := unlocked(t)
	m = update(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	m = run(t, m, tea.KeyMsg{Type: tea.KeyCtrlS})
	if m.screen != screenForm {
		t.Fatal("a form without a title must not be saved")
	}
	if m.form.err == nil {
		t.Fatal("want an error message on the form")
	}
}

func TestDuplicateTitleIsRefusedWithAClearMessage(t *testing.T) {
	m := unlocked(t)
	m = update(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	m = update(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("Facebook")})
	m = run(t, m, tea.KeyMsg{Type: tea.KeyCtrlS})
	if m.screen != screenForm {
		t.Fatal("a duplicate title must not be saved")
	}
	if m.form.err == nil || !strings.Contains(m.form.err.Error(), "already exists") {
		t.Fatalf("the error message does not explain the collision: %v", m.form.err)
	}
}
