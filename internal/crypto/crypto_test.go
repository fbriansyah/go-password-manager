package crypto_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fbriansyah/go-password-manager/internal/crypto"
)

func keys(t *testing.T, password string) (identity, recipient string) {
	t.Helper()
	dir := t.TempDir()
	identity = filepath.Join(dir, "identity.age")
	recipient = filepath.Join(dir, "recipient.pub")
	if err := crypto.GenerateKeypair(identity, recipient, password); err != nil {
		t.Fatalf("GenerateKeypair: %v", err)
	}
	return identity, recipient
}

func TestUnlockLaluRoundTrip(t *testing.T) {
	id, _ := keys(t, "master-yang-panjang")
	s, err := crypto.Unlock(id, "master-yang-panjang")
	if err != nil {
		t.Fatalf("Unlock: %v", err)
	}
	ct, err := s.Encrypt([]byte(`{"meta":{"title":"Facebook"}}`))
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	plain, err := s.Decrypt(ct)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}
	if string(plain) != `{"meta":{"title":"Facebook"}}` {
		t.Fatalf("plaintext berubah: %s", plain)
	}
}

func TestCiphertextArmoredDanTidakMembocorkanIsi(t *testing.T) {
	id, _ := keys(t, "master")
	s, _ := crypto.Unlock(id, "master")
	ct, err := s.Encrypt([]byte("Facebook rahasia123"))
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	if !strings.HasPrefix(string(ct), "-----BEGIN AGE ENCRYPTED FILE-----") {
		t.Fatalf("bukan armored ASCII: %.40s", ct)
	}
	if strings.Contains(string(ct), "rahasia123") {
		t.Fatal("plaintext bocor ke ciphertext")
	}
}

func TestPasswordSalahDitolak(t *testing.T) {
	id, _ := keys(t, "benar")
	if _, err := crypto.Unlock(id, "salah"); !errors.Is(err, crypto.ErrWrongPassword) {
		t.Fatalf("err = %v, mau ErrWrongPassword", err)
	}
}

func TestIdentityTersimpanTerenkripsi(t *testing.T) {
	id, _ := keys(t, "master")
	data, err := os.ReadFile(id)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if strings.Contains(string(data), "AGE-SECRET-KEY-") {
		t.Fatal("private key tersimpan tanpa enkripsi")
	}
	info, err := os.Stat(id)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("izin identity = %v, mau 0600", info.Mode().Perm())
	}
}

// Recipient saja cukup untuk menulis, tetapi tidak untuk membaca — inilah satu
// alasan desain ini asimetris (docs/adr/0003).
func TestRecipientOnlyBisaMenulisTapiTidakMembaca(t *testing.T) {
	idPath, recPath := keys(t, "master")
	writer, err := crypto.RecipientOnly(recPath)
	if err != nil {
		t.Fatalf("RecipientOnly: %v", err)
	}
	if writer.CanRead() {
		t.Fatal("sesi recipient-only mengaku bisa membaca")
	}
	ct, err := writer.Encrypt([]byte("dari mesin tanpa identity"))
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	if _, err := writer.Decrypt(ct); err == nil {
		t.Fatal("sesi recipient-only berhasil mendekripsi")
	}
	reader, err := crypto.Unlock(idPath, "master")
	if err != nil {
		t.Fatalf("Unlock: %v", err)
	}
	plain, err := reader.Decrypt(ct)
	if err != nil {
		t.Fatalf("Decrypt oleh pemilik identity: %v", err)
	}
	if string(plain) != "dari mesin tanpa identity" {
		t.Fatalf("plaintext berubah: %s", plain)
	}
}

func TestGenerateKeypairMenolakMenimpaIdentity(t *testing.T) {
	id, rec := keys(t, "master")
	err := crypto.GenerateKeypair(id, rec, "master")
	if err == nil {
		t.Fatal("mau error, key lama tidak boleh tertimpa")
	}
	if !strings.Contains(err.Error(), "sudah ada") {
		t.Fatalf("pesan error tidak menjelaskan: %v", err)
	}
}
