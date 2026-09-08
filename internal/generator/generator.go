// Package generator membuat password acak untuk mengisi Field bertipe ps.
package generator

import (
	"crypto/rand"
	"errors"
	"math/big"
	"strings"
)

// Kelas karakter yang bisa dipilih.
const (
	Lower   = "abcdefghijkmnopqrstuvwxyz"
	Upper   = "ABCDEFGHJKLMNPQRSTUVWXYZ"
	Digits  = "23456789"
	Symbols = "!@#$%^&*()-_=+[]{};:,.?"
)

// Options mengatur bentuk password yang dihasilkan.
type Options struct {
	Length  int
	Upper   bool
	Digits  bool
	Symbols bool
}

// Default adalah pilihan yang dipakai saat pengguna belum mengubah apa pun.
func Default() Options {
	return Options{Length: 20, Upper: true, Digits: true, Symbols: true}
}

// ErrTooShort dikembalikan saat panjang yang diminta tidak masuk akal.
var ErrTooShort = errors.New("panjang password minimal 8 karakter")

// Generate menghasilkan password acak. Setiap kelas karakter yang diaktifkan
// dijamin muncul minimal sekali, sehingga hasilnya selalu lolos aturan
// kompleksitas situs yang umum.
func Generate(o Options) (string, error) {
	if o.Length < 8 {
		return "", ErrTooShort
	}
	classes := []string{Lower}
	if o.Upper {
		classes = append(classes, Upper)
	}
	if o.Digits {
		classes = append(classes, Digits)
	}
	if o.Symbols {
		classes = append(classes, Symbols)
	}
	pool := strings.Join(classes, "")

	out := make([]byte, 0, o.Length)
	for _, class := range classes {
		c, err := pick(class)
		if err != nil {
			return "", err
		}
		out = append(out, c)
	}
	for len(out) < o.Length {
		c, err := pick(pool)
		if err != nil {
			return "", err
		}
		out = append(out, c)
	}
	if err := shuffle(out); err != nil {
		return "", err
	}
	return string(out), nil
}

func pick(set string) (byte, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(int64(len(set))))
	if err != nil {
		return 0, errors.New("sumber acak sistem tidak tersedia")
	}
	return set[n.Int64()], nil
}

// shuffle mengacak posisi agar karakter wajib tiap kelas tidak selalu di depan.
func shuffle(b []byte) error {
	for i := len(b) - 1; i > 0; i-- {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(i+1)))
		if err != nil {
			return errors.New("sumber acak sistem tidak tersedia")
		}
		j := n.Int64()
		b[i], b[j] = b[j], b[i]
	}
	return nil
}
