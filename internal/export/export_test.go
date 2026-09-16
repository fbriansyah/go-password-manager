package export_test

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/fbriansyah/go-password-manager/internal/config"
	"github.com/fbriansyah/go-password-manager/internal/crypto"
	"github.com/fbriansyah/go-password-manager/internal/export"
)

func testConfig(t *testing.T, password string) config.Config {
	t.Helper()
	dir := t.TempDir()
	cfg := config.Config{
		PrivateKeyPath: filepath.Join(dir, "identity.age"),
		PublicKeyPath:  filepath.Join(dir, "recipient.pub"),
	}
	if err := crypto.GenerateKeypair(cfg.PrivateKeyPath, cfg.PublicKeyPath, password); err != nil {
		t.Fatalf("GenerateKeypair: %v", err)
	}
	return cfg
}

func TestKeysWritesIdentityRecipientAndManifest(t *testing.T) {
	cfg := testConfig(t, "correct horse battery staple")

	var buf bytes.Buffer
	if err := export.Keys(cfg, "correct horse battery staple", &buf); err != nil {
		t.Fatalf("Keys: %v", err)
	}

	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatalf("zip.NewReader: %v", err)
	}

	files := map[string][]byte{}
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("open %s: %v", f.Name, err)
		}
		var b bytes.Buffer
		if _, err := b.ReadFrom(rc); err != nil {
			t.Fatalf("read %s: %v", f.Name, err)
		}
		rc.Close()
		files[f.Name] = b.Bytes()
	}

	for _, name := range []string{"identity.age", "recipient.pub", "manifest.json"} {
		if _, ok := files[name]; !ok {
			t.Errorf("archive is missing %s", name)
		}
	}

	var m struct {
		FormatVersion  int    `json:"format_version"`
		App            string `json:"app"`
		ExportedAt     string `json:"exported_at"`
		IdentitySHA256 string `json:"identity_sha256"`
	}
	if err := json.Unmarshal(files["manifest.json"], &m); err != nil {
		t.Fatalf("unmarshal manifest: %v", err)
	}
	if m.FormatVersion != export.FormatVersion {
		t.Errorf("format_version = %d, want %d", m.FormatVersion, export.FormatVersion)
	}
	if m.App != "gopm" {
		t.Errorf("app = %q, want gopm", m.App)
	}
	if m.ExportedAt == "" {
		t.Error("exported_at is empty")
	}

	sum := sha256.Sum256(files["identity.age"])
	if want := hex.EncodeToString(sum[:]); m.IdentitySHA256 != want {
		t.Errorf("identity_sha256 = %q, want %q", m.IdentitySHA256, want)
	}
}

func TestKeysRefusesWrongPassword(t *testing.T) {
	cfg := testConfig(t, "correct horse battery staple")

	var buf bytes.Buffer
	err := export.Keys(cfg, "wrong password entirely", &buf)
	if err == nil {
		t.Fatal("Keys succeeded with the wrong password")
	}
	if buf.Len() != 0 {
		t.Errorf("Keys wrote %d bytes despite failing", buf.Len())
	}
}
