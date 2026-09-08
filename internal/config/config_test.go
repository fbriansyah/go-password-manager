package config_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/fbriansyah/go-password-manager/internal/config"
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
