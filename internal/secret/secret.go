// Package secret mendefinisikan bentuk Secret dan cara ia diserialisasi.
// Paket ini tidak tahu apa-apa tentang enkripsi, filesystem, maupun TUI.
package secret

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// Meta mendeskripsikan sebuah Secret: dipakai untuk menemukannya, bukan
// sebagai kredensial.
type Meta struct {
	Title       string   `json:"title"`
	Description string   `json:"description,omitempty"`
	Tags        []string `json:"tags,omitempty"`
}

// Field adalah satu pasangan label dan nilai di dalam Secret. Type menunjuk ke
// Field Type yang menentukan cara nilainya ditampilkan, diedit, dan disalin.
type Field struct {
	Type  string `json:"type"`
	Label string `json:"label"`
	Value string `json:"value"`
}

// Secret adalah satu kumpulan kredensial: Meta ditambah sederet Field.
type Secret struct {
	Meta   Meta    `json:"meta"`
	Fields []Field `json:"fields"`
}

// ErrEmptyTitle dikembalikan saat Secret tidak punya judul; judul dibutuhkan
// karena Slug diturunkan darinya.
var ErrEmptyTitle = errors.New("judul secret tidak boleh kosong")

// Validate memeriksa Secret cukup lengkap untuk disimpan.
func (s *Secret) Validate() error {
	if strings.TrimSpace(s.Meta.Title) == "" {
		return ErrEmptyTitle
	}
	for i, f := range s.Fields {
		if strings.TrimSpace(f.Label) == "" {
			return fmt.Errorf("field ke-%d tidak punya label", i+1)
		}
		if f.Type == "" {
			return fmt.Errorf("field %q tidak punya tipe", f.Label)
		}
	}
	return nil
}

// Marshal menghasilkan bentuk JSON yang akan dienkripsi.
func Marshal(s *Secret) ([]byte, error) {
	if s.Fields == nil {
		s.Fields = []Field{}
	}
	return json.MarshalIndent(s, "", "  ")
}

// Unmarshal membaca kembali bentuk JSON hasil Marshal. Field bertipe asing
// tetap dimuat apa adanya; lihat TypeFor.
func Unmarshal(data []byte) (*Secret, error) {
	var s Secret
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("secret tidak dapat dibaca: %w", err)
	}
	if s.Fields == nil {
		s.Fields = []Field{}
	}
	return &s, nil
}
