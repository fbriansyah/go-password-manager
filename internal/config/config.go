// Package config reads the YAML configuration: one global file, overridden by
// an optional file inside the Vault folder.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/spf13/viper"

	"github.com/fbriansyah/go-password-manager/internal/generator"
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
	// Generator is the Generator Policy this session starts from (docs/adr/0005).
	Generator generator.Options
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
	cfg := Config{Generator: generator.Default()}
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

// Generator Policy keys. Flat and SCREAMING_SNAKE, like the two key paths
// above them, so one merge rule covers the whole file (docs/adr/0005).
const (
	keyGeneratorLength  = "GENERATOR_LENGTH"
	keyGeneratorUpper   = "GENERATOR_UPPER"
	keyGeneratorDigits  = "GENERATOR_DIGITS"
	keyGeneratorSymbols = "GENERATOR_SYMBOLS"
)

// UpdateGenerator saves a Generator Policy into the configuration at path, in
// place: a line for a key that is already there is replaced where it stands,
// a key that is missing is appended, and everything else in the file —
// comments, blank lines, keys this package does not know about — is left
// untouched. The file is created if it does not exist yet.
func UpdateGenerator(path string, o generator.Options) error {
	return updateLines(path, []line{
		{keyGeneratorLength, strconv.Itoa(o.Length)},
		{keyGeneratorUpper, strconv.FormatBool(o.Upper)},
		{keyGeneratorDigits, strconv.FormatBool(o.Digits)},
		{keyGeneratorSymbols, strconv.FormatBool(o.Symbols)},
	})
}

// line is one key/value pair to place in a configuration file. The value is
// written verbatim, so it must already be the text the file should show.
type line struct {
	key   string
	value string
}

func updateLines(path string, lines []line) error {
	var body []string
	if raw, err := os.ReadFile(path); err == nil {
		body = strings.Split(strings.TrimRight(string(raw), "\n"), "\n")
		if len(body) == 1 && body[0] == "" {
			body = nil
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("could not read %s: %w", path, err)
	}

	for _, l := range lines {
		rendered := fmt.Sprintf("%s: %s", l.key, l.value)
		replaced := false
		for i, existing := range body {
			if lineKey(existing) == l.key {
				body[i] = rendered
				replaced = true
				break
			}
		}
		if !replaced {
			body = append(body, rendered)
		}
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("could not create the configuration folder: %w", err)
	}
	if err := os.WriteFile(path, []byte(strings.Join(body, "\n")+"\n"), 0o600); err != nil {
		return fmt.Errorf("could not write %s: %w", path, err)
	}
	return nil
}

// lineKey extracts the key from one line of the file, so updateLines can find
// where a key already lives without parsing the file as YAML.
func lineKey(l string) string {
	i := strings.Index(l, ":")
	if i < 0 {
		return ""
	}
	return strings.TrimSpace(l[:i])
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
	// A boolean read from an absent key is indistinguishable from one written
	// as false, so absence must be tested with IsSet rather than a zero check
	// (docs/adr/0005).
	if v.IsSet(keyGeneratorLength) {
		cfg.Generator.Length = v.GetInt(keyGeneratorLength)
	}
	if v.IsSet(keyGeneratorUpper) {
		cfg.Generator.Upper = v.GetBool(keyGeneratorUpper)
	}
	if v.IsSet(keyGeneratorDigits) {
		cfg.Generator.Digits = v.GetBool(keyGeneratorDigits)
	}
	if v.IsSet(keyGeneratorSymbols) {
		cfg.Generator.Symbols = v.GetBool(keyGeneratorSymbols)
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
