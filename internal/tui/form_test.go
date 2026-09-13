package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/fbriansyah/go-password-manager/internal/generator"
	"github.com/fbriansyah/go-password-manager/internal/secret"
)

// press feeds keys to the form one after another, keeping only the last
// outcome — the way a user types a sequence and looks at the screen once.
func press(f formModel, msgs ...tea.KeyPressMsg) (formModel, formOutcome) {
	var out formOutcome
	for _, msg := range msgs {
		f, out, _ = f.handle(msg)
	}
	return f, out
}

func TestSaveOnATitledFormSubmitsTheSecret(t *testing.T) {
	f := newForm(generator.Default())
	f, out := press(f, text("Gmail"), key("ctrl+s"))
	if out.kind != formSubmit {
		t.Fatalf("outcome = %v, want formSubmit (err: %v)", out.kind, f.err)
	}
	if out.secret == nil || out.secret.Meta.Title != "Gmail" {
		t.Fatalf("submitted secret = %+v, want title Gmail", out.secret)
	}
}

func TestSaveRefusesASecretWithoutATitle(t *testing.T) {
	f := newForm(generator.Default())
	f, out := press(f, key("ctrl+s"))
	if out.kind != formNone {
		t.Fatalf("outcome = %v, want formNone", out.kind)
	}
	if f.err == nil {
		t.Fatal("want an error message on the form")
	}
}

func TestEscOnAnUntouchedFormCancels(t *testing.T) {
	f := newForm(generator.Default())
	_, out := press(f, key("esc"))
	if out.kind != formCancelled {
		t.Fatalf("outcome = %v, want formCancelled", out.kind)
	}
}

func TestEscOnADirtyFormAsksFirst(t *testing.T) {
	f := newForm(generator.Default())
	f, out := press(f, text("x"), key("esc"))
	if out.kind != formNone || !f.confirmDiscard {
		t.Fatalf("first esc: outcome %v, confirmDiscard %v; want to be asked", out.kind, f.confirmDiscard)
	}

	// Any other key cancels the discard and keeps editing — without typing.
	f, out = press(f, text("y"))
	if out.kind != formNone || f.confirmDiscard {
		t.Fatal("a non-esc key should cancel the pending discard")
	}
	if got := f.title.Value(); got != "x" {
		t.Fatalf("the cancelling key was typed into the title: %q", got)
	}

	f, out = press(f, key("esc"), key("esc"))
	if out.kind != formCancelled {
		t.Fatalf("second esc: outcome %v, want formCancelled", out.kind)
	}
}

func TestTabAndArrowsPickTheFieldAndItsType(t *testing.T) {
	f := newForm(generator.Default())
	f, _ = press(f, text("Gmail"),
		key("tab"), key("tab"), key("tab"), // description, tags, label
		text("Password"),
		key("right"), // tx -> ps
		key("tab"),   // value
		text("hunter2"))
	s := f.secretValue()
	if len(s.Fields) != 1 || s.Fields[0].Type != "ps" || s.Fields[0].Label != "Password" || s.Fields[0].Value != "hunter2" {
		t.Fatalf("fields = %+v", s.Fields)
	}

	// Arrows on a value cell never change the type; shift+tab walks back.
	f, _ = press(f, key("right"), key("shift+tab"), key("left"))
	if got := f.secretValue().Fields[0].Type; got != "tx" {
		t.Fatalf("type after right-on-value, shift+tab, left = %q, want tx", got)
	}
}

