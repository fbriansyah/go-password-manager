package tui

import (
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/fbriansyah/go-password-manager/internal/config"
	"github.com/fbriansyah/go-password-manager/internal/crypto"
	"github.com/fbriansyah/go-password-manager/internal/generator"
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

	// config.Load always seeds Generator with generator.Default() before a file
	// can override it; a hand-built Config here does the same so the panel is
	// never opened onto the zero value.
	m := New(vaultDir, config.Config{PrivateKeyPath: idPath, PublicKeyPath: recPath, Generator: generator.Default()})
	m = update(t, m, tea.WindowSizeMsg{Width: 120, Height: 32})
	m = update(t, m, text(testPassword))
	m = run(t, m, key("enter"))
	if m.screen != screenList {
		t.Fatalf("screen = %v, want screenList", m.screen)
	}
	return m
}

// key builds the tea.KeyPressMsg for one of the named keys this test suite
// presses. Bubble Tea v2 dropped the v1 tea.KeyMsg.Type enum, so tests build
// the Code/Mod pair the real key would carry.
func key(name string) tea.KeyPressMsg {
	switch name {
	case "enter":
		return tea.KeyPressMsg{Code: tea.KeyEnter}
	case "esc":
		return tea.KeyPressMsg{Code: tea.KeyEsc}
	case "tab":
		return tea.KeyPressMsg{Code: tea.KeyTab}
	case "up":
		return tea.KeyPressMsg{Code: tea.KeyUp}
	case "down":
		return tea.KeyPressMsg{Code: tea.KeyDown}
	case "left":
		return tea.KeyPressMsg{Code: tea.KeyLeft}
	case "right":
		return tea.KeyPressMsg{Code: tea.KeyRight}
	case "backspace":
		return tea.KeyPressMsg{Code: tea.KeyBackspace}
	case "ctrl+g":
		return tea.KeyPressMsg{Code: 'g', Mod: tea.ModCtrl}
	case "ctrl+r":
		return tea.KeyPressMsg{Code: 'r', Mod: tea.ModCtrl}
	case "ctrl+s":
		return tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl}
	case "super+s":
		return tea.KeyPressMsg{Code: 's', Mod: tea.ModSuper}
	case "super+g":
		return tea.KeyPressMsg{Code: 'g', Mod: tea.ModSuper}
	case "super+c":
		return tea.KeyPressMsg{Code: 'c', Mod: tea.ModSuper}
	}
	panic("key: unknown key name " + name)
}

// text builds the tea.KeyPressMsg a normal typed rune (or run of runes, for
// convenience) arrives as in v2 — Code holds the first rune, Text the string
// widgets actually insert.
func text(s string) tea.KeyPressMsg {
	r := []rune(s)
	return tea.KeyPressMsg{Code: r[0], Text: s}
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
	view := m.View().Content
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
	m = update(t, m, text("very-wrong"))
	m = run(t, m, key("enter"))
	if m.screen != screenUnlock {
		t.Fatal("a wrong password must not open the vault")
	}
	if m.unlock.err == nil {
		t.Fatal("want an error message on the unlock screen")
	}
	if strings.Contains(m.View().Content, "very-wrong") {
		t.Fatal("the typed password is visible on screen")
	}
}

func TestPasswordStaysHiddenUntilR(t *testing.T) {
	m := unlocked(t)
	m = update(t, m, key("tab")) // focus the detail pane
	m = update(t, m, text("j"))  // move to the Password field
	if strings.Contains(m.View().Content, "s3cret-value") {
		t.Fatal("the password is shown without being asked for")
	}
	m = update(t, m, text("r"))
	if !strings.Contains(m.View().Content, "s3cret-value") {
		t.Fatalf("r did not reveal the value:\n%s", m.View().Content)
	}
	// Moving to another field hides the value that was revealed.
	m = update(t, m, text("k"))
	if strings.Contains(m.View().Content, "s3cret-value") {
		t.Fatal("the value stayed revealed after moving to another field")
	}
}

