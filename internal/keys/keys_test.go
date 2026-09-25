package keys_test

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/fbriansyah/go-password-manager/internal/config"
	"github.com/fbriansyah/go-password-manager/internal/crypto"
	"github.com/fbriansyah/go-password-manager/internal/keys"
	"github.com/fbriansyah/go-password-manager/internal/secret"
)

const testPassword = "master-password"

// configured writes a global configuration pointing at key paths that do not
// exist yet. Everything about locating a Vault can be tested from here, with
// no cryptography involved.
func configured(t *testing.T) config.Config {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cfg, cfgPath, err := config.Defaults()
	if err != nil {
		t.Fatalf("Defaults: %v", err)
	}
	if err := config.Write(cfgPath, cfg); err != nil {
		t.Fatalf("Write: %v", err)
	}
	return cfg
}

// keyed is configured plus a real keypair, for the tests that actually open
// the Identity. Only these pay for scrypt.
func keyed(t *testing.T) config.Config {
	t.Helper()
	cfg := configured(t)
	if err := crypto.GenerateKeypair(cfg.PrivateKeyPath, cfg.PublicKeyPath, testPassword); err != nil {
		t.Fatalf("GenerateKeypair: %v", err)
	}
	return cfg
}

// The whole ladder, in the order a real run climbs it.
func TestLocateThenUnlockReadsBackASecret(t *testing.T) {
	cfg := keyed(t)
	vaultDir := t.TempDir()

	loc, err := keys.Locate(vaultDir)
	if err != nil {
		t.Fatalf("Locate: %v", err)
	}
	if loc.Dir != vaultDir {
		t.Fatalf("Dir = %q, want %q", loc.Dir, vaultDir)
	}
	if !loc.Configured {
		t.Fatal("a located config must report Configured")
	}
	if loc.Config.PrivateKeyPath != cfg.PrivateKeyPath {
		t.Fatalf("PrivateKeyPath = %q, want %q", loc.Config.PrivateKeyPath, cfg.PrivateKeyPath)
	}
	if err := loc.RequireIdentity(); err != nil {
		t.Fatalf("RequireIdentity: %v", err)
	}

	store, err := loc.Unlock(testPassword)
	if err != nil {
		t.Fatalf("Unlock: %v", err)
	}
	if _, err := store.Create(&secret.Secret{
		Meta:   secret.Meta{Title: "Facebook"},
		Fields: []secret.Field{{Type: "ps", Label: "Password", Value: "s3cret-value"}},
	}); err != nil {
		t.Fatalf("Create: %v", err)
	}
	back, err := store.Load("facebook")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if back.Fields[0].Value != "s3cret-value" {
		t.Fatalf("value = %q, want the one stored", back.Fields[0].Value)
	}
}

// A Recipient alone opens a Vault well enough to write and to list, and not to
// read (docs/adr/0003, docs/adr/0012).
func TestRecipientWritesButCannotRead(t *testing.T) {
	keyed(t)
	loc, err := keys.Locate(t.TempDir())
	if err != nil {
		t.Fatalf("Locate: %v", err)
	}
	session, err := loc.Recipient()
	if err != nil {
		t.Fatalf("Recipient: %v", err)
	}
	store, err := loc.Vault(session)
	if err != nil {
		t.Fatalf("Vault: %v", err)
	}
	if _, err := store.Create(&secret.Secret{Meta: secret.Meta{Title: "Facebook"}}); err != nil {
		t.Fatalf("Create: %v", err)
	}
	slugs, err := store.List()
	if err != nil || len(slugs) != 1 {
		t.Fatalf("List = %v, %v; want one slug", slugs, err)
	}
	if _, err := store.Load("facebook"); err == nil {
		t.Fatal("a recipient-only session must not read a Secret back")
	}
}

func TestUnlockRefusesTheWrongMasterPassword(t *testing.T) {
	keyed(t)
	loc, err := keys.Locate(t.TempDir())
	if err != nil {
		t.Fatalf("Locate: %v", err)
	}
	if _, err := loc.Unlock("not-the-password"); !errors.Is(err, crypto.ErrWrongPassword) {
		t.Fatalf("err = %v, want ErrWrongPassword", err)
	}
}