// A Secret written by a newer version may carry a Field Type this version
// does not know. Editing it must not rewrite the type (secret.TypeFor's
// promise), unless the user changes it on purpose.
func TestAForeignFieldTypeSurvivesAnEdit(t *testing.T) {
	s := &secret.Secret{
		Meta:   secret.Meta{Title: "Bank"},
		Fields: []secret.Field{{Type: "totp", Label: "Code", Value: "JBSWY3DP"}},
	}
	f := editForm(generator.Default(), "bank", s)
	f, _ = press(f, key("tab"), key("tab"), key("tab"), key("tab"), text("X")) // touch the value
	got := f.secretValue().Fields[0]
	if got.Type != "totp" || got.Value != "JBSWY3DPX" {
		t.Fatalf("field after edit = %+v, want type totp kept", got)
	}

	// Cycling the type is the one deliberate way off a foreign type.
	f, _ = press(f, key("shift+tab"), key("right"))
	if got := f.secretValue().Fields[0].Type; !secret.TypeFor(got).Known() {
		t.Fatalf("type after cycling = %q, want a registered type", got)
	}
}

func TestAddAndRemoveFieldRows(t *testing.T) {
	f := newForm(generator.Default())
	f, _ = press(f, key("ctrl+n"), key("ctrl+n"))
	if len(f.rows) != 3 {
		t.Fatalf("rows after two ctrl+n = %d, want 3", len(f.rows))
	}
	f, _ = press(f, key("ctrl+d"), key("ctrl+d"), key("ctrl+d"))
	if len(f.rows) != 1 {
		t.Fatalf("rows after three ctrl+d = %d, want 1 — the last row can never be removed", len(f.rows))
	}
}

func TestCtrlRRevealsOnlyAMaskedValue(t *testing.T) {
	s := &secret.Secret{Meta: secret.Meta{Title: "Facebook"}, Fields: []secret.Field{
		{Type: "tx", Label: "Username", Value: "user@mail.com"},
		{Type: "ps", Label: "Password", Value: "s3cret-value"},
	}}
	f := editForm(generator.Default(), "facebook", s)
	f, _ = press(f, key("tab"), key("tab"), key("tab"), key("tab")) // Username value
	f, _ = press(f, key("ctrl+r"))
	if f.rows[0].reveal {
		t.Fatal("ctrl+r toggled reveal on a field type that is never masked")
	}
	f, _ = press(f, key("tab"), key("tab")) // Password value
	if strings.Contains(f.View(80), "s3cret-value") {
		t.Fatal("the value is shown before ctrl+r is pressed")
	}
	f, _ = press(f, key("ctrl+r"))
	if !strings.Contains(f.View(80), "s3cret-value") {
		t.Fatalf("ctrl+r did not reveal the value:\n%s", f.View(80))
	}
	f, _ = press(f, key("ctrl+r"))
	if strings.Contains(f.View(80), "s3cret-value") {
		t.Fatal("a second ctrl+r did not hide the value again")
	}
}

func TestGeneratorPanelRefusesANonGeneratableType(t *testing.T) {
	f := newForm(generator.Default())
	f, _ = press(f, key("tab"), key("tab"), key("tab"), key("tab"), key("ctrl+g")) // value of a "tx" field
	if f.genOpen {
		t.Fatal("the panel opened on a field type the generator cannot fill")
	}
	if f.err == nil {
		t.Fatal("want an error explaining why nothing happened")
	}
}

// onPasswordValue is a fresh form with the focus on the value of a "ps" field.
func onPasswordValue(t *testing.T) formModel {
	t.Helper()
	f := newForm(generator.Default())
	f, _ = press(f, key("tab"), key("tab"), key("tab"), key("right"), key("tab"))
	if got := f.secretValue(); len(f.rows) != 1 || f.rows[0].typeID != "ps" {
		t.Fatalf("setup: rows %+v, secret %+v", f.rows, got)
	}
	return f
}

func TestGeneratorPanelEnterAcceptsTheKnobsChosen(t *testing.T) {
	f := onPasswordValue(t)
	f, _ = press(f, key("ctrl+g"))
	if !f.genOpen {
		t.Fatal("the panel did not open on a generatable field")
	}
	f, out := press(f, key("down"), key("down"), key("down"), key("left"), key("enter")) // symbols off, accept
	if out.kind != formNone || f.genOpen {
		t.Fatalf("enter: outcome %v, genOpen %v; want the panel closed quietly", out.kind, f.genOpen)
	}
	got := f.rows[0].value()
	if len(got) != 20 || strings.ContainsAny(got, generator.Symbols) {
		t.Fatalf("value %q: want 20 chars and no symbols", got)
	}
	if f.policy.Symbols {
		t.Fatal("the Policy was not updated with the knob change")
	}
}

