package cmd

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fbriansyah/go-password-manager/internal/config"
	"github.com/fbriansyah/go-password-manager/internal/crypto"
	"github.com/fbriansyah/go-password-manager/internal/vault"
)

const testPassword = "master-password"

// gopm runs the real command tree with args, capturing what it printed. The
// tree is rebuilt every time, so the flag variables never leak between tests.
func gopm(t *testing.T, args ...string) (string, error) {
	t.Helper()
	root := newRoot()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs(args)
	err := root.Execute()
	return out.String(), err
}

// answers puts a scripted prompt in askPassword's place, handing back the
// given replies in order. A prompt beyond the script is an error, so a command
// that asks more often than a test expects fails rather than hangs.
func answers(t *testing.T, replies ...string) {
	t.Helper()
	original := askPassword
	i := 0
	askPassword = func(string) (string, error) {
		if i >= len(replies) {
			return "", errors.New("unexpected password prompt")
		}
		i++
		return replies[i-1], nil
	}
	t.Cleanup(func() { askPassword = original })
}

// initialised runs `gopm init` into a fresh configuration folder and returns
// the Config it wrote.
func initialised(t *testing.T) config.Config {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	answers(t, testPassword, testPassword)
	if _, err := gopm(t, "init"); err != nil {
		t.Fatalf("init: %v", err)
	}
	cfg, _, err := config.Defaults()
	if err != nil {
		t.Fatalf("Defaults: %v", err)
	}
	return cfg
}

func TestInitCreatesTheKeypairAndTheConfiguration(t *testing.T) {
	cfg := initialised(t)
	for _, p := range []string{cfg.PrivateKeyPath, cfg.PublicKeyPath} {
		if _, err := os.Stat(p); err != nil {
			t.Fatalf("%s was not written: %v", p, err)
		}
	}
	if _, err := crypto.Unlock(cfg.PrivateKeyPath, testPassword); err != nil {
		t.Fatalf("the identity init wrote does not open: %v", err)
	}
}

func TestInitRefusesAShortMasterPassword(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	answers(t, "short")
	_, err := gopm(t, "init")
	if err == nil || !strings.Contains(err.Error(), "8 characters") {
		t.Fatalf("err = %v, want a refusal naming the minimum length", err)
	}
}

func TestInitRefusesAMistypedRepeat(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	answers(t, testPassword, "master-passwerd")
	_, err := gopm(t, "init")
	if err == nil || !strings.Contains(err.Error(), "do not match") {
		t.Fatalf("err = %v, want a refusal about the repeat", err)
	}
}

// Starting over is the user's decision to make deliberately, never init's.
func TestInitRefusesAnExistingConfiguration(t *testing.T) {
	initialised(t)
	answers(t, testPassword, testPassword)
	_, err := gopm(t, "init")
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("err = %v, want a refusal to overwrite", err)
	}
}

func TestExportKeysWritesAZip(t *testing.T) {
	initialised(t)
	out := filepath.Join(t.TempDir(), "keys.zip")
	answers(t, testPassword)
	printed, err := gopm(t, "export-keys", "-d", t.TempDir(), "-o", out)
	if err != nil {
		t.Fatalf("export-keys: %v", err)
	}
	if _, err := os.Stat(out); err != nil {
		t.Fatalf("the zip was not written: %v", err)
	}
	if !strings.Contains(printed, out) {
		t.Fatalf("output %q does not name the zip", printed)
	}
}

func TestExportKeysRefusesAnExistingOutput(t *testing.T) {
	initialised(t)
	out := filepath.Join(t.TempDir(), "keys.zip")
	if err := os.WriteFile(out, []byte("not mine"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	answers(t, testPassword)
	_, err := gopm(t, "export-keys", "-o", out)
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("err = %v, want a refusal to overwrite", err)
	}
	if data, _ := os.ReadFile(out); string(data) != "not mine" {
		t.Fatal("the existing file was touched")
	}
}

func TestExportKeysRefusesTheWrongMasterPassword(t *testing.T) {
	initialised(t)
	out := filepath.Join(t.TempDir(), "keys.zip")
	answers(t, "not-the-password")
	if _, err := gopm(t, "export-keys", "-o", out); !errors.Is(err, crypto.ErrWrongPassword) {
		t.Fatalf("err = %v, want ErrWrongPassword", err)
	}
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		t.Fatal("a refused export must leave no zip behind")
	}
}

