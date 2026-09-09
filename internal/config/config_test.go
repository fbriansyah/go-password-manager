package config_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fbriansyah/go-password-manager/internal/config"
	"github.com/fbriansyah/go-password-manager/internal/generator"
)

func TestVaultOverrideBeatsGlobal(t *testing.T) {
	confHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", confHome)
	global := filepath.Join(confHome, "gopm", "config.yaml")
	if err := config.Write(global, config.Config{PrivateKeyPath: "/global/id.age", PublicKeyPath: "/global/rec.pub"}); err != nil {
		t.Fatalf("Write: %v", err)
	}

	vaultDir := t.TempDir()
	cfg, err := config.Load(vaultDir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.PrivateKeyPath != "/global/id.age" {
		t.Fatalf("without an override, PrivateKeyPath = %q", cfg.PrivateKeyPath)
	}

	override := filepath.Join(vaultDir, config.OverrideName)
	if err := os.WriteFile(override, []byte("PRIVATE_KEY_PATH: /klien/id.age\n"), 0o600); err != nil {
		t.Fatalf("tulis override: %v", err)
	}
	cfg, err = config.Load(vaultDir)
	if err != nil {
		t.Fatalf("Load with an override: %v", err)
	}
	if cfg.PrivateKeyPath != "/klien/id.age" {
		t.Fatalf("PrivateKeyPath = %q, want the value from the override", cfg.PrivateKeyPath)
	}
	// Values the override does not mention still come from the global file.
	if cfg.PublicKeyPath != "/global/rec.pub" {
		t.Fatalf("PublicKeyPath = %q, want it kept from the global file", cfg.PublicKeyPath)
	}
}

func TestLoadWithoutAnyConfiguration(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if _, err := config.Load(t.TempDir()); !errors.Is(err, config.ErrNotConfigured) {
		t.Fatalf("err = %v, want ErrNotConfigured", err)
	}
}

func TestTildeAndEnvAreExpanded(t *testing.T) {
	confHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", confHome)
	t.Setenv("KEYDIR", "/keys")
	global := filepath.Join(confHome, "gopm", "config.yaml")
	body := "PRIVATE_KEY_PATH: ~/id.age\nPUBLIC_KEY_PATH: ${KEYDIR}/rec.pub\n"
	if err := os.MkdirAll(filepath.Dir(global), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(global, []byte(body), 0o600); err != nil {
		t.Fatalf("tulis: %v", err)
	}
	cfg, err := config.Load(t.TempDir())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	home, _ := os.UserHomeDir()
	if cfg.PrivateKeyPath != filepath.Join(home, "id.age") {
		t.Fatalf("~ was not expanded: %q", cfg.PrivateKeyPath)
	}
	if cfg.PublicKeyPath != "/keys/rec.pub" {
		t.Fatalf("env was not expanded: %q", cfg.PublicKeyPath)
	}
}

// A file that says nothing about the Generator Policy inherits the built-in
// default (docs/adr/0005) — no configuration is the common case, and it must
// still leave ctrl+g usable.
func TestLoadWithoutAGeneratorPolicyUsesTheDefault(t *testing.T) {
	confHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", confHome)
	global := filepath.Join(confHome, "gopm", "config.yaml")
	if err := config.Write(global, config.Config{PrivateKeyPath: "/id.age", PublicKeyPath: "/rec.pub"}); err != nil {
		t.Fatalf("Write: %v", err)
	}
	cfg, err := config.Load(t.TempDir())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Generator != generator.Default() {
		t.Fatalf("Generator = %+v, want the built-in default", cfg.Generator)
	}
}

// The merge rule for the Generator Policy is per key, not per file: a Vault's
// .gopm.yaml overriding only GENERATOR_SYMBOLS must not reset the length that
// the global file set.
func TestGeneratorPolicyMergesPerKey(t *testing.T) {
	confHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", confHome)
	global := filepath.Join(confHome, "gopm", "config.yaml")
	body := "PRIVATE_KEY_PATH: /id.age\nPUBLIC_KEY_PATH: /rec.pub\nGENERATOR_LENGTH: 32\n"
	if err := os.MkdirAll(filepath.Dir(global), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(global, []byte(body), 0o600); err != nil {
		t.Fatalf("write global: %v", err)
	}

	vaultDir := t.TempDir()
	override := filepath.Join(vaultDir, config.OverrideName)
	if err := os.WriteFile(override, []byte("GENERATOR_SYMBOLS: false\n"), 0o600); err != nil {
		t.Fatalf("write override: %v", err)
	}

	cfg, err := config.Load(vaultDir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Generator.Length != 32 {
		t.Fatalf("Length = %d, want 32 inherited from the global file", cfg.Generator.Length)
	}
	if cfg.Generator.Symbols {
		t.Fatal("Symbols = true, want false from the Vault override")
	}
	if !cfg.Generator.Upper || !cfg.Generator.Digits {
		t.Fatalf("Upper/Digits should still be the built-in default: %+v", cfg.Generator)
	}
}

// GENERATOR_SYMBOLS: false must be honoured, not confused with the key being
// absent — the whole point of reading it through IsSet (docs/adr/0005).
func TestGeneratorSymbolsFalseIsHonoured(t *testing.T) {
	confHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", confHome)
	global := filepath.Join(confHome, "gopm", "config.yaml")
	body := "PRIVATE_KEY_PATH: /id.age\nPUBLIC_KEY_PATH: /rec.pub\nGENERATOR_SYMBOLS: false\n"
	if err := os.MkdirAll(filepath.Dir(global), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(global, []byte(body), 0o600); err != nil {
		t.Fatalf("write global: %v", err)
	}
	cfg, err := config.Load(t.TempDir())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Generator.Symbols {
		t.Fatal("Symbols = true, want false as written in the file")
	}
}

// A bad length in a file is not a startup error: it loads, and only fails
// later when Generate is actually asked to use it (docs/milestone-3.md).
func TestLoadAcceptsAnUnsafeGeneratedLength(t *testing.T) {
	confHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", confHome)
	global := filepath.Join(confHome, "gopm", "config.yaml")
	body := "PRIVATE_KEY_PATH: /id.age\nPUBLIC_KEY_PATH: /rec.pub\nGENERATOR_LENGTH: 4\n"
	if err := os.MkdirAll(filepath.Dir(global), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(global, []byte(body), 0o600); err != nil {
		t.Fatalf("write global: %v", err)
	}
	cfg, err := config.Load(t.TempDir())
	if err != nil {
		t.Fatalf("Load must not fail on a short length: %v", err)
	}
	if cfg.Generator.Length != 4 {
		t.Fatalf("Length = %d, want the file's own value, unclamped", cfg.Generator.Length)
	}
	if _, err := generator.Generate(cfg.Generator); !errors.Is(err, generator.ErrTooShort) {
		t.Fatalf("Generate err = %v, want ErrTooShort", err)
	}
}

// UpdateGenerator edits a file in place: existing keys move where they stand,
// comments and unrelated keys are left alone, and no key is duplicated.
func TestUpdateGeneratorEditsInPlace(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	body := "# office keys, do not touch\nPRIVATE_KEY_PATH: \"/id.age\"\nPUBLIC_KEY_PATH: \"/rec.pub\"\nGENERATOR_LENGTH: 20\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	if err := config.UpdateGenerator(path, generator.Options{Length: 32, Upper: true, Digits: false, Symbols: false}); err != nil {
		t.Fatalf("UpdateGenerator: %v", err)
	}
	first, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	text := string(first)
	if !strings.Contains(text, "# office keys, do not touch") {
		t.Fatal("the comment was lost")
	}
	if !strings.Contains(text, `PRIVATE_KEY_PATH: "/id.age"`) {
		t.Fatal("an unrelated key was lost")
	}
	if strings.Count(text, "GENERATOR_LENGTH:") != 1 || !strings.Contains(text, "GENERATOR_LENGTH: 32") {
		t.Fatalf("GENERATOR_LENGTH was not updated in place:\n%s", text)
	}
	if !strings.Contains(text, "GENERATOR_DIGITS: false") || !strings.Contains(text, "GENERATOR_SYMBOLS: false") {
		t.Fatalf("a missing key was not appended:\n%s", text)
	}

	// Saving again with the same values must not duplicate any key.
	if err := config.UpdateGenerator(path, generator.Options{Length: 32, Upper: true, Digits: false, Symbols: false}); err != nil {
		t.Fatalf("UpdateGenerator (again): %v", err)
	}
	second, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if strings.Count(string(second), "GENERATOR_LENGTH:") != 1 {
		t.Fatalf("a repeated save duplicated a key:\n%s", second)
	}
}

// UpdateGenerator must create the file when it does not exist yet — the
// panel's ctrl+s is not guaranteed to run after `gopm init`.
func TestUpdateGeneratorCreatesAMissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "config.yaml")
	if err := config.UpdateGenerator(path, generator.Default()); err != nil {
		t.Fatalf("UpdateGenerator: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("file was not created: %v", err)
	}
}
