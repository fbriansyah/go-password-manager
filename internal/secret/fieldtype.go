package secret

import "strings"

// EditorKind menyatakan bentuk editor yang dibutuhkan sebuah Field Type,
// tanpa menyebut komponen TUI mana pun — pemetaannya milik lapisan TUI.
type EditorKind int

const (
	EditorLine   EditorKind = iota // input satu baris
	EditorMasked                   // input satu baris yang menyembunyikan ketikan
	EditorArea                     // input banyak baris
)

// Type adalah kontrak sebuah Field Type: bagaimana nilainya ditampilkan,
// bagaimana ia diedit, dan apa yang disalin darinya. Ketiganya dipisah karena
// nilai yang disalin tidak selalu sama dengan nilai yang tersimpan — TOTP
// menyimpan secret base32 tetapi menyalin enam digit.
type Type struct {
	ID     string
	Name   string
	Editor EditorKind

	// Render menghasilkan tampilan nilai untuk panel detail. reveal bernilai
	// true saat pengguna meminta nilai tersembunyi diperlihatkan.
	Render func(value string, reveal bool) string

	// Copy menghasilkan nilai yang masuk ke clipboard.
	Copy func(value string) (string, error)

	// Generatable menandai tipe yang boleh diisi oleh generator password.
	Generatable bool

	known bool // false untuk tipe asing yang ditemui saat memuat file
}

var registry = map[string]Type{}
var order []string

// Register menambahkan Field Type ke registry. Dipanggil saat init; tipe yang
// terdaftar belakangan menimpa yang bernama sama.
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

// TypeFor mengembalikan Field Type untuk id. Tipe yang tidak dikenal — file
// yang dibuat versi lebih baru — jatuh ke teks biasa, bukan error, dan
// nilainya tetap utuh saat disimpan kembali.
func TypeFor(id string) Type {
	if t, ok := registry[id]; ok {
		return t
	}
	return Type{
		ID:     id,
		Name:   id + " (tidak dikenal)",
		Editor: EditorLine,
		Render: plainRender,
		Copy:   rawCopy,
	}
}

// Known melaporkan apakah tipe ini terdaftar; tipe asing bernilai false.
func (t Type) Known() bool { return t.known }

// Types mengembalikan Field Type yang boleh dipilih pengguna, sesuai urutan
// pendaftaran.
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
		ID: "tx", Name: "Teks", Editor: EditorLine,
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
		ID: "ta", Name: "Catatan", Editor: EditorArea,
	})
}
