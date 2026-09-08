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

// FS adalah Vault yang menyimpan Secret sebagai file di satu folder.
type FS struct {
	dir    string
	cipher Cipher
}

// Open menunjuk Vault ke sebuah folder. Folder harus sudah ada — folder yang
// salah ketik lebih baik ditolak daripada menjadi Vault kosong yang baru.
func Open(dir string, cipher Cipher) (*FS, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("path vault tidak sah: %w", err)
	}
	info, err := os.Stat(abs)
	if err != nil {
		return nil, fmt.Errorf("folder vault tidak dapat dibuka: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%s bukan folder", abs)
	}
	return &FS{dir: abs, cipher: cipher}, nil
}

func (v *FS) Dir() string { return v.dir }

func (v *FS) List() ([]string, error) {
	entries, err := os.ReadDir(v.dir)
	if err != nil {
		return nil, fmt.Errorf("isi vault tidak terbaca: %w", err)
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
		return nil, fmt.Errorf("gagal membaca %s: %w", slug, err)
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
	// Tulis isi baru lebih dulu; file lama baru dihapus setelah penulisan
	// berhasil, sehingga kegagalan di tengah tidak pernah menghilangkan Secret.
	if err := writeAtomic(v.path(slug), data); err != nil {
		return "", err
	}
	if slug != old {
		if err := os.Remove(v.path(old)); err != nil {
			return slug, fmt.Errorf("secret tersimpan sebagai %s, tetapi file lama %s gagal dihapus: %w", slug, old, err)
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
		return fmt.Errorf("gagal menghapus %s: %w", slug, err)
	}
	return nil
}

// prepare memvalidasi, menyerialisasi, dan mengenkripsi Secret sekaligus
// menghitung slug tujuannya.
func (v *FS) prepare(s *secret.Secret) (string, []byte, error) {
	if err := s.Validate(); err != nil {
		return "", nil, err
	}
	slug := Slug(s.Meta.Title)
	if slug == "" {
		return "", nil, fmt.Errorf("judul %q tidak menghasilkan nama file; pakai huruf atau angka", s.Meta.Title)
	}
	plain, err := secret.Marshal(s)
	if err != nil {
		return "", nil, fmt.Errorf("gagal menyiapkan secret: %w", err)
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

// writeAtomic menulis ke file sementara di folder yang sama, mem-fsync-nya,
// lalu rename ke tujuan. Rename bersifat atomik di POSIX, sehingga pembaca
// tidak pernah melihat Secret yang setengah tertulis.
func writeAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".gopm-*.tmp")
	if err != nil {
		return fmt.Errorf("gagal menyiapkan file sementara: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // tidak berefek jika rename sudah berhasil

	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return fmt.Errorf("gagal mengatur izin file: %w", err)
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("gagal menulis secret: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return fmt.Errorf("gagal menulis secret ke disk: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("gagal menutup file sementara: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("gagal memindahkan secret ke tempatnya: %w", err)
	}
	return nil
}
