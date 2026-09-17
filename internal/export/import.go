package export

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/fbriansyah/go-password-manager/internal/config"
	"github.com/fbriansyah/go-password-manager/internal/crypto"
)

// ErrKeypairExists is returned when Import finds a keypair already at cfg's
// paths. Import never overwrites one: the caller removes it deliberately
// first (docs/adr/0010).
var ErrKeypairExists = errors.New("a keypair already exists at the destination")

// Import installs the Identity and Recipient carried by zipData at cfg's
// paths, verifying at every step before anything is written: the manifest's
// format, the Identity's checksum, the Master Password, and finally that the
// Recipient actually pairs with the Identity. Any failure after a file has
// been written removes it again, so a failed Import never leaves a half
// installed keypair behind (docs/adr/0010).
func Import(zipData []byte, cfg config.Config, masterPassword string) error {
	for _, p := range []string{cfg.PrivateKeyPath, cfg.PublicKeyPath} {
		if _, err := os.Stat(p); err == nil {
			return ErrKeypairExists
		} else if !os.IsNotExist(err) {
			return fmt.Errorf("could not check %s: %w", p, err)
		}
	}

	zr, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
	if err != nil {
		return fmt.Errorf("not a valid zip archive: %w", err)
	}
	identity, err := readEntry(zr, "identity.age")
	if err != nil {
		return err
	}
	recipient, err := readEntry(zr, "recipient.pub")
	if err != nil {
		return err
	}
	manifestJSON, err := readEntry(zr, "manifest.json")
	if err != nil {
		return err
	}

	var m manifest
	if err := json.Unmarshal(manifestJSON, &m); err != nil {
		return fmt.Errorf("manifest.json is invalid: %w", err)
	}
	if m.FormatVersion != FormatVersion {
		return fmt.Errorf("this archive is format version %d, this build of gopm only understands %d", m.FormatVersion, FormatVersion)
	}
	sum := sha256.Sum256(identity)
	if hex.EncodeToString(sum[:]) != m.IdentitySHA256 {
		return errors.New("identity.age does not match the checksum in manifest.json; the archive may be corrupt")
	}

	if err := os.MkdirAll(filepath.Dir(cfg.PrivateKeyPath), 0o700); err != nil {
		return fmt.Errorf("could not create the configuration folder: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(cfg.PrivateKeyPath), ".identity-import-*")
	if err != nil {
		return fmt.Errorf("could not create a temporary file: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if _, err := tmp.Write(identity); err != nil {
		tmp.Close()
		return fmt.Errorf("could not write a temporary file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("could not write a temporary file: %w", err)
	}

	session, err := crypto.Unlock(tmpPath, masterPassword)
	if err != nil {
		return err
	}
	if strings.TrimSpace(string(recipient)) != session.RecipientString() {
		return errors.New("recipient.pub does not match identity.age; they are not the same keypair")
	}

	if err := os.Rename(tmpPath, cfg.PrivateKeyPath); err != nil {
		return fmt.Errorf("could not write %s: %w", cfg.PrivateKeyPath, err)
	}
	if err := os.WriteFile(cfg.PublicKeyPath, recipient, 0o644); err != nil {
		os.Remove(cfg.PrivateKeyPath)
		return fmt.Errorf("could not write %s: %w", cfg.PublicKeyPath, err)
	}
	return nil
}

func readEntry(zr *zip.Reader, name string) ([]byte, error) {
	for _, f := range zr.File {
		if f.Name != name {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return nil, fmt.Errorf("could not read %s from the archive: %w", name, err)
		}
		defer rc.Close()
		var buf bytes.Buffer
		if _, err := buf.ReadFrom(rc); err != nil {
			return nil, fmt.Errorf("could not read %s from the archive: %w", name, err)
		}
		return buf.Bytes(), nil
	}
	return nil, fmt.Errorf("the archive does not contain %s", name)
}
