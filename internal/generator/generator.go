// Package generator makes random passwords to fill Fields of type ps.
package generator

import (
	"crypto/rand"
	"errors"
	"math/big"
	"strings"
)

// The character classes that can be picked.
const (
	Lower   = "abcdefghijkmnopqrstuvwxyz"
	Upper   = "ABCDEFGHJKLMNPQRSTUVWXYZ"
	Digits  = "23456789"
	Symbols = "!@#$%^&*()-_=+[]{};:,.?"
)

// Options shapes the generated password.
type Options struct {
	Length  int
	Upper   bool
	Digits  bool
	Symbols bool
}

// Default is what is used until the user changes anything.
func Default() Options {
	return Options{Length: 20, Upper: true, Digits: true, Symbols: true}
}

// ErrTooShort is returned when the requested length is unreasonable.
var ErrTooShort = errors.New("a password must be at least 8 characters long")

// Generate produces a random password. Every enabled character class is
// guaranteed to appear at least once, so the result always passes the usual
// site complexity rules.
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
		return 0, errors.New("the system random source is unavailable")
	}
	return set[n.Int64()], nil
}

// shuffle mixes positions so the mandatory character of each class is not always first.
func shuffle(b []byte) error {
	for i := len(b) - 1; i > 0; i-- {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(i+1)))
		if err != nil {
			return errors.New("the system random source is unavailable")
		}
		j := n.Int64()
		b[i], b[j] = b[j], b[i]
	}
	return nil
}