func TestGeneratorPanelEscLeavesTheFieldUntouched(t *testing.T) {
	f := onPasswordValue(t)
	f, _ = press(f, key("ctrl+g"), key("left"), key("esc"))
	if f.genOpen || f.rows[0].value() != "" {
		t.Fatalf("esc: genOpen %v, value %q", f.genOpen, f.rows[0].value())
	}
	if f.policy.Length != generator.Default().Length-1 {
		t.Fatalf("length after one step down = %d", f.policy.Length)
	}
}

func TestGeneratorPanelTypesALengthAndRerolls(t *testing.T) {
	f := onPasswordValue(t)
	f, _ = press(f, key("ctrl+g"), text("1"), text("2"))
	if f.policy.Length != 12 || len(f.candidate) != 12 {
		t.Fatalf("length %d, candidate %q; want 12", f.policy.Length, f.candidate)
	}
	before := f.candidate
	f, _ = press(f, text("r"))
	if f.candidate == before {
		t.Fatal("r did not reroll the candidate")
	}
	if f.rows[0].value() != "" {
		t.Fatalf("panel keys reached the field: %q", f.rows[0].value())
	}
}

func TestGeneratorPanelSaveAndHelpAreReportedToTheCaller(t *testing.T) {
	f := onPasswordValue(t)
	f, out := press(f, key("ctrl+g"), key("ctrl+s"))
	if out.kind != formSavePolicy || !f.genOpen {
		t.Fatalf("ctrl+s: outcome %v, genOpen %v; want formSavePolicy with the panel still open", out.kind, f.genOpen)
	}
	_, out = press(f, text("?"))
	if out.kind != formOpenHelp {
		t.Fatalf("?: outcome %v, want formOpenHelp", out.kind)
	}
}

// ? in the form proper is a character the user is typing, never Help.
func TestQuestionMarkInTheFormIsTyped(t *testing.T) {
	f := newForm(generator.Default())
	f, out := press(f, text("?"))
	if out.kind != formNone || f.title.Value() != "?" {
		t.Fatalf("outcome %v, title %q", out.kind, f.title.Value())
	}
}

// The footer and Help are both drawn from the Bindings, so every key the
// form answers to is named in both, written with the modifier of this OS.
func TestFooterAndHelpAreDrawnFromTheBindings(t *testing.T) {
	withKeyOS(t, "darwin")
	f := newForm(generator.Default())
	footer := f.View(120)
	for _, want := range []string{"tab move", "←/→ change type", "⌘/^n add field", "⌘/^s save", "esc cancel"} {
		if !strings.Contains(footer, want) {
			t.Errorf("form footer lacks %q:\n%s", want, footer)
		}
	}
	help := helpView(f.help(), 120)
	for _, want := range []string{"⌘/^s", "save the secret", "shift+tab", "⌘/^r", "reveal or hide"} {
		if !strings.Contains(help, want) {
			t.Errorf("form Help lacks %q:\n%s", want, help)
		}
	}

	f, _ = press(f, key("tab"), key("tab"), key("tab"), key("right"), key("tab"), key("ctrl+g"))
	footer = f.View(120)
	for _, want := range []string{"↑/↓ knob", "0-9 length", "r reroll", "⌘/^s save default", "? help"} {
		if !strings.Contains(footer, want) {
			t.Errorf("panel footer lacks %q:\n%s", want, footer)
		}
	}
	if help := helpView(f.help(), 120); !strings.Contains(help, "accept the candidate") {
		t.Errorf("panel Help is not the panel's:\n%s", help)
	}
}
