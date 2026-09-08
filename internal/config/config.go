// Package config membaca konfigurasi YAML: satu file global, ditimpa oleh file
// opsional di dalam folder Vault.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
)

// Nama file konfigurasi. Override dicari di folder Vault yang terpilih, bukan
// di $PWD, sehingga key selalu mengikuti Secret yang dibuka (docs/adr/0001).
const (
	GlobalName   = "config.yaml"
	OverrideName = ".gopm.yaml"
)

// Config menunjuk ke Identity dan Recipient yang dipakai sesi ini.
type Config struct {
	// PrivateKeyPath adalah lokasi Identity (private key terenkripsi Master Password).
	PrivateKeyPath string
	// PublicKeyPath adalah lokasi Recipient (public key).
	PublicKeyPath string
	// Source menyebutkan file mana yang terakhir mengisi nilai di atas.
	Source string
}

// ErrNotConfigured dikembalikan saat belum ada konfigurasi sama sekali.
var ErrNotConfigured = errors.New("belum ada konfigurasi; jalankan `gopm init` lebih dulu")

// Dir mengembalikan folder konfigurasi global.
func Dir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("folder konfigurasi tidak diketahui: %w", err)
	}
	return filepath.Join(base, "gopm"), nil
}

// Defaults menghasilkan konfigurasi bawaan untuk `gopm init`.
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

// Load membaca konfigurasi global lalu menimpanya dengan .gopm.yaml di
// vaultDir bila ada.
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
		return Config{}, fmt.Errorf("konfigurasi di %s tidak lengkap: PRIVATE_KEY_PATH dan PUBLIC_KEY_PATH keduanya wajib", cfg.Source)
	}
	return cfg, nil
}

// Write menulis konfigurasi ke path dalam format yang dijelaskan Design.md.
func Write(path string, cfg Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("gagal membuat folder konfigurasi: %w", err)
	}
	body := fmt.Sprintf("PUBLIC_KEY_PATH: %q\nPRIVATE_KEY_PATH: %q\n", cfg.PublicKeyPath, cfg.PrivateKeyPath)
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		return fmt.Errorf("gagal menulis %s: %w", path, err)
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
		return fmt.Errorf("gagal membaca %s: %w", path, err)
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

// expand menerjemahkan ~ dan variabel lingkungan supaya config bisa ditulis
// tangan tanpa path absolut.
func expand(s string) string {
	s = strings.TrimSpace(os.ExpandEnv(s))
	if s == "~" || strings.HasPrefix(s, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, strings.TrimPrefix(s, "~"))
		}
	}
	return s
}
