package vault

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/fbriansyah/go-password-manager/internal/secret"
)

// FS is a Vault that stores Secrets as files in a single folder.
type FS struct {
	dir    string
	cipher Cipher
}

// Open points a Vault at a folder. The folder must already exist — a mistyped
// path is better refused than turned into a new, empty Vault.
func Open(dir string, cipher Cipher) (*FS, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("invalid vault path: %w", err)
	}
	info, err := os.Stat(abs)
	if err != nil {
		return nil, fmt.Errorf("vault folder cannot be opened: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%s is not a folder", abs)
	}
	return &FS{dir: abs, cipher: cipher}, nil
}

func (v *FS) Dir() string { return v.dir }

func (v *FS) List() ([]string, error) {
	entries, err := os.ReadDir(v.dir)
	if err != nil {
		return nil, fmt.Errorf("vault contents are unreadable: %w", err)
	}
	var slugs []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), Ext) {
			continue
		}
		slugs = append(slugs, strings.TrimSuffix(e.Name(), Ext))
	}
	sort.Strings(slugs)
	return slugs, nil
}

func (v *FS) Load(slug string) (*secret.Secret, error) {
	data, err := os.ReadFile(v.path(slug))
	if errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("%w: %s", ErrNotFound, slug)
	}
	if err != nil {
		return nil, fmt.Errorf("could not read %s: %w", slug, err)
	}
	plain, err := v.cipher.Decrypt(data)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", slug, err)
	}
	return secret.Unmarshal(plain)
}

func (v *FS) Create(s *secret.Secret) (string, error) {
	slug, data, err := v.prepare(s)
	if err != nil {
		return "", err
	}
	if v.exists(slug) {
		return "", fmt.Errorf("%w: %s", ErrSlugTaken, slug)
	}
	if err := writeAtomic(v.path(slug), data); err != nil {
		return "", err
	}
	return slug, nil
}

func (v *FS) Save(old string, s *secret.Secret) (string, error) {
	if !v.exists(old) {
		return "", fmt.Errorf("%w: %s", ErrNotFound, old)
	}
	slug, data, err := v.prepare(s)
	if err != nil {
		return "", err
	}
	if slug != old && v.exists(slug) {
		return "", fmt.Errorf("%w: %s", ErrSlugTaken, slug)
	}
	// Write the new contents first; the old file is removed only once that
	// succeeded, so a failure midway never loses a Secret.
	if err := writeAtomic(v.path(slug), data); err != nil {
		return "", err
	}
	if slug != old {
		if err := os.Remove(v.path(old)); err != nil {
			return slug, fmt.Errorf("secret saved as %s, but the old file %s could not be removed: %w", slug, old, err)
		}
	}
	return slug, nil
}

func (v *FS) Delete(slug string) error {
	err := os.Remove(v.path(slug))
	if errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("%w: %s", ErrNotFound, slug)
	}
	if err != nil {
		return fmt.Errorf("could not delete %s: %w", slug, err)
	}
	return nil
}

// prepare validates, serialises, and encrypts a Secret, and works out the slug
// it should be stored under.
func (v *FS) prepare(s *secret.Secret) (string, []byte, error) {
	if err := s.Validate(); err != nil {
		return "", nil, err
	}
	slug := Slug(s.Meta.Title)
	if slug == "" {
		return "", nil, fmt.Errorf("the title %q produces no file name; use letters or digits", s.Meta.Title)
	}
	plain, err := secret.Marshal(s)
	if err != nil {
		return "", nil, fmt.Errorf("could not prepare the secret: %w", err)
	}
	data, err := v.cipher.Encrypt(plain)
	if err != nil {
		return "", nil, err
	}
	return slug, data, nil
}

func (v *FS) path(slug string) string { return filepath.Join(v.dir, slug+Ext) }

func (v *FS) exists(slug string) bool {
	_, err := os.Stat(v.path(slug))
	return err == nil
}

// writeAtomic writes to a temporary file in the same folder, fsyncs it, then
// renames it into place. Rename is atomic on POSIX, so a reader never sees a
// half-written Secret.
func writeAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".gopm-*.tmp")
	if err != nil {
		return fmt.Errorf("could not prepare the temporary file: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // a no-op once the rename succeeded

	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return fmt.Errorf("could not set the file permissions: %w", err)
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("could not write the secret: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return fmt.Errorf("could not flush the secret to disk: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("could not close the temporary file: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("could not move the secret into place: %w", err)
	}
	return nil
}
