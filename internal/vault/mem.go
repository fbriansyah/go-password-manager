package vault

import (
	"fmt"
	"sort"

	"github.com/fbriansyah/go-password-manager/internal/secret"
)

// Mem is an in-memory Vault with the same rules as FS. Used by the tests and
// as a reference for the expected behaviour.
type Mem struct {
	cipher Cipher
	data   map[string][]byte
}

// NewMem creates an empty in-memory Vault.
func NewMem(cipher Cipher) *Mem {
	return &Mem{cipher: cipher, data: map[string][]byte{}}
}

func (v *Mem) Dir() string { return "(memori)" }

func (v *Mem) List() ([]string, error) {
	slugs := make([]string, 0, len(v.data))
	for slug := range v.data {
		slugs = append(slugs, slug)
	}
	sort.Strings(slugs)
	return slugs, nil
}

func (v *Mem) Load(slug string) (*secret.Secret, error) {
	data, ok := v.data[slug]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrNotFound, slug)
	}
	plain, err := v.cipher.Decrypt(data)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", slug, err)
	}
	return secret.Unmarshal(plain)
}

func (v *Mem) Create(s *secret.Secret) (string, error) {
	slug, data, err := v.prepare(s)
	if err != nil {
		return "", err
	}
	if _, exists := v.data[slug]; exists {
		return "", fmt.Errorf("%w: %s", ErrSlugTaken, slug)
	}
	v.data[slug] = data
	return slug, nil
}

func (v *Mem) Save(old string, s *secret.Secret) (string, error) {
	if _, exists := v.data[old]; !exists {
		return "", fmt.Errorf("%w: %s", ErrNotFound, old)
	}
	slug, data, err := v.prepare(s)
	if err != nil {
		return "", err
	}
	if slug != old {
		if _, exists := v.data[slug]; exists {
			return "", fmt.Errorf("%w: %s", ErrSlugTaken, slug)
		}
		delete(v.data, old)
	}
	v.data[slug] = data
	return slug, nil
}

func (v *Mem) Delete(slug string) error {
	if _, exists := v.data[slug]; !exists {
		return fmt.Errorf("%w: %s", ErrNotFound, slug)
	}
	delete(v.data, slug)
	return nil
}

func (v *Mem) prepare(s *secret.Secret) (string, []byte, error) {
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