// The Vault folder follows -d, and the override is read there rather than in
// $PWD (docs/adr/0001).
func TestTheOverrideIsReadInTheVaultFolder(t *testing.T) {
	configured(t)
	vaultDir := t.TempDir()
	other := filepath.Join(t.TempDir(), "elsewhere.age")
	override := "PRIVATE_KEY_PATH: " + strconv.Quote(other) + "\n"
	if err := os.WriteFile(filepath.Join(vaultDir, config.OverrideName), []byte(override), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	loc, err := keys.Locate(vaultDir)
	if err != nil {
		t.Fatalf("Locate: %v", err)
	}
	if loc.Config.PrivateKeyPath != other {
		t.Fatalf("PrivateKeyPath = %q, want the override at %q", loc.Config.PrivateKeyPath, other)
	}
}

func TestRequireIdentityNamesTheMissingFile(t *testing.T) {
	keyed(t)
	loc, err := keys.Locate(t.TempDir())
	if err != nil {
		t.Fatalf("Locate: %v", err)
	}
	if err := os.Remove(loc.Config.PrivateKeyPath); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	err = loc.RequireIdentity()
	if err == nil || !strings.Contains(err.Error(), loc.Config.PrivateKeyPath) {
		t.Fatalf("err = %v, want one naming the identity path", err)
	}
}

// A mistyped -d fails before anything else is read, rather than becoming a
// new, empty Vault (docs/adr/0001).
func TestAMissingVaultFolderIsRefused(t *testing.T) {
	configured(t)
	missing := filepath.Join(t.TempDir(), "typo")
	_, err := keys.Locate(missing)
	if err == nil || !strings.Contains(err.Error(), "does not exist") {
		t.Fatalf("err = %v, want a refusal naming the folder", err)
	}
}

func TestAFileIsNotAVaultFolder(t *testing.T) {
	configured(t)
	file := filepath.Join(t.TempDir(), "notafolder")
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if _, err := keys.Locate(file); err == nil {
		t.Fatal("a file must not be accepted as a Vault folder")
	}
}

// Locate refuses an unconfigured folder; LocateOrDefaults is the way past it,
// and says which of the two it got (docs/adr/0010).
func TestLocateOrDefaultsFallsBackBeforeAnyConfiguration(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	vaultDir := t.TempDir()

	if _, err := keys.Locate(vaultDir); !errors.Is(err, config.ErrNotConfigured) {
		t.Fatalf("err = %v, want ErrNotConfigured", err)
	}

	loc, err := keys.LocateOrDefaults(vaultDir)
	if err != nil {
		t.Fatalf("LocateOrDefaults: %v", err)
	}
	if loc.Configured {
		t.Fatal("defaults must not report Configured")
	}
	want, wantPath, err := config.Defaults()
	if err != nil {
		t.Fatalf("Defaults: %v", err)
	}
	if loc.Config.PrivateKeyPath != want.PrivateKeyPath || loc.ConfigPath != wantPath {
		t.Fatalf("got %q / %q, want the defaults %q / %q",
			loc.Config.PrivateKeyPath, loc.ConfigPath, want.PrivateKeyPath, wantPath)
	}
	if loc.Config.Generator.Length == 0 {
		t.Fatal("a defaulted Location must still carry a usable Generator Policy")
	}
}

// LocateOrDefaults reports a real configuration as configured, so import-keys
// honours a Vault's override instead of overwriting it (docs/adr/0010).
func TestLocateOrDefaultsPrefersAnExistingConfiguration(t *testing.T) {
	cfg := configured(t)
	loc, err := keys.LocateOrDefaults(t.TempDir())
	if err != nil {
		t.Fatalf("LocateOrDefaults: %v", err)
	}
	if !loc.Configured || loc.Config.PrivateKeyPath != cfg.PrivateKeyPath {
		t.Fatalf("got Configured=%v %q, want the configured %q",
			loc.Configured, loc.Config.PrivateKeyPath, cfg.PrivateKeyPath)
	}
}
