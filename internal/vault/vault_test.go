package vault_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fbriansyah/go-password-manager/internal/secret"
	"github.com/fbriansyah/go-password-manager/internal/vault"
)

// nopCipher leaves content as it is, so the Vault tests exercise the storage
// rules rather than the cryptography.
type nopCipher struct{}

func (nopCipher) Encrypt(b []byte) ([]byte, error) { return b, nil }
func (nopCipher) Decrypt(b []byte) ([]byte, error) { return b, nil }

func vaults(t *testing.T) map[string]vault.Vault {
	t.Helper()
	fs, err := vault.Open(t.TempDir(), nopCipher{})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	return map[string]vault.Vault{"fs": fs, "mem": vault.NewMem(nopCipher{})}
}

func facebook() *secret.Secret {
	return &secret.Secret{
		Meta: secret.Meta{Title: "Facebook", Description: "Facebook Credential", Tags: []string{"app"}},
		Fields: []secret.Field{
			{Type: "tx", Label: "Username", Value: "user@mail.com"},
			{Type: "ps", Label: "Password", Value: "s3cret-value"},
		},
	}
}

func TestCreateThenLoadRoundTrip(t *testing.T) {
	for name, v := range vaults(t) {
		t.Run(name, func(t *testing.T) {
			slug, err := v.Create(facebook())
			if err != nil {
				t.Fatalf("Create: %v", err)
			}
			if slug != "facebook" {
				t.Fatalf("slug = %q, want %q", slug, "facebook")
			}
			got, err := v.Load(slug)
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			if got.Meta.Title != "Facebook" || len(got.Fields) != 2 {
				t.Fatalf("secret did not survive: %+v", got)
			}
			if got.Fields[1].Value != "s3cret-value" {
				t.Fatalf("field value was lost: %+v", got.Fields[1])
			}
		})
	}
}

func TestCreateRefusesATakenSlug(t *testing.T) {
	for name, v := range vaults(t) {
		t.Run(name, func(t *testing.T) {
			if _, err := v.Create(facebook()); err != nil {
				t.Fatalf("first Create: %v", err)
			}
			if _, err := v.Create(facebook()); !errors.Is(err, vault.ErrSlugTaken) {
				t.Fatalf("err = %v, want ErrSlugTaken", err)
			}
		})
	}
}

func TestSaveRenamesWhenTheTitleChanges(t *testing.T) {
	for name, v := range vaults(t) {
		t.Run(name, func(t *testing.T) {
			slug, err := v.Create(facebook())
			if err != nil {
				t.Fatalf("Create: %v", err)
			}
			s := facebook()
			s.Meta.Title = "Meta"
			newSlug, err := v.Save(slug, s)
			if err != nil {
				t.Fatalf("Save: %v", err)
			}
			if newSlug != "meta" {
				t.Fatalf("new slug = %q, want %q", newSlug, "meta")
			}
			if _, err := v.Load(slug); !errors.Is(err, vault.ErrNotFound) {
				t.Fatalf("the old slug is still there: %v", err)
			}
			list, err := v.List()
			if err != nil {
				t.Fatalf("List: %v", err)
			}
			if len(list) != 1 || list[0] != "meta" {
				t.Fatalf("List = %v, want [meta]", list)
			}
		})
	}
}

func TestSaveRefusesToOverwriteAnotherSecret(t *testing.T) {
	for name, v := range vaults(t) {
		t.Run(name, func(t *testing.T) {
			slug, err := v.Create(facebook())
			if err != nil {
				t.Fatalf("Create: %v", err)
			}
			other := facebook()
			other.Meta.Title = "Gmail"
			other.Fields[1].Value = "gmail-secret"
			if _, err := v.Create(other); err != nil {
				t.Fatalf("second Create: %v", err)
			}

			clashing := facebook()
			clashing.Meta.Title = "Gmail"
			if _, err := v.Save(slug, clashing); !errors.Is(err, vault.ErrSlugTaken) {
				t.Fatalf("err = %v, want ErrSlugTaken", err)
			}
			// The Secret that was collided with must be left intact.
			gmail, err := v.Load("gmail")
			if err != nil {
				t.Fatalf("Load gmail: %v", err)
			}
			if gmail.Fields[1].Value != "gmail-secret" {
				t.Fatalf("the other secret was overwritten: %+v", gmail.Fields[1])
			}
		})
	}
}

