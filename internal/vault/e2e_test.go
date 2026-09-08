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

// The whole Milestone 1 flow: create a keypair, unlock, store a Secret, read
// it back through a fresh session — exactly like closing and reopening the app.
func TestCreateUnlockReadBackFlow(t *testing.T) {
	keyDir, vaultDir := t.TempDir(), t.TempDir()
	idPath := filepath.Join(keyDir, "identity.age")
	recPath := filepath.Join(keyDir, "recipient.pub")
	if err := crypto.GenerateKeypair(idPath, recPath, "master-password"); err != nil {
		t.Fatalf("GenerateKeypair: %v", err)
	}

	writer, err := crypto.Unlock(idPath, "master-password")
	if err != nil {
		t.Fatalf("Unlock: %v", err)
	}
	v, err := vault.Open(vaultDir, writer)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	slug, err := v.Create(facebook())
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// The file on disk must be armored and leak nothing beyond the slug.
	raw, err := os.ReadFile(filepath.Join(vaultDir, slug+vault.Ext))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !strings.HasPrefix(string(raw), "-----BEGIN AGE ENCRYPTED FILE-----") {
		t.Fatalf("file is not armored: %.40s", raw)
	}
	for _, leaked := range []string{"s3cret-value", "user@mail.com", "Facebook Credential"} {
		if strings.Contains(string(raw), leaked) {
			t.Fatalf("%q is readable in the encrypted file", leaked)
		}
	}

	// A new session with the same password opens the contents again.
	reader2, err := crypto.Unlock(idPath, "master-password")
	if err != nil {
		t.Fatalf("second Unlock: %v", err)
	}
	v2, err := vault.Open(vaultDir, reader2)
	if err != nil {
		t.Fatalf("second Open: %v", err)
	}
	got, err := v2.Load(slug)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.Fields[1].Value != "s3cret-value" {
		t.Fatalf("value did not survive: %+v", got.Fields[1])
	}
}

// A different Identity cannot read a Secret belonging to this one.
func TestSecretIsUnreadableByAnotherIdentity(t *testing.T) {
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
		t.Fatal("another identity managed to read the secret")
	}
}

// The Recipient alone is enough to add a Secret without the master password.
func TestWritingToAVaultWithoutTheMasterPassword(t *testing.T) {
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
		t.Fatalf("Create without the master password: %v", err)
	}
	if _, err := v.Load(slug); err == nil {
		t.Fatal("a recipient-only vault managed to read")
	}
	reader, _ := crypto.Unlock(idPath, "master-password")
	vr, _ := vault.Open(vaultDir, reader)
	got, err := vr.Load(slug)
	if err != nil {
		t.Fatalf("Load by the identity owner: %v", err)
	}
	if got.Fields[0].Value != "tok-123" {
		t.Fatalf("value did not survive: %+v", got.Fields[0])
	}
}
