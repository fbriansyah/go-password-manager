// Package config reads the YAML configuration: one global file, overridden by
// an optional file inside the Vault folder.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
)

// Configuration file names. The override is looked for in the selected Vault
// folder, not in $PWD, so the keys always follow the Secrets (docs/adr/0001).
const (
	GlobalName   = "config.yaml"
	OverrideName = ".gopm.yaml"
)

// Config points at the Identity and Recipient this session uses.
type Config struct {
	// PrivateKeyPath is where the Identity lives (private key encrypted with the Master Password).
	PrivateKeyPath string
	// PublicKeyPath is where the Recipient lives (public key).
	PublicKeyPath string
	// Source names the file that last filled in the values above.
	Source string
}

// ErrNotConfigured is returned when there is no configuration at all yet.
var ErrNotConfigured = errors.New("no configuration yet; run `gopm init` first")

// Dir returns the global configuration folder.
func Dir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("configuration folder is unknown: %w", err)
	}
	return filepath.Join(base, "gopm"), nil
}

// Defaults produces the configuration `gopm init` starts from.
func Defaults() (Config, string, error) {
	dir, err := Dir()
	if err != nil {
		return Config{}, "", err
	}
	return Config{
		PrivateKeyPath: filepath.Join(dir, "identity.age"),
		PublicKeyPath:  filepath.Join(dir, "recipient.pub"),
	}, filepath.Join(dir, GlobalName), nil
}

// Load reads the global configuration, then overlays .gopm.yaml from vaultDir
// when that file exists.
func Load(vaultDir string) (Config, error) {
	dir, err := Dir()
	if err != nil {
		return Config{}, err
	}
	var cfg Config
	globalPath := filepath.Join(dir, GlobalName)
	found := false

	if err := merge(globalPath, &cfg); err == nil {
		found = true
	} else if !errors.Is(err, os.ErrNotExist) {
		return Config{}, err
	}

	overridePath := filepath.Join(vaultDir, OverrideName)
	if err := merge(overridePath, &cfg); err == nil {
		found = true
	} else if !errors.Is(err, os.ErrNotExist) {
		return Config{}, err
	}

	if !found {
		return Config{}, ErrNotConfigured
	}
	if cfg.PrivateKeyPath == "" || cfg.PublicKeyPath == "" {
		return Config{}, fmt.Errorf("configuration in %s is incomplete: PRIVATE_KEY_PATH and PUBLIC_KEY_PATH are both required", cfg.Source)
	}
	return cfg, nil
}

// Write writes the configuration to path in the documented key format.
func Write(path string, cfg Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("could not create the configuration folder: %w", err)
	}
	body := fmt.Sprintf("PUBLIC_KEY_PATH: %q\nPRIVATE_KEY_PATH: %q\n", cfg.PublicKeyPath, cfg.PrivateKeyPath)
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		return fmt.Errorf("could not write %s: %w", path, err)
	}
	return nil
}

func merge(path string, cfg *Config) error {
	if _, err := os.Stat(path); err != nil {
		return err
	}
	v := viper.New()
	v.SetConfigFile(path)
	if err := v.ReadInConfig(); err != nil {
		return fmt.Errorf("could not read %s: %w", path, err)
	}
	if s := expand(v.GetString("PRIVATE_KEY_PATH")); s != "" {
		cfg.PrivateKeyPath = s
	}
	if s := expand(v.GetString("PUBLIC_KEY_PATH")); s != "" {
		cfg.PublicKeyPath = s
	}
	cfg.Source = path
	return nil
}

// expand resolves ~ and environment variables so a config can be written by
// hand without spelling out absolute paths.
func expand(s string) string {
	s = strings.TrimSpace(os.ExpandEnv(s))
	if s == "~" || strings.HasPrefix(s, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, strings.TrimPrefix(s, "~"))
		}
	}
	return s
}
