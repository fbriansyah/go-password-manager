// Package export bundles the Identity and the Recipient into a zip so they
// can be moved to another drive. It adds no cryptography of its own — the
// Identity is already encrypted with the Master Password, and the zip is a
// container, not a second vault (docs/adr/0009).
package export

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/fbriansyah/go-password-manager/internal/config"
	"github.com/fbriansyah/go-password-manager/internal/crypto"
)

// FormatVersion identifies the layout of the zip this package writes, so a
// future import can tell exports apart as the layout evolves.
const FormatVersion = 1

// manifest is written into the zip as manifest.json.
type manifest struct {
	FormatVersion  int    `json:"format_version"`
	App            string `json:"app"`
	ExportedAt     string `json:"exported_at"`
	IdentitySHA256 string `json:"identity_sha256"`
}

// Now is time.Now, replaced in tests so ExportedAt is deterministic.
var Now = time.Now

// Keys verifies masterPassword opens the Identity at cfg.PrivateKeyPath, then
// writes a zip to w containing the Identity, the Recipient, and a manifest.
// Verifying first means a bad export is caught now rather than the day it is
// needed for recovery.
func Keys(cfg config.Config, masterPassword string, w io.Writer) error {
	if _, err := crypto.Unlock(cfg.PrivateKeyPath, masterPassword); err != nil {
		return err
	}

	identity, err := os.ReadFile(cfg.PrivateKeyPath)
	if err != nil {
		return fmt.Errorf("identity is unreadable: %w", err)
	}
	recipient, err := os.ReadFile(cfg.PublicKeyPath)
	if err != nil {
		return fmt.Errorf("recipient is unreadable: %w", err)
	}
	sum := sha256.Sum256(identity)

	m := manifest{
		FormatVersion:  FormatVersion,
		App:            "gopm",
		ExportedAt:     Now().UTC().Format(time.RFC3339),
		IdentitySHA256: hex.EncodeToString(sum[:]),
	}
	manifestJSON, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("could not build manifest: %w", err)
	}

	zw := zip.NewWriter(w)
	for _, f := range []struct {
		name string
		data []byte
	}{
		{"identity.age", identity},
		{"recipient.pub", recipient},
		{"manifest.json", manifestJSON},
	} {
		entry, err := zw.Create(f.name)
		if err != nil {
			return fmt.Errorf("could not add %s to the archive: %w", f.name, err)
		}
		if _, err := entry.Write(f.data); err != nil {
			return fmt.Errorf("could not write %s to the archive: %w", f.name, err)
		}
	}
	if err := zw.Close(); err != nil {
		return fmt.Errorf("could not finish the archive: %w", err)
	}
	return nil
}