func TestValidationRefusesASecretWithoutATitle(t *testing.T) {
	for name, v := range vaults(t) {
		t.Run(name, func(t *testing.T) {
			s := facebook()
			s.Meta.Title = "  "
			if _, err := v.Create(s); !errors.Is(err, secret.ErrEmptyTitle) {
				t.Fatalf("err = %v, want ErrEmptyTitle", err)
			}
		})
	}
}

func TestListIgnoresNonSecretFiles(t *testing.T) {
	dir := t.TempDir()
	v, err := vault.Open(dir, nopCipher{})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if _, err := v.Create(facebook()); err != nil {
		t.Fatalf("Create: %v", err)
	}
	for _, name := range []string{"README.md", "main.go", ".gopm.yaml"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	if err := os.Mkdir(filepath.Join(dir, "staging"), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	list, err := v.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 1 || list[0] != "facebook" {
		t.Fatalf("List = %v, want [facebook]", list)
	}
}

func TestOpenRefusesAMissingFolder(t *testing.T) {
	if _, err := vault.Open(filepath.Join(t.TempDir(), "typo"), nopCipher{}); err == nil {
		t.Fatal("want an error for a folder that does not exist")
	}
}

func TestWriteAtomicLeavesNoLeftovers(t *testing.T) {
	dir := t.TempDir()
	v, err := vault.Open(dir, nopCipher{})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if _, err := v.Create(facebook()); err != nil {
		t.Fatalf("Create: %v", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".tmp") {
			t.Fatalf("a temporary file was left behind: %s", e.Name())
		}
	}
}

func TestEditThenDelete(t *testing.T) {
	for name, v := range vaults(t) {
		t.Run(name, func(t *testing.T) {
			slug, err := v.Create(facebook())
			if err != nil {
				t.Fatalf("Create: %v", err)
			}
			s := facebook()
			s.Fields[1].Value = "rotated-value"
			newSlug, err := v.Save(slug, s)
			if err != nil {
				t.Fatalf("Save: %v", err)
			}
			if newSlug != slug {
				t.Fatalf("slug changed on a value-only edit: %q -> %q", slug, newSlug)
			}
			got, err := v.Load(newSlug)
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			if got.Fields[1].Value != "rotated-value" {
				t.Fatalf("edit did not persist: %+v", got.Fields[1])
			}

			if err := v.Delete(newSlug); err != nil {
				t.Fatalf("Delete: %v", err)
			}
			if _, err := v.Load(newSlug); !errors.Is(err, vault.ErrNotFound) {
				t.Fatalf("err = %v, want ErrNotFound after delete", err)
			}
			list, err := v.List()
			if err != nil {
				t.Fatalf("List: %v", err)
			}
			if len(list) != 0 {
				t.Fatalf("List = %v, want empty after delete", list)
			}
		})
	}
}

func TestDeleteASecretThatDoesNotExist(t *testing.T) {
	for name, v := range vaults(t) {
		t.Run(name, func(t *testing.T) {
			if err := v.Delete("no-such-secret"); !errors.Is(err, vault.ErrNotFound) {
				t.Fatalf("err = %v, want ErrNotFound", err)
			}
		})
	}
}

func TestSlug(t *testing.T) {
	cases := map[string]string{
		"Facebook":           "facebook",
		"Bank BCA":           "bank-bca",
		"  GitHub  ":         "github",
		"e-mail / kantor":    "e-mail-kantor",
		"Akun #1 (pribadi)":  "akun-1-pribadi",
		"...":                "",
		"Google — Workspace": "google-workspace",
	}
	for title, want := range cases {
		if got := vault.Slug(title); got != want {
			t.Errorf("Slug(%q) = %q, want %q", title, got, want)
		}
	}
}