// The Identity opens with the new password afterwards, and not with the old
// one (docs/adr/0011).
func TestChangeMasterPasswordReseals(t *testing.T) {
	cfg := initialised(t)
	answers(t, testPassword, "new-master-password", "new-master-password")
	if _, err := gopm(t, "change-master-password"); err != nil {
		t.Fatalf("change-master-password: %v", err)
	}
	if _, err := crypto.Unlock(cfg.PrivateKeyPath, "new-master-password"); err != nil {
		t.Fatalf("the resealed identity does not open with the new password: %v", err)
	}
	if _, err := crypto.Unlock(cfg.PrivateKeyPath, testPassword); !errors.Is(err, crypto.ErrWrongPassword) {
		t.Fatal("the old master password still opens the identity")
	}
}

func TestChangeMasterPasswordRefusesTheCurrentOneAsNew(t *testing.T) {
	initialised(t)
	answers(t, testPassword, testPassword)
	_, err := gopm(t, "change-master-password")
	if err == nil || !strings.Contains(err.Error(), "same as the current one") {
		t.Fatalf("err = %v, want a refusal to reuse the password", err)
	}
}

func TestChangeMasterPasswordRefusesAWrongCurrentPassword(t *testing.T) {
	initialised(t)
	answers(t, "not-the-password")
	if _, err := gopm(t, "change-master-password"); !errors.Is(err, crypto.ErrWrongPassword) {
		t.Fatalf("err = %v, want ErrWrongPassword", err)
	}
}

// export-keys then import-keys into a configuration folder that has nothing in
// it: the one path that runs before any configuration exists (docs/adr/0010).
func TestImportKeysInstallsIntoAFreshConfiguration(t *testing.T) {
	source := initialised(t)
	zip := filepath.Join(t.TempDir(), "keys.zip")
	answers(t, testPassword)
	if _, err := gopm(t, "export-keys", "-o", zip); err != nil {
		t.Fatalf("export-keys: %v", err)
	}

	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	answers(t, testPassword)
	printed, err := gopm(t, "import-keys", zip)
	if err != nil {
		t.Fatalf("import-keys: %v", err)
	}

	cfg, cfgPath, err := config.Defaults()
	if err != nil {
		t.Fatalf("Defaults: %v", err)
	}
	if !strings.Contains(printed, cfgPath) {
		t.Fatalf("output %q does not name the configuration it wrote", printed)
	}
	if _, err := config.Load(t.TempDir()); err != nil {
		t.Fatalf("the written configuration does not load: %v", err)
	}
	installed, err := os.ReadFile(cfg.PublicKeyPath)
	if err != nil {
		t.Fatalf("recipient: %v", err)
	}
	original, err := os.ReadFile(source.PublicKeyPath)
	if err != nil {
		t.Fatalf("source recipient: %v", err)
	}
	if string(installed) != string(original) {
		t.Fatal("the installed recipient is not the exported one")
	}
}

// An existing keypair is never overwritten; there is no --force (docs/adr/0010).
func TestImportKeysRefusesAnExistingKeypair(t *testing.T) {
	initialised(t)
	zip := filepath.Join(t.TempDir(), "keys.zip")
	answers(t, testPassword)
	if _, err := gopm(t, "export-keys", "-o", zip); err != nil {
		t.Fatalf("export-keys: %v", err)
	}
	answers(t, testPassword)
	if _, err := gopm(t, "import-keys", zip); err == nil {
		t.Fatal("import-keys must refuse to replace a keypair that is already there")
	}
}

const onePasswordCSV = `Title,Url,Username,Password,OTPAuth,Favorite,Archived,Tags,Notes
Facebook,https://facebook.com,user@mail.com,s3cret,,false,false,social,
Packtpub,https://packtpub.com,user@mail.com,b00ks,,true,false,,a note
`

func writeCSV(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "export.csv")
	if err := os.WriteFile(path, []byte(onePasswordCSV), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	return path
}

// A dry run writes nothing and needs no Master Password at all: the Recipient
// alone lists the slugs already taken (docs/adr/0012).
func TestImportSecretsDryRunWritesNothingAndNeverPrompts(t *testing.T) {
	initialised(t)
	vaultDir := t.TempDir()
	answers(t) // any prompt at all is a failure
	printed, err := gopm(t, "import-secrets", "--dry-run", "-d", vaultDir, writeCSV(t))
	if err != nil {
		t.Fatalf("import-secrets --dry-run: %v", err)
	}
	if !strings.Contains(printed, "1password") || !strings.Contains(printed, "Secrets    : 2") {
		t.Fatalf("output %q does not report the plan", printed)
	}
	if !strings.Contains(printed, "Nothing was written") {
		t.Fatalf("output %q does not say it wrote nothing", printed)
	}
	entries, err := os.ReadDir(vaultDir)
	if err != nil || len(entries) != 0 {
		t.Fatalf("the vault holds %d files after a dry run, want 0", len(entries))
	}
}

