// Package vault stores and loads Secrets from a folder. It is the only layer
// that touches the filesystem, and the only one that turns a title into a
// file name.
package vault

import (
	"errors"
	"strings"
	"unicode"

	"github.com/fbriansyah/go-password-manager/internal/secret"
)

// Ext is the Secret file extension. Only files carrying it count as part of
// the Vault.
const Ext = ".gopm"

var (
	// ErrNotFound is returned when a slug is not in the Vault.
	ErrNotFound = errors.New("secret not found")
	// ErrSlugTaken is returned when the target file name already belongs to
	// another Secret. A Vault never overwrites a Secret it was not asked to save.
	ErrSlugTaken = errors.New("a secret with that title already exists")
)

// Cipher is the encryption layer a Vault uses. crypto.Session satisfies it;
// the tests use an implementation that encrypts nothing.
type Cipher interface {
	Encrypt(plaintext []byte) ([]byte, error)
	Decrypt(ciphertext []byte) ([]byte, error)
}

// Vault is a collection of Secrets that can be listed, loaded, and stored.
type Vault interface {
	// Dir names where the Vault lives, for showing to the user.
	Dir() string
	// List returns every slug in the Vault, sorted.
	List() ([]string, error)
	// Load decrypts one Secret.
	Load(slug string) (*secret.Secret, error)
	// Create stores a new Secret and returns the slug it got.
	Create(s *secret.Secret) (string, error)
	// Save stores changes to an existing slug. If the title changed, the file is
	// renamed too and the new slug is returned.
	Save(slug string, s *secret.Secret) (string, error)
	// Delete removes one Secret.
	Delete(slug string) error
}

// Slug turns a title into a safe file name. This is the only part of a Secret
// readable without unlocking — see docs/adr/0004.
func Slug(title string) string {
	var b strings.Builder
	lastDash := true // suppress a leading dash
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
