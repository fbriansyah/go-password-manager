// Package vault menyimpan dan memuat Secret dari sebuah folder. Ia adalah
// satu-satunya lapisan yang menyentuh filesystem, dan satu-satunya yang
// menerjemahkan judul menjadi nama file.
package vault

import (
	"errors"
	"strings"
	"unicode"

	"github.com/fbriansyah/go-password-manager/internal/secret"
)

// Ext adalah ekstensi file Secret. Hanya file berekstensi ini yang dianggap
// bagian dari Vault.
const Ext = ".gopm"

var (
	// ErrNotFound dikembalikan saat slug tidak ada di Vault.
	ErrNotFound = errors.New("secret tidak ditemukan")
	// ErrSlugTaken dikembalikan saat nama file tujuan sudah dipakai Secret
	// lain. Vault tidak pernah menimpa Secret yang bukan sasaran penyimpanan.
	ErrSlugTaken = errors.New("sudah ada secret dengan judul itu")
)

// Cipher adalah lapisan enkripsi yang dipakai Vault. crypto.Session
// memenuhinya; pengujian memakai implementasi yang tidak mengenkripsi apa pun.
type Cipher interface {
	Encrypt(plaintext []byte) ([]byte, error)
	Decrypt(ciphertext []byte) ([]byte, error)
}

// Vault adalah kumpulan Secret yang bisa didaftar, dimuat, dan disimpan.
type Vault interface {
	// Dir menyebutkan lokasi Vault, untuk ditampilkan ke pengguna.
	Dir() string
	// List mengembalikan seluruh slug di Vault, terurut.
	List() ([]string, error)
	// Load mendekripsi satu Secret.
	Load(slug string) (*secret.Secret, error)
	// Create menyimpan Secret baru dan mengembalikan slug hasilnya.
	Create(s *secret.Secret) (string, error)
	// Save menyimpan perubahan pada slug yang ada. Jika judulnya berubah,
	// filenya ikut di-rename dan slug baru dikembalikan.
	Save(slug string, s *secret.Secret) (string, error)
	// Delete menghapus satu Secret.
	Delete(slug string) error
}

// Slug mengubah judul menjadi nama file yang aman. Ini satu-satunya bagian
// Secret yang terbaca tanpa Unlock — lihat docs/adr/0004.
func Slug(title string) string {
	var b strings.Builder
	lastDash := true // menahan dash di awal
	for _, r := range strings.ToLower(strings.TrimSpace(title)) {
		switch {
		case unicode.IsLetter(r) && r < unicode.MaxASCII, unicode.IsDigit(r) && r < unicode.MaxASCII:
			b.WriteRune(r)
			lastDash = false
		default:
			if !lastDash {
				b.WriteRune('-')
				lastDash = true
			}
		}
	}
	return strings.Trim(b.String(), "-")
}