func TestCreateNewSecretThroughTheForm(t *testing.T) {
	m := unlocked(t)
	m = update(t, m, text("n"))
	if m.screen != screenForm {
		t.Fatalf("screen = %v, want screenForm", m.screen)
	}
	m = update(t, m, text("Gmail"))
	m = update(t, m, key("tab")) // description
	m = update(t, m, key("tab")) // tags
	m = update(t, m, key("tab")) // label of the first field
	m = update(t, m, text("Password"))
	m = update(t, m, key("right"))  // tx -> ps
	m = update(t, m, key("tab"))    // value
	m = update(t, m, key("ctrl+g")) // open the generator panel
	m = update(t, m, key("enter"))  // accept the candidate shown

	m = run(t, m, key("ctrl+s"))
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
	m = update(t, m, text("n"))
	m = run(t, m, key("ctrl+s"))
	if m.screen != screenForm {
		t.Fatal("a form without a title must not be saved")
	}
	if m.form.err == nil {
		t.Fatal("want an error message on the form")
	}
}

// Opening the panel on a Field Type the generator cannot fill must not open
// it — instead it reports why, on the same err path a bad save uses.
func TestGeneratorPanelRefusesANonGeneratableField(t *testing.T) {
	m := unlocked(t)
	m = update(t, m, text("n"))
	m = update(t, m, key("tab")) // description
	m = update(t, m, key("tab")) // tags
	m = update(t, m, key("tab")) // label of the first field ("tx" by default)
	m = update(t, m, key("tab")) // value
	m = update(t, m, key("ctrl+g"))
	if m.form.genOpen {
		t.Fatal("the panel opened on a field type the generator cannot fill")
	}
	if m.form.err == nil {
		t.Fatal("want an error explaining why nothing happened")
	}
}

// esc leaves the Field exactly as it was, even after knobs were changed.
func TestGeneratorPanelEscLeavesFieldUntouched(t *testing.T) {
	m := unlocked(t)
	m = update(t, m, text("n"))
	m = update(t, m, key("tab"))   // description
	m = update(t, m, key("tab"))   // tags
	m = update(t, m, key("tab"))   // label
	m = update(t, m, key("right")) // tx -> ps
	m = update(t, m, key("tab"))   // value

	m = update(t, m, key("ctrl+g"))
	if !m.form.genOpen {
		t.Fatal("the panel did not open on a generatable field")
	}
	m = update(t, m, key("left")) // turn the length knob down
	m = update(t, m, key("esc"))

	if m.form.genOpen {
		t.Fatal("esc did not close the panel")
	}
	if m.form.rows[0].value() != "" {
		t.Fatalf("esc filled the field: %q", m.form.rows[0].value())
	}
}

// enter accepts exactly the candidate on screen, and the length knob turns
// off the symbols and turns down the length, both honoured in the result.
func TestGeneratorPanelEnterAcceptsTheKnobsChosen(t *testing.T) {
	m := unlocked(t)
	m = update(t, m, text("n"))
	m = update(t, m, key("tab"))
	m = update(t, m, key("tab"))
	m = update(t, m, key("tab"))
	m = update(t, m, key("right")) // tx -> ps
	m = update(t, m, key("tab"))   // value

	m = update(t, m, key("ctrl+g"))
	m = update(t, m, key("down"))  // knob: upper
	m = update(t, m, key("down"))  // knob: digits
	m = update(t, m, key("down"))  // knob: symbols
	m = update(t, m, key("left"))  // symbols off
	m = update(t, m, key("enter")) // accept

	if m.form.genOpen {
		t.Fatal("enter did not close the panel")
	}
	got := m.form.rows[0].value()
	if len(got) != 20 {
		t.Fatalf("length = %d, want the default 20", len(got))
	}
	if strings.ContainsAny(got, generator.Symbols) {
		t.Fatalf("%q contains a symbol despite the knob being off", got)
	}
	if m.cfg.Generator.Symbols {
		t.Fatal("the session Policy was not updated with the knob change")
	}
}

