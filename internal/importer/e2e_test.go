package importer_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fbriansyah/go-password-manager/internal/crypto"
	"github.com/fbriansyah/go-password-manager/internal/importer"
	"github.com/fbriansyah/go-password-manager/internal/vault"
)

// The whole import flow against a real Vault on disk, with real encryption:
// the Mem-backed tests prove the mapping, this proves the result is a Vault
// the application can actually open.
func TestImportIntoARealVault(t *testing.T) {
	keyDir, vaultDir := t.TempDir(), t.TempDir()
	idPath := filepath.Join(keyDir, "identity.age")
	recPath := filepath.Join(keyDir, "recipient.pub")
	if err := crypto.GenerateKeypair(idPath, recPath, "master-password"); err != nil {
		t.Fatalf("GenerateKeypair: %v", err)
	}

	// A dry run needs no Master Password: it writes nothing, so the Recipient
	// alone is enough to see which slugs are taken (docs/adr/0012).
	reader, err := crypto.RecipientOnly(recPath)
	if err != nil {
		t.Fatalf("RecipientOnly: %v", err)
	}
	preview, err := vault.Open(vaultDir, reader)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	data := onePasswordCSV(
		`GitHub,https://github.com,octocat,hunter2,otpauth://totp/?secret=JBHB4LQQCADBOM3P,false,false,work,`,
		`GitHub,,other,swordfish,,true,false,work;personal,"line one`+"\n"+`line two"`,
	)
	format, err := importer.Detect(data)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	plan, err := importer.Build(format, data, preview)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if len(plan.Renamed) != 1 {
		t.Fatalf("renamed = %+v, want the second GitHub shifted", plan.Renamed)
	}
	if entries, _ := os.ReadDir(vaultDir); len(entries) != 0 {
		t.Fatalf("a dry run wrote %d files, want none", len(entries))
	}

	writer, err := crypto.Unlock(idPath, "master-password")
	if err != nil {
		t.Fatalf("Unlock: %v", err)
	}
	store, err := vault.Open(vaultDir, writer)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	imported, err := plan.Apply(store)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if imported != 2 {
		t.Fatalf("imported = %d, want 2", imported)
	}

	// Nothing but the slug is readable on disk.
	raw, err := os.ReadFile(filepath.Join(vaultDir, "github"+vault.Ext))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !strings.HasPrefix(string(raw), "-----BEGIN AGE ENCRYPTED FILE-----") {
		t.Fatalf("file is not armored: %.40s", raw)
	}
	for _, leaked := range []string{"hunter2", "octocat", "JBHB4LQQCADBOM3P"} {
		if strings.Contains(string(raw), leaked) {
			t.Errorf("%q is readable in the stored file", leaked)
		}
	}

	// A fresh session — as if the app were closed and reopened — reads both.
	fresh, err := crypto.Unlock(idPath, "master-password")
	if err != nil {
		t.Fatalf("Unlock: %v", err)
	}
	reopened, err := vault.Open(vaultDir, fresh)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	first, err := reopened.Load("github")
	if err != nil {
		t.Fatalf("Load(github): %v", err)
	}
	if got := first.Fields[2].Value; got != "otpauth://totp/?secret=JBHB4LQQCADBOM3P" {
		t.Errorf("TOTP field = %q, want the seed stored as handed over", got)
	}
	second, err := reopened.Load("github-2")
	if err != nil {
		t.Fatalf("Load(github-2): %v", err)
	}
	if second.Meta.Title != "GitHub 2" {
		t.Errorf("title = %q, want %q", second.Meta.Title, "GitHub 2")
	}
	if got := second.Fields[len(second.Fields)-1].Value; got != "line one\nline two" {
		t.Errorf("notes = %q, want both lines", got)
	}
}
