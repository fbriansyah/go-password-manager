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

// nopCipher membiarkan isi apa adanya, sehingga pengujian Vault menguji aturan
// penyimpanan dan bukan kripto.
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
			{Type: "tx", Label: "Username", Value: "budi@mail.com"},
			{Type: "ps", Label: "Password", Value: "rahasia123"},
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
				t.Fatalf("slug = %q, mau %q", slug, "facebook")
			}
			got, err := v.Load(slug)
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			if got.Meta.Title != "Facebook" || len(got.Fields) != 2 {
				t.Fatalf("secret tidak utuh: %+v", got)
			}
			if got.Fields[1].Value != "rahasia123" {
				t.Fatalf("nilai field hilang: %+v", got.Fields[1])
			}
		})
	}
}

func TestCreateMenolakSlugYangSudahDipakai(t *testing.T) {
	for name, v := range vaults(t) {
		t.Run(name, func(t *testing.T) {
			if _, err := v.Create(facebook()); err != nil {
				t.Fatalf("Create pertama: %v", err)
			}
			if _, err := v.Create(facebook()); !errors.Is(err, vault.ErrSlugTaken) {
				t.Fatalf("err = %v, mau ErrSlugTaken", err)
			}
		})
	}
}

func TestSaveMerenameSaatJudulBerubah(t *testing.T) {
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
				t.Fatalf("slug baru = %q, mau %q", newSlug, "meta")
			}
			if _, err := v.Load(slug); !errors.Is(err, vault.ErrNotFound) {
				t.Fatalf("slug lama masih ada: %v", err)
			}
			list, err := v.List()
			if err != nil {
				t.Fatalf("List: %v", err)
			}
			if len(list) != 1 || list[0] != "meta" {
				t.Fatalf("List = %v, mau [meta]", list)
			}
		})
	}
}

func TestSaveMenolakMenimpaSecretLain(t *testing.T) {
	for name, v := range vaults(t) {
		t.Run(name, func(t *testing.T) {
			slug, err := v.Create(facebook())
			if err != nil {
				t.Fatalf("Create: %v", err)
			}
			lain := facebook()
			lain.Meta.Title = "Gmail"
			lain.Fields[1].Value = "punya-gmail"
			if _, err := v.Create(lain); err != nil {
				t.Fatalf("Create kedua: %v", err)
			}

			bentrok := facebook()
			bentrok.Meta.Title = "Gmail"
			if _, err := v.Save(slug, bentrok); !errors.Is(err, vault.ErrSlugTaken) {
				t.Fatalf("err = %v, mau ErrSlugTaken", err)
			}
			// Secret yang menjadi sasaran tabrakan harus tetap utuh.
			gmail, err := v.Load("gmail")
			if err != nil {
				t.Fatalf("Load gmail: %v", err)
			}
			if gmail.Fields[1].Value != "punya-gmail" {
				t.Fatalf("secret lain tertimpa: %+v", gmail.Fields[1])
			}
		})
	}
}

func TestValidasiMenolakSecretTanpaJudul(t *testing.T) {
	for name, v := range vaults(t) {
		t.Run(name, func(t *testing.T) {
			s := facebook()
			s.Meta.Title = "  "
			if _, err := v.Create(s); !errors.Is(err, secret.ErrEmptyTitle) {
				t.Fatalf("err = %v, mau ErrEmptyTitle", err)
			}
		})
	}
}

func TestListMengabaikanFileBukanSecret(t *testing.T) {
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
			t.Fatalf("tulis %s: %v", name, err)
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
		t.Fatalf("List = %v, mau [facebook]", list)
	}
}

func TestOpenMenolakFolderYangTidakAda(t *testing.T) {
	if _, err := vault.Open(filepath.Join(t.TempDir(), "salah-ketik"), nopCipher{}); err == nil {
		t.Fatal("mau error untuk folder yang tidak ada")
	}
}

func TestWriteAtomicTidakMeninggalkanSampah(t *testing.T) {
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
			t.Fatalf("file sementara tertinggal: %s", e.Name())
		}
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
			t.Errorf("Slug(%q) = %q, mau %q", title, got, want)
		}
	}
}