// A Policy changed in the panel stays changed for the rest of the session,
// even without an explicit save: the next form to open starts from it.
func TestGeneratorPolicyStaysForTheSession(t *testing.T) {
	m := unlocked(t)
	m = update(t, m, text("n"))
	m = update(t, m, key("tab"))
	m = update(t, m, key("tab"))
	m = update(t, m, key("tab"))
	m = update(t, m, key("right")) // tx -> ps
	m = update(t, m, key("tab"))   // value
	m = update(t, m, key("ctrl+g"))
	m = update(t, m, key("down")) // upper
	m = update(t, m, key("down")) // digits
	m = update(t, m, key("down")) // symbols
	m = update(t, m, key("left"))
	m = update(t, m, key("esc")) // close without accepting the field

	// Leave the form and start a fresh one, as the "n" key does after any save.
	m.screen = screenList
	m = update(t, m, text("n"))
	if m.form.policy.Symbols {
		t.Fatal("the next form did not inherit the session's Policy change")
	}
}

func TestEditFormChangesAValueInPlace(t *testing.T) {
	m := unlocked(t)
	m = update(t, m, text("e"))
	if m.screen != screenForm {
		t.Fatalf("screen = %v, want screenForm", m.screen)
	}
	if m.form.editSlug != "facebook" {
		t.Fatalf("editSlug = %q, want facebook", m.form.editSlug)
	}
	if m.form.title.Value() != "Facebook" {
		t.Fatalf("title = %q, want the existing title", m.form.title.Value())
	}

	m = update(t, m, key("tab")) // description
	m = update(t, m, key("tab")) // tags
	m = update(t, m, key("tab")) // label of the first field
	m = update(t, m, key("tab")) // value of the first field
	// clear the existing username and type a new one
	for range "user@mail.com" {
		m = update(t, m, key("backspace"))
	}
	m = update(t, m, text("new@mail.com"))

	m = run(t, m, key("ctrl+s"))
	if m.screen != screenList {
		t.Fatalf("screen = %v after saving, want screenList (err: %v)", m.screen, m.form.err)
	}
	if len(m.list.Items()) != 1 {
		t.Fatalf("item count = %d, want 1 — the slug must not have changed", len(m.list.Items()))
	}
	s, err := m.store.Load("facebook")
	if err != nil {
		t.Fatalf("Load facebook: %v", err)
	}
	if s.Fields[0].Value != "new@mail.com" {
		t.Fatalf("the edit did not persist: %+v", s.Fields[0])
	}
}

func TestEditFormRenamesOnTitleChange(t *testing.T) {
	m := unlocked(t)
	m = update(t, m, text("e"))
	for range "Facebook" {
		m = update(t, m, key("backspace"))
	}
	m = update(t, m, text("Meta"))
	m = run(t, m, key("ctrl+s"))
	if m.screen != screenList {
		t.Fatalf("screen = %v after saving, want screenList (err: %v)", m.screen, m.form.err)
	}
	if _, err := m.store.Load("facebook"); err == nil {
		t.Fatal("the old slug is still there after a rename")
	}
	got, err := m.store.Load("meta")
	if err != nil {
		t.Fatalf("Load meta: %v", err)
	}
	if got.Fields[0].Value != "user@mail.com" {
		t.Fatalf("the contents did not survive the rename: %+v", got.Fields[0])
	}
}

func TestEditFormRefusesARenameThatCollides(t *testing.T) {
	m := unlocked(t)
	// A second secret to collide with.
	if _, err := m.store.Create(&secret.Secret{
		Meta:   secret.Meta{Title: "Gmail"},
		Fields: []secret.Field{{Type: "tx", Label: "Username", Value: "x"}},
	}); err != nil {
		t.Fatalf("Create Gmail: %v", err)
	}

	m = update(t, m, text("e")) // edit Facebook
	for range "Facebook" {
		m = update(t, m, key("backspace"))
	}
	m = update(t, m, text("Gmail"))
	m = run(t, m, key("ctrl+s"))
	if m.screen != screenForm {
		t.Fatal("a colliding rename must not be saved")
	}
	if m.form.err == nil || !strings.Contains(m.form.err.Error(), "already exists") {
		t.Fatalf("the error message does not explain the collision: %v", m.form.err)
	}
	if _, err := m.store.Load("facebook"); err != nil {
		t.Fatalf("the original secret was touched despite the refusal: %v", err)
	}
}

