// Package crypto membungkus filippo.io/age. Seluruh kripto aplikasi ini ada di
// sini; tidak ada primitif yang dirakit sendiri.
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

// ErrWrongPassword dikembalikan saat Master Password tidak membuka Identity.
var ErrWrongPassword = errors.New("master password salah")

// Session adalah hasil Unlock: Identity yang sudah terbuka di memori, hidup
// selama proses berjalan dan tidak pernah disimpan ke disk.
type Session struct {
	identity  *age.X25519Identity
	recipient *age.X25519Recipient
}

// GenerateKeypair membuat Identity baru, menulisnya ke identityPath dalam
// bentuk terenkripsi Master Password, dan menulis Recipient ke recipientPath
// sebagai teks biasa. Kedua file ditolak jika sudah ada — menimpa Identity
// berarti kehilangan seluruh Secret yang pernah dienkripsi untuknya.
func GenerateKeypair(identityPath, recipientPath, masterPassword string) error {
	if masterPassword == "" {
		return errors.New("master password tidak boleh kosong")
	}
	for _, p := range []string{identityPath, recipientPath} {
		if _, err := os.Stat(p); err == nil {
			return fmt.Errorf("%s sudah ada; hapus sendiri jika memang ingin membuat key baru", p)
		}
	}
	id, err := age.GenerateX25519Identity()
	if err != nil {
		return fmt.Errorf("gagal membuat keypair: %w", err)
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

// Unlock membuka Identity di identityPath dengan Master Password. Recipient
// diturunkan dari Identity, sehingga sesi yang terbuka selalu bisa membaca apa
// yang ia tulis.
func Unlock(identityPath, masterPassword string) (*Session, error) {
	sealed, err := os.ReadFile(identityPath)
	if err != nil {
		return nil, fmt.Errorf("identity tidak terbaca: %w", err)
	}
	plain, err := decryptWithPassword(sealed, masterPassword)
	if err != nil {
		return nil, err
	}
	id, err := age.ParseX25519Identity(strings.TrimSpace(string(plain)))
	if err != nil {
		return nil, fmt.Errorf("isi identity tidak sah: %w", err)
	}
	return &Session{identity: id, recipient: id.Recipient()}, nil
}

// RecipientOnly membuat sesi yang hanya bisa menulis: cukup public key, tanpa
// Master Password. Membaca Secret dengan sesi ini akan gagal.
func RecipientOnly(recipientPath string) (*Session, error) {
	data, err := os.ReadFile(recipientPath)
	if err != nil {
		return nil, fmt.Errorf("recipient tidak terbaca: %w", err)
	}
	r, err := age.ParseX25519Recipient(strings.TrimSpace(string(data)))
	if err != nil {
		return nil, fmt.Errorf("isi recipient tidak sah: %w", err)
	}
	return &Session{recipient: r}, nil
}

// Encrypt membungkus plaintext untuk Recipient sesi ini dalam bentuk armored
// ASCII, sehingga file Secret aman melewati git dan copy-paste.
func (s *Session) Encrypt(plaintext []byte) ([]byte, error) {
	var buf bytes.Buffer
	a := armor.NewWriter(&buf)
	w, err := age.Encrypt(a, s.recipient)
	if err != nil {
		return nil, fmt.Errorf("gagal mengenkripsi: %w", err)
	}
	if _, err := w.Write(plaintext); err != nil {
		return nil, fmt.Errorf("gagal mengenkripsi: %w", err)
	}
	if err := w.Close(); err != nil {
		return nil, fmt.Errorf("gagal mengenkripsi: %w", err)
	}
	if err := a.Close(); err != nil {
		return nil, fmt.Errorf("gagal mengenkripsi: %w", err)
	}
	return buf.Bytes(), nil
}

// Decrypt membuka ciphertext armored dengan Identity sesi ini.
func (s *Session) Decrypt(ciphertext []byte) ([]byte, error) {
	if s.identity == nil {
		return nil, errors.New("sesi ini hanya bisa menulis; tidak ada identity untuk membaca")
	}
	r, err := age.Decrypt(armor.NewReader(bytes.NewReader(ciphertext)), s.identity)
	if err != nil {
		return nil, fmt.Errorf("gagal mendekripsi: %w", err)
	}
	plain, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("gagal mendekripsi: %w", err)
	}
	return plain, nil
}

// CanRead melaporkan apakah sesi ini membawa Identity.
func (s *Session) CanRead() bool { return s.identity != nil }

func encryptWithPassword(plaintext []byte, password string) ([]byte, error) {
	r, err := age.NewScryptRecipient(password)
	if err != nil {
		return nil, fmt.Errorf("master password ditolak: %w", err)
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
		return nil, fmt.Errorf("master password ditolak: %w", err)
	}
	r, err := age.Decrypt(armor.NewReader(bytes.NewReader(ciphertext)), id)
	if err != nil {
		// age tidak membedakan password salah dari file rusak; pada file yang
		// kita tulis sendiri, penyebab yang jauh lebih mungkin adalah password.
		return nil, ErrWrongPassword
	}
	return io.ReadAll(r)
}

func writeNew(path string, data []byte, perm os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("gagal membuat folder untuk %s: %w", path, err)
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, perm)
	if err != nil {
		return fmt.Errorf("gagal menulis %s: %w", path, err)
	}
	defer f.Close()
	if _, err := f.Write(data); err != nil {
		return fmt.Errorf("gagal menulis %s: %w", path, err)
	}
	return f.Sync()
}