func TestImportSecretsFillsTheVault(t *testing.T) {
	cfg := initialised(t)
	vaultDir := t.TempDir()
	answers(t, testPassword)
	printed, err := gopm(t, "import-secrets", "-d", vaultDir, writeCSV(t))
	if err != nil {
		t.Fatalf("import-secrets: %v", err)
	}
	if !strings.Contains(printed, "Imported   : 2 of 2") {
		t.Fatalf("output %q does not report both secrets", printed)
	}

	session, err := crypto.Unlock(cfg.PrivateKeyPath, testPassword)
	if err != nil {
		t.Fatalf("Unlock: %v", err)
	}
	store, err := vault.Open(vaultDir, session)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	s, err := store.Load("facebook")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if s.Fields[1].Value != "s3cret" {
		t.Fatalf("the imported password is %q, want the one in the file", s.Fields[1].Value)
	}
}

// Import only ever adds: a title already in the Vault is numbered, never
// overwritten (docs/adr/0012).
func TestImportSecretsNumbersATitleAlreadyTaken(t *testing.T) {
	initialised(t)
	vaultDir := t.TempDir()
	answers(t, testPassword, testPassword)
	csv := writeCSV(t)
	if _, err := gopm(t, "import-secrets", "-d", vaultDir, csv); err != nil {
		t.Fatalf("first import: %v", err)
	}
	printed, err := gopm(t, "import-secrets", "-d", vaultDir, csv)
	if err != nil {
		t.Fatalf("second import: %v", err)
	}
	if !strings.Contains(printed, "Renamed    : 2") || !strings.Contains(printed, `"Facebook 2"`) {
		t.Fatalf("output %q does not report the renames", printed)
	}
	if _, err := os.Stat(filepath.Join(vaultDir, "facebook-2.gopm")); err != nil {
		t.Fatalf("facebook-2.gopm was not written: %v", err)
	}
}

func TestImportSecretsRefusesAFileNoFormatRecognises(t *testing.T) {
	initialised(t)
	path := filepath.Join(t.TempDir(), "mystery.csv")
	if err := os.WriteFile(path, []byte("a,b,c\n1,2,3\n"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	answers(t)
	_, err := gopm(t, "import-secrets", "--dry-run", "-d", t.TempDir(), path)
	if err == nil || !strings.Contains(err.Error(), "--from") {
		t.Fatalf("err = %v, want a refusal pointing at --from", err)
	}
}

// -d is checked before anything else is read, so a typo fails plainly rather
// than as a confusing configuration error (docs/adr/0001).
func TestAMistypedDirectoryIsRefused(t *testing.T) {
	initialised(t)
	missing := filepath.Join(t.TempDir(), "typo")
	answers(t)
	_, err := gopm(t, "export-keys", "-d", missing, "-o", filepath.Join(t.TempDir(), "k.zip"))
	if err == nil || !strings.Contains(err.Error(), "does not exist") {
		t.Fatalf("err = %v, want a refusal naming the folder", err)
	}
}

func TestACommandWithoutAnyConfigurationSaysToRunInit(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	answers(t)
	_, err := gopm(t, "export-keys", "-d", t.TempDir(), "-o", filepath.Join(t.TempDir(), "k.zip"))
	if !errors.Is(err, config.ErrNotConfigured) {
		t.Fatalf("err = %v, want ErrNotConfigured", err)
	}
}

// The Identity is checked before the Master Password is asked for, so a Vault
// whose keys have gone missing fails without a pointless prompt.
func TestAMissingIdentityIsRefusedBeforeAnyPrompt(t *testing.T) {
	cfg := initialised(t)
	if err := os.Remove(cfg.PrivateKeyPath); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	answers(t) // a prompt here would be the bug
	_, err := gopm(t, "export-keys", "-o", filepath.Join(t.TempDir(), "k.zip"))
	if err == nil || !strings.Contains(err.Error(), cfg.PrivateKeyPath) {
		t.Fatalf("err = %v, want a refusal naming the identity", err)
	}
}