func TestEscOnAnUntouchedEditFormCancelsImmediately(t *testing.T) {
	m := unlocked(t)
	m = update(t, m, text("e"))
	m = update(t, m, key("esc"))
	if m.screen != screenList {
		t.Fatalf("screen = %v, want screenList — esc on an untouched form should not ask", m.screen)
	}
}

func TestEscOnADirtyEditFormAsksFirst(t *testing.T) {
	m := unlocked(t)
	m = update(t, m, text("e"))
	m = update(t, m, text("x")) // dirty the title
	m = update(t, m, key("esc"))
	if m.screen != screenForm {
		t.Fatal("the first esc on a dirty form must ask before discarding")
	}
	if !m.form.confirmDiscard {
		t.Fatal("want confirmDiscard set after the first esc")
	}

	// Any other key cancels the discard and returns to editing.
	m = update(t, m, text("y"))
	if m.form.confirmDiscard {
		t.Fatal("a non-esc key should cancel the pending discard")
	}
	if m.screen != screenForm {
		t.Fatal("a non-esc key should not leave the form")
	}

	// A second esc actually discards.
	m = update(t, m, key("esc"))
	if m.screen != screenForm {
		t.Fatal("esc must ask again on the still-dirty form")
	}
	m = update(t, m, key("esc"))
	if m.screen != screenList {
		t.Fatal("the second esc did not discard the form")
	}
	s, err := m.store.Load("facebook")
	if err != nil {
		t.Fatalf("Load facebook: %v", err)
	}
	if s.Meta.Title != "Facebook" {
		t.Fatalf("the discarded edit reached the vault: %+v", s.Meta)
	}
}

func TestDeleteRemovesTheSecretAfterConfirmation(t *testing.T) {
	m := unlocked(t)
	m = update(t, m, text("d"))
	if m.screen != screenConfirmDelete {
		t.Fatalf("screen = %v, want screenConfirmDelete", m.screen)
	}
	m = run(t, m, text("y"))
	if m.screen != screenList {
		t.Fatalf("screen = %v after confirming, want screenList", m.screen)
	}
	if len(m.list.Items()) != 0 {
		t.Fatalf("item count = %d, want 0", len(m.list.Items()))
	}
	if _, err := m.store.Load("facebook"); err == nil {
		t.Fatal("the secret is still in the vault after delete")
	}
}

func TestCancellingDeleteTouchesNothing(t *testing.T) {
	m := unlocked(t)
	m = update(t, m, text("d"))
	m = update(t, m, text("n"))
	if m.screen != screenList {
		t.Fatalf("screen = %v after cancelling, want screenList", m.screen)
	}
	if len(m.list.Items()) != 1 {
		t.Fatalf("item count = %d, want 1 — cancel must not delete", len(m.list.Items()))
	}
	if _, err := m.store.Load("facebook"); err != nil {
		t.Fatalf("Load facebook after cancel: %v", err)
	}
}

func TestCtrlRRevealsAPasswordValueInTheForm(t *testing.T) {
	m := unlocked(t)
	m = update(t, m, text("e"))  // edit Facebook
	m = update(t, m, key("tab")) // description
	m = update(t, m, key("tab")) // tags
	m = update(t, m, key("tab")) // label of the second field (Password)
	m = update(t, m, key("tab")) // value of the first field (Username)
	m = update(t, m, key("tab")) // label of the second field
	m = update(t, m, key("tab")) // value of the second field (Password)

	if strings.Contains(m.View().Content, "s3cret-value") {
		t.Fatal("the value is shown before ctrl+r is pressed")
	}
	m = update(t, m, key("ctrl+r"))
	if !strings.Contains(m.View().Content, "s3cret-value") {
		t.Fatalf("ctrl+r did not reveal the value:\n%s", m.View().Content)
	}
	m = update(t, m, key("ctrl+r"))
	if strings.Contains(m.View().Content, "s3cret-value") {
		t.Fatal("a second ctrl+r did not hide the value again")
	}
}

