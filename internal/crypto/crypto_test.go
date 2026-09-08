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
	id, _ := keys(t, "a-long-master-password")
	s, err := crypto.Unlock(id, "a-long-master-password")
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

func TestCiphertextIsArmoredAndLeaksNothing(t *testing.T) {
	id, _ := keys(t, "master")
	s, _ := crypto.Unlock(id, "master")
	ct, err := s.Encrypt([]byte("Facebook s3cret-value"))
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	if !strings.HasPrefix(string(ct), "-----BEGIN AGE ENCRYPTED FILE-----") {
		t.Fatalf("not armored ASCII: %.40s", ct)
	}
	if strings.Contains(string(ct), "s3cret-value") {
		t.Fatal("plaintext leaked into the ciphertext")
	}
}

func TestWrongPasswordIsRefused(t *testing.T) {
	id, _ := keys(t, "benar")
	if _, err := crypto.Unlock(id, "wrong"); !errors.Is(err, crypto.ErrWrongPassword) {
		t.Fatalf("err = %v, want ErrWrongPassword", err)
	}
}

func TestIdentityIsStoredEncrypted(t *testing.T) {
	id, _ := keys(t, "master")
	data, err := os.ReadFile(id)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if strings.Contains(string(data), "AGE-SECRET-KEY-") {
		t.Fatal("the private key is stored unencrypted")
	}
	info, err := os.Stat(id)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("identity permissions = %v, want 0600", info.Mode().Perm())
	}
}

// The Recipient alone is enough to write but not to read — this is one reason
// the design is asymmetric (docs/adr/0003).
func TestRecipientOnlyWritesButCannotRead(t *testing.T) {
	idPath, recPath := keys(t, "master")
	writer, err := crypto.RecipientOnly(recPath)
	if err != nil {
		t.Fatalf("RecipientOnly: %v", err)
	}
	if writer.CanRead() {
		t.Fatal("a recipient-only session claims it can read")
	}
	ct, err := writer.Encrypt([]byte("from a machine without an identity"))
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	if _, err := writer.Decrypt(ct); err == nil {
		t.Fatal("a recipient-only session managed to decrypt")
	}
	reader, err := crypto.Unlock(idPath, "master")
	if err != nil {
		t.Fatalf("Unlock: %v", err)
	}
	plain, err := reader.Decrypt(ct)
	if err != nil {
		t.Fatalf("Decrypt by the identity owner: %v", err)
	}
	if string(plain) != "from a machine without an identity" {
		t.Fatalf("plaintext berubah: %s", plain)
	}
}

func TestGenerateKeypairRefusesToOverwriteAnIdentity(t *testing.T) {
	id, rec := keys(t, "master")
	err := crypto.GenerateKeypair(id, rec, "master")
	if err == nil {
		t.Fatal("want an error; an existing key must not be overwritten")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("the error message does not explain itself: %v", err)
	}
}
