package export_test

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/fbriansyah/go-password-manager/internal/config"
	"github.com/fbriansyah/go-password-manager/internal/crypto"
	"github.com/fbriansyah/go-password-manager/internal/export"
)

func destConfig(t *testing.T) config.Config {
	t.Helper()
	dir := t.TempDir()
	return config.Config{
		PrivateKeyPath: filepath.Join(dir, "sub", "identity.age"),
		PublicKeyPath:  filepath.Join(dir, "sub", "recipient.pub"),
	}
}

func buildZip(t *testing.T, entries map[string][]byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, data := range entries {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatalf("zw.Create(%s): %v", name, err)
		}
		if _, err := w.Write(data); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("zw.Close: %v", err)
	}
	return buf.Bytes()
}

func exportZip(t *testing.T, cfg config.Config, password string) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := export.Keys(cfg, password, &buf); err != nil {
		t.Fatalf("Keys: %v", err)
	}
	return buf.Bytes()
}

func TestImportRoundTrip(t *testing.T) {
	src := testConfig(t, "correct horse battery staple")
	zipData := exportZip(t, src, "correct horse battery staple")

	dst := destConfig(t)
	if err := export.Import(zipData, dst, "correct horse battery staple"); err != nil {
		t.Fatalf("Import: %v", err)
	}

	if _, err := crypto.Unlock(dst.PrivateKeyPath, "correct horse battery staple"); err != nil {
		t.Fatalf("imported identity does not unlock: %v", err)
	}
	wantIdentity, _ := os.ReadFile(src.PrivateKeyPath)
	gotIdentity, _ := os.ReadFile(dst.PrivateKeyPath)
	if !bytes.Equal(wantIdentity, gotIdentity) {
		t.Error("imported identity.age does not match the exported one")
	}
	wantRecipient, _ := os.ReadFile(src.PublicKeyPath)
	gotRecipient, _ := os.ReadFile(dst.PublicKeyPath)
	if !bytes.Equal(wantRecipient, gotRecipient) {
		t.Error("imported recipient.pub does not match the exported one")
	}
}

func TestImportRefusesExistingKeypair(t *testing.T) {
	src := testConfig(t, "correct horse battery staple")
	zipData := exportZip(t, src, "correct horse battery staple")

	dst := testConfig(t, "some other password")
	before, _ := os.ReadFile(dst.PrivateKeyPath)

	err := export.Import(zipData, dst, "correct horse battery staple")
	if err == nil {
		t.Fatal("Import succeeded despite an existing keypair")
	}
	after, _ := os.ReadFile(dst.PrivateKeyPath)
	if !bytes.Equal(before, after) {
		t.Error("Import modified the existing identity.age")
	}
}

func TestImportRefusesWrongPassword(t *testing.T) {
	src := testConfig(t, "correct horse battery staple")
	zipData := exportZip(t, src, "correct horse battery staple")

	dst := destConfig(t)
	if err := export.Import(zipData, dst, "wrong password entirely"); err == nil {
		t.Fatal("Import succeeded with the wrong password")
	}
	if _, err := os.Stat(dst.PrivateKeyPath); err == nil {
		t.Error("Import left an identity.age behind after failing")
	}
}

func TestImportRefusesChecksumMismatch(t *testing.T) {
	src := testConfig(t, "correct horse battery staple")
	identity, _ := os.ReadFile(src.PrivateKeyPath)
	recipient, _ := os.ReadFile(src.PublicKeyPath)
	manifestJSON := []byte(`{"format_version":1,"app":"gopm","exported_at":"2026-01-01T00:00:00Z","identity_sha256":"0000000000000000000000000000000000000000000000000000000000000000"}`)
	zipData := buildZip(t, map[string][]byte{
		"identity.age":  identity,
		"recipient.pub": recipient,
		"manifest.json": manifestJSON,
	})

	dst := destConfig(t)
	if err := export.Import(zipData, dst, "correct horse battery staple"); err == nil {
		t.Fatal("Import succeeded despite a checksum mismatch")
	}
}

func TestImportRefusesMismatchedRecipient(t *testing.T) {
	srcA := testConfig(t, "password for a")
	srcB := testConfig(t, "password for b")
	zipA := exportZip(t, srcA, "password for a")

	zr, err := zip.NewReader(bytes.NewReader(zipA), int64(len(zipA)))
	if err != nil {
		t.Fatalf("zip.NewReader: %v", err)
	}
	entries := map[string][]byte{}
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("open %s: %v", f.Name, err)
		}
		var b bytes.Buffer
		b.ReadFrom(rc)
		rc.Close()
		entries[f.Name] = b.Bytes()
	}
	entries["recipient.pub"], _ = os.ReadFile(srcB.PublicKeyPath)
	zipData := buildZip(t, entries)

	dst := destConfig(t)
	if err := export.Import(zipData, dst, "password for a"); err == nil {
		t.Fatal("Import succeeded with a recipient from a different keypair")
	}
	if _, err := os.Stat(dst.PrivateKeyPath); err == nil {
		t.Error("Import left an identity.age behind after failing")
	}
}

func TestImportRefusesMissingManifest(t *testing.T) {
	src := testConfig(t, "correct horse battery staple")
	identity, _ := os.ReadFile(src.PrivateKeyPath)
	recipient, _ := os.ReadFile(src.PublicKeyPath)
	zipData := buildZip(t, map[string][]byte{
		"identity.age":  identity,
		"recipient.pub": recipient,
	})

	dst := destConfig(t)
	if err := export.Import(zipData, dst, "correct horse battery staple"); err == nil {
		t.Fatal("Import succeeded without a manifest")
	}
}

func TestImportRefusesUnknownFormatVersion(t *testing.T) {
	src := testConfig(t, "correct horse battery staple")
	identity, _ := os.ReadFile(src.PrivateKeyPath)
	recipient, _ := os.ReadFile(src.PublicKeyPath)
	manifestJSON := []byte(`{"format_version":2,"app":"gopm","exported_at":"2026-01-01T00:00:00Z","identity_sha256":"x"}`)
	zipData := buildZip(t, map[string][]byte{
		"identity.age":  identity,
		"recipient.pub": recipient,
		"manifest.json": manifestJSON,
	})

	dst := destConfig(t)
	if err := export.Import(zipData, dst, "correct horse battery staple"); err == nil {
		t.Fatal("Import succeeded with an unrecognized format_version")
	}
}