func TestCtrlRDoesNothingOnANonMaskedField(t *testing.T) {
	m := unlocked(t)
	m = update(t, m, text("e"))  // edit Facebook
	m = update(t, m, key("tab")) // description
	m = update(t, m, key("tab")) // tags
	m = update(t, m, key("tab")) // label of the first field (Username, "tx")
	m = update(t, m, key("tab")) // value of the first field

	m = update(t, m, key("ctrl+r"))
	if m.form.rows[0].reveal {
		t.Fatal("ctrl+r toggled reveal on a field type that is never masked")
	}
}

func TestDuplicateTitleIsRefusedWithAClearMessage(t *testing.T) {
	m := unlocked(t)
	m = update(t, m, text("n"))
	m = update(t, m, text("Facebook"))
	m = run(t, m, key("ctrl+s"))
	if m.screen != screenForm {
		t.Fatal("a duplicate title must not be saved")
	}
	if m.form.err == nil || !strings.Contains(m.form.err.Error(), "already exists") {
		t.Fatalf("the error message does not explain the collision: %v", m.form.err)
	}
}

// ? opens Help from the list; ? or esc closes it and lands back on the list.
func TestQuestionMarkOpensHelpFromTheList(t *testing.T) {
	m := unlocked(t)
	m = update(t, m, text("?"))
	if m.screen != screenHelp {
		t.Fatalf("screen = %v, want screenHelp", m.screen)
	}
	m = update(t, m, text("?"))
	if m.screen != screenList {
		t.Fatalf("second ? left screen = %v, want screenList", m.screen)
	}
	m = update(t, m, text("?"))
	m = update(t, m, key("esc"))
	if m.screen != screenList {
		t.Fatalf("esc left screen = %v, want screenList", m.screen)
	}
}

// Help is modal: a key that would act on the list underneath does nothing
// while Help is showing, so reading "d delete" cannot delete anything.
func TestHelpIgnoresOtherKeys(t *testing.T) {
	m := unlocked(t)
	m = update(t, m, text("?"))
	for _, k := range []tea.KeyPressMsg{text("d"), text("n"), text("q"), key("tab")} {
		m = update(t, m, k)
		if m.screen != screenHelp {
			t.Fatalf("%q left Help: screen = %v", k.String(), m.screen)
		}
	}
}

// ? inside a search must stay a search character.
func TestQuestionMarkWhileFilteringIsASearchCharacter(t *testing.T) {
	m := unlocked(t)
	m = update(t, m, text("/"))
	m = update(t, m, text("?"))
	if m.screen != screenList {
		t.Fatalf("screen = %v, want screenList", m.screen)
	}
	if got := m.list.FilterValue(); got != "?" {
		t.Fatalf("filter = %q, want %q", got, "?")
	}
}

// Help from the generator panel returns to the form with the panel still open.
func TestHelpFromTheGeneratorReturnsToThePanel(t *testing.T) {
	m := unlocked(t)
	m = update(t, m, text("n"))
	m = update(t, m, key("tab"))   // description
	m = update(t, m, key("tab"))   // tags
	m = update(t, m, key("tab"))   // label
	m = update(t, m, key("right")) // tx -> ps
	m = update(t, m, key("tab"))   // value
	m = update(t, m, key("ctrl+g"))
	if !m.form.genOpen {
		t.Fatal("the panel did not open")
	}

	m = update(t, m, text("?"))
	if m.screen != screenHelp {
		t.Fatalf("screen = %v, want screenHelp", m.screen)
	}
	m = update(t, m, key("esc"))
	if m.screen != screenForm || !m.form.genOpen {
		t.Fatalf("esc landed on screen %v with genOpen=%v; want the open panel", m.screen, m.form.genOpen)
	}
}

// ? in the form is a character the user is typing, never Help.
func TestQuestionMarkInTheFormIsTyped(t *testing.T) {
	m := unlocked(t)
	m = update(t, m, text("n"))
	m = update(t, m, text("?"))
	if m.screen != screenForm {
		t.Fatalf("screen = %v, want screenForm", m.screen)
	}
	if got := m.form.title.Value(); got != "?" {
		t.Fatalf("title = %q, want %q", got, "?")
	}
}
