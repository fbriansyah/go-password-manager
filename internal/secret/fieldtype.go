package secret

import "strings"

// EditorKind states the kind of editor a Field Type needs, without naming any
// TUI component — that mapping belongs to the TUI layer.
type EditorKind int

const (
	EditorLine   EditorKind = iota // single-line input
	EditorMasked                   // single-line input that hides what is typed
	EditorArea                     // multi-line input
)

// Type is the contract of a Field Type: how its value is rendered, how it is
// edited, and what gets copied from it. The three are kept apart because the
// copied value is not always the stored value — a TOTP field stores a base32
// secret but copies six digits.
type Type struct {
	ID     string
	Name   string
	Editor EditorKind

	// Render produces the display form of the value for the detail pane.
	// reveal is true when the user asks for a hidden value to be shown.
	Render func(value string, reveal bool) string

	// Copy produces the value that goes to the clipboard.
	Copy func(value string) (string, error)

	// Generatable marks types the password generator is allowed to fill.
	Generatable bool

	known bool // false for foreign types met while loading a file
}

var registry = map[string]Type{}
var order []string

// Register adds a Field Type to the registry. Called from init; a type
// registered later replaces an earlier one with the same id.
func Register(t Type) {
	t.known = true
	if t.Render == nil {
		t.Render = plainRender
	}
	if t.Copy == nil {
		t.Copy = rawCopy
	}
	if _, exists := registry[t.ID]; !exists {
		order = append(order, t.ID)
	}
	registry[t.ID] = t
}

// TypeFor returns the Field Type for id. Unknown types — a file written by a
// newer version — fall back to plain text rather than an error, and their
// values survive intact when the Secret is saved again.
func TypeFor(id string) Type {
	if t, ok := registry[id]; ok {
		return t
	}
	return Type{
		ID:     id,
		Name:   id + " (unknown)",
		Editor: EditorLine,
		Render: plainRender,
		Copy:   rawCopy,
	}
}

// Known reports whether this type is registered; foreign types are false.
func (t Type) Known() bool { return t.known }

// Types returns the Field Types a user may pick from, in registration order.
func Types() []Type {
	out := make([]Type, 0, len(order))
	for _, id := range order {
		out = append(out, registry[id])
	}
	return out
}

func plainRender(value string, _ bool) string { return value }
func rawCopy(value string) (string, error)    { return value, nil }

func init() {
	Register(Type{
		ID: "tx", Name: "Text", Editor: EditorLine,
	})
	Register(Type{
		ID: "ps", Name: "Password", Editor: EditorMasked, Generatable: true,
		Render: func(value string, reveal bool) string {
			if reveal {
				return value
			}
			if value == "" {
				return ""
			}
			return strings.Repeat("•", 12)
		},
	})
	Register(Type{
		ID: "ta", Name: "Note", Editor: EditorArea,
	})
}
