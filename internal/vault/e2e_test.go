package vault_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fbriansyah/go-password-manager/internal/crypto"
	"github.com/fbriansyah/go-password-manager/internal/secret"
	"github.com/fbriansyah/go-password-manager/internal/vault"
)

// Alur utuh Milestone 1: buat keypair, unlock, simpan Secret, baca kembali
// lewat sesi baru — persis seperti menutup lalu membuka lagi aplikasinya.
func TestAlurBuatUnlockBacaKembali(t *testing.T) {
	keyDir, vaultDir := t.TempDir(), t.TempDir()
	idPath := filepath.Join(keyDir, "identity.age")
	recPath := filepath.Join(keyDir, "recipient.pub")
	if err := crypto.GenerateKeypair(idPath, recPath, "master-password"); err != nil {
		t.Fatalf("GenerateKeypair: %v", err)
	}

	menulis, err := crypto.Unlock(idPath, "master-password")
	if err != nil {
		t.Fatalf("Unlock: %v", err)
	}
	v, err := vault.Open(vaultDir, menulis)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	slug, err := v.Create(facebook())
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// File di disk harus armored dan tidak membocorkan apa pun selain slug.
	raw, err := os.ReadFile(filepath.Join(vaultDir, slug+vault.Ext))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !strings.HasPrefix(string(raw), "-----BEGIN AGE ENCRYPTED FILE-----") {
		t.Fatalf("file bukan armored: %.40s", raw)
	}
	for _, bocor := range []string{"rahasia123", "budi@mail.com", "Facebook Credential"} {
		if strings.Contains(string(raw), bocor) {
			t.Fatalf("%q terbaca di file terenkripsi", bocor)
		}
	}

	// Sesi baru dengan password yang sama membuka isinya kembali.
	membaca, err := crypto.Unlock(idPath, "master-password")
	if err != nil {
		t.Fatalf("Unlock kedua: %v", err)
	}
	v2, err := vault.Open(vaultDir, membaca)
	if err != nil {
		t.Fatalf("Open kedua: %v", err)
	}
	got, err := v2.Load(slug)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.Fields[1].Value != "rahasia123" {
		t.Fatalf("nilai tidak utuh: %+v", got.Fields[1])
	}
}

// Identity lain tidak bisa membaca Secret milik Identity ini.
func TestSecretTidakTerbacaOlehIdentityLain(t *testing.T) {
	dirA, dirB, vaultDir := t.TempDir(), t.TempDir(), t.TempDir()
	if err := crypto.GenerateKeypair(filepath.Join(dirA, "id.age"), filepath.Join(dirA, "rec.pub"), "aaa-master"); err != nil {
		t.Fatalf("keypair A: %v", err)
	}
	if err := crypto.GenerateKeypair(filepath.Join(dirB, "id.age"), filepath.Join(dirB, "rec.pub"), "bbb-master"); err != nil {
		t.Fatalf("keypair B: %v", err)
	}
	a, _ := crypto.Unlock(filepath.Join(dirA, "id.age"), "aaa-master")
	b, _ := crypto.Unlock(filepath.Join(dirB, "id.age"), "bbb-master")

	va, err := vault.Open(vaultDir, a)
	if err != nil {
		t.Fatalf("Open A: %v", err)
	}
	slug, err := va.Create(facebook())
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	vb, err := vault.Open(vaultDir, b)
	if err != nil {
		t.Fatalf("Open B: %v", err)
	}
	if _, err := vb.Load(slug); err == nil {
		t.Fatal("identity lain berhasil membaca secret")
	}
}

// Recipient saja cukup untuk menambah Secret ke Vault tanpa master password.
func TestMenulisKeVaultTanpaMasterPassword(t *testing.T) {
	keyDir, vaultDir := t.TempDir(), t.TempDir()
	idPath := filepath.Join(keyDir, "identity.age")
	recPath := filepath.Join(keyDir, "recipient.pub")
	if err := crypto.GenerateKeypair(idPath, recPath, "master-password"); err != nil {
		t.Fatalf("GenerateKeypair: %v", err)
	}
	writer, err := crypto.RecipientOnly(recPath)
	if err != nil {
		t.Fatalf("RecipientOnly: %v", err)
	}
	v, err := vault.Open(vaultDir, writer)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	s := &secret.Secret{
		Meta:   secret.Meta{Title: "Server Log"},
		Fields: []secret.Field{{Type: "ps", Label: "Token", Value: "tok-123"}},
	}
	slug, err := v.Create(s)
	if err != nil {
		t.Fatalf("Create tanpa master password: %v", err)
	}
	if _, err := v.Load(slug); err == nil {
		t.Fatal("vault recipient-only berhasil membaca")
	}
	reader, _ := crypto.Unlock(idPath, "master-password")
	vr, _ := vault.Open(vaultDir, reader)
	got, err := vr.Load(slug)
	if err != nil {
		t.Fatalf("Load oleh pemilik identity: %v", err)
	}
	if got.Fields[0].Value != "tok-123" {
		t.Fatalf("nilai tidak utuh: %+v", got.Fields[0])
	}
}
