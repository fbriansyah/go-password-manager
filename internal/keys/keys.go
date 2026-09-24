// Package keys resolves a Location: the Vault folder a run is pointed at
// together with the Identity and Recipient that open it (CONTEXT.md:
// Location). It is the single place that decides which keys a folder uses, so
// the CLI and the TUI reach a Vault by the same route.
package keys

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/fbriansyah/go-password-manager/internal/config"
	"github.com/fbriansyah/go-password-manager/internal/crypto"
	"github.com/fbriansyah/go-password-manager/internal/vault"
)

// Location is a Vault folder and the keys that open it, resolved before any
// Master Password has been given. It knows where the keys are, never what
// they hold; Unlock is what turns one into a readable Vault.
type Location struct {
	// Dir is the Vault folder: absolute, and known to exist.
	Dir string
	// Config points at the Identity and Recipient this run uses.
	Config config.Config
	// ConfigPath is where the configuration lives, or where it would be
	// written when there is none yet.
	ConfigPath string
	// Configured reports whether a file supplied Config. False means these are
	// the defaults `gopm init` would have chosen.
	Configured bool
}

// Locate resolves the Vault folder dirFlag names — the working directory when
// it is empty — and reads the configuration that applies to it. A folder with
// no configuration at all is refused; LocateOrDefaults is the way past that.
func Locate(dirFlag string) (Location, error) {
	loc, err := LocateOrDefaults(dirFlag)
	if err != nil {
		return Location{}, err
	}
	if !loc.Configured {
		return Location{}, config.ErrNotConfigured
	}
	return loc, nil
}

// LocateOrDefaults is Locate for a command that may run before there is any
// configuration: `import-keys` installs the very keypair a configuration would
// point at, so it falls back to the defaults `gopm init` would choose and
// reports through Configured which of the two it got.
//
// The .gopm.yaml override is looked for in the resolved Vault folder rather
// than in $PWD, so the keys always follow the Secrets (docs/adr/0001).
func LocateOrDefaults(dirFlag string) (Location, error) {
	dir, err := resolveDir(dirFlag)
	if err != nil {
		return Location{}, err
	}
	cfg, err := config.Load(dir)
	if err == nil {
		return Location{Dir: dir, Config: cfg, ConfigPath: cfg.Source, Configured: true}, nil
	}
	if !errors.Is(err, config.ErrNotConfigured) {
		return Location{}, err
	}
	cfg, path, err := config.Defaults()
	if err != nil {
		return Location{}, err
	}
	return Location{Dir: dir, Config: cfg, ConfigPath: path}, nil
}

// RequireIdentity refuses a Location whose Identity is not there, so a command
// fails before asking for a Master Password it would have nothing to use on.
func (l Location) RequireIdentity() error {
	if _, err := os.Stat(l.Config.PrivateKeyPath); err != nil {
		return fmt.Errorf("identity at %s cannot be opened: %w", l.Config.PrivateKeyPath, err)
	}
	return nil
}

// Identity opens the Identity with the Master Password. Reading a Secret needs
// both; writing one does not (docs/adr/0003).
func (l Location) Identity(password string) (*crypto.Session, error) {
	return crypto.Unlock(l.Config.PrivateKeyPath, password)
}

// Recipient builds a write-only session from the Recipient alone: enough to
// create Secrets, and to list the slugs already taken, with no proof of
// ownership at all (docs/adr/0003, docs/adr/0012).
func (l Location) Recipient() (*crypto.Session, error) {
	return crypto.RecipientOnly(l.Config.PublicKeyPath)
}

// Vault opens the Vault at this Location, reading and writing through session.
func (l Location) Vault(session *crypto.Session) (vault.Vault, error) {
	v, err := vault.Open(l.Dir, session)
	if err != nil {
		return nil, err
	}
	return v, nil
}

// Unlock is the act CONTEXT.md names: the Master Password opens the Identity,
// and every Secret in the Vault becomes readable.
func (l Location) Unlock(password string) (vault.Vault, error) {
	session, err := l.Identity(password)
	if err != nil {
		return nil, err
	}
	return l.Vault(session)
}

// resolveDir decides the Vault folder: dirFlag when it is given, otherwise the
// working directory. A folder that does not exist is refused rather than
// silently created — a mistyped path is better failing than becoming a new,
// empty Vault (docs/adr/0001).
func resolveDir(dirFlag string) (string, error) {
	dir := dirFlag
	if dir == "" {
		wd, err := os.Getwd()
		if err != nil {
			return "", fmt.Errorf("working directory is unknown: %w", err)
		}
		dir = wd
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", fmt.Errorf("invalid vault path: %w", err)
	}
	info, err := os.Stat(abs)
	if errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("vault folder %s does not exist", abs)
	}
	if err != nil {
		return "", fmt.Errorf("vault folder cannot be opened: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("%s is not a folder", abs)
	}
	return abs, nil
}
