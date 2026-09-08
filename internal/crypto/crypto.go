// Package crypto wraps filippo.io/age. All of this application's cryptography
// lives here; no primitive is assembled by hand.
package crypto

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"filippo.io/age"
	"filippo.io/age/armor"
)

// ErrWrongPassword is returned when the Master Password does not open the Identity.
var ErrWrongPassword = errors.New("wrong master password")

// Session is the result of Unlock: an Identity held open in memory, alive for
// as long as the process runs and never written back to disk.
type Session struct {
	identity  *age.X25519Identity
	recipient *age.X25519Recipient
}

// GenerateKeypair creates a new Identity, writes it to identityPath encrypted
// with the Master Password, and writes the Recipient to recipientPath as plain
// text. Either file is refused if it already exists — overwriting an Identity
// means losing every Secret ever encrypted for it.
func GenerateKeypair(identityPath, recipientPath, masterPassword string) error {
	if masterPassword == "" {
		return errors.New("the master password cannot be empty")
	}
	for _, p := range []string{identityPath, recipientPath} {
		if _, err := os.Stat(p); err == nil {
			return fmt.Errorf("%s already exists; remove it yourself if you really want a new key", p)
		}
	}
	id, err := age.GenerateX25519Identity()
	if err != nil {
		return fmt.Errorf("could not create the keypair: %w", err)
	}
	sealed, err := encryptWithPassword([]byte(id.String()+"\n"), masterPassword)
	if err != nil {
		return err
	}
	if err := writeNew(identityPath, sealed, 0o600); err != nil {
		return err
	}
	return writeNew(recipientPath, []byte(id.Recipient().String()+"\n"), 0o644)
}

// Unlock opens the Identity at identityPath with the Master Password. The
// Recipient is derived from the Identity, so an unlocked session can always
// read back what it writes.
func Unlock(identityPath, masterPassword string) (*Session, error) {
	sealed, err := os.ReadFile(identityPath)
	if err != nil {
		return nil, fmt.Errorf("identity is unreadable: %w", err)
	}
	plain, err := decryptWithPassword(sealed, masterPassword)
	if err != nil {
		return nil, err
	}
	id, err := age.ParseX25519Identity(strings.TrimSpace(string(plain)))
	if err != nil {
		return nil, fmt.Errorf("identity contents are invalid: %w", err)
	}
	return &Session{identity: id, recipient: id.Recipient()}, nil
}

// RecipientOnly builds a write-only session: the public key is enough, no
// Master Password needed. Reading a Secret with this session fails.
func RecipientOnly(recipientPath string) (*Session, error) {
	data, err := os.ReadFile(recipientPath)
	if err != nil {
		return nil, fmt.Errorf("recipient is unreadable: %w", err)
	}
	r, err := age.ParseX25519Recipient(strings.TrimSpace(string(data)))
	if err != nil {
		return nil, fmt.Errorf("recipient contents are invalid: %w", err)
	}
	return &Session{recipient: r}, nil
}

// Encrypt wraps plaintext for this session's Recipient as armored ASCII, so a
// Secret file survives git and copy-paste unharmed.
func (s *Session) Encrypt(plaintext []byte) ([]byte, error) {
	var buf bytes.Buffer
	a := armor.NewWriter(&buf)
	w, err := age.Encrypt(a, s.recipient)
	if err != nil {
		return nil, fmt.Errorf("could not encrypt: %w", err)
	}
	if _, err := w.Write(plaintext); err != nil {
		return nil, fmt.Errorf("could not encrypt: %w", err)
	}
	if err := w.Close(); err != nil {
		return nil, fmt.Errorf("could not encrypt: %w", err)
	}
	if err := a.Close(); err != nil {
		return nil, fmt.Errorf("could not encrypt: %w", err)
	}
	return buf.Bytes(), nil
}

// Decrypt opens armored ciphertext with this session's Identity.
func (s *Session) Decrypt(ciphertext []byte) ([]byte, error) {
	if s.identity == nil {
		return nil, errors.New("this session can only write; there is no identity to read with")
	}
	r, err := age.Decrypt(armor.NewReader(bytes.NewReader(ciphertext)), s.identity)
	if err != nil {
		return nil, fmt.Errorf("could not decrypt: %w", err)
	}
	plain, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("could not decrypt: %w", err)
	}
	return plain, nil
}

// CanRead reports whether this session carries an Identity.
func (s *Session) CanRead() bool { return s.identity != nil }

func encryptWithPassword(plaintext []byte, password string) ([]byte, error) {
	r, err := age.NewScryptRecipient(password)
	if err != nil {
		return nil, fmt.Errorf("master password refused: %w", err)
	}
	var buf bytes.Buffer
	a := armor.NewWriter(&buf)
	w, err := age.Encrypt(a, r)
	if err != nil {
		return nil, err
	}
	if _, err := w.Write(plaintext); err != nil {
		return nil, err
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	if err := a.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func decryptWithPassword(ciphertext []byte, password string) ([]byte, error) {
	id, err := age.NewScryptIdentity(password)
	if err != nil {
		return nil, fmt.Errorf("master password refused: %w", err)
	}
	r, err := age.Decrypt(armor.NewReader(bytes.NewReader(ciphertext)), id)
	if err != nil {
		// age does not tell a wrong password from a corrupt file; on a file we
		// wrote ourselves, the far likelier cause is the password.
		return nil, ErrWrongPassword
	}
	return io.ReadAll(r)
}

func writeNew(path string, data []byte, perm os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("could not create the folder for %s: %w", path, err)
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, perm)
	if err != nil {
		return fmt.Errorf("could not write %s: %w", path, err)
	}
	defer f.Close()
	if _, err := f.Write(data); err != nil {
		return fmt.Errorf("could not write %s: %w", path, err)
	}
	return f.Sync()
}
