// Package totp derives Codes from an Authenticator Seed (RFC 6238 over
// RFC 4226). It is a leaf: it knows nothing about Secrets or the TUI.
package totp

import (
	"crypto/hmac"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"hash"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Now is the clock Codes are derived from when no instant is given; tests
// replace it.
var Now = time.Now

// Seed is a parsed Authenticator Seed together with the parameters that shape
// its Codes.
type Seed struct {
	key    []byte
	digits int
	period time.Duration
	algo   func() hash.Hash
}

// Parse reads an Authenticator Seed as a service hands it over: either a bare
// base32 secret, read with RFC 6238 defaults, or an otpauth://totp/ URI whose
// digits, period and algorithm are honoured. The label and issuer inside a
// URI are ignored — a Secret's Meta already carries its name (ADR 0008).
func Parse(value string) (Seed, error) {
	value = strings.TrimSpace(value)
	if strings.HasPrefix(strings.ToLower(value), "otpauth://") {
		return parseURI(value)
	}
	key, err := decodeBase32(value)
	if err != nil {
		return Seed{}, err
	}
	return Seed{key: key, digits: 6, period: 30 * time.Second, algo: sha1.New}, nil
}

func parseURI(value string) (Seed, error) {
	u, err := url.Parse(value)
	if err != nil {
		return Seed{}, fmt.Errorf("the seed is not a valid otpauth URI: %w", err)
	}
	if !strings.EqualFold(u.Host, "totp") {
		return Seed{}, fmt.Errorf("only otpauth://totp/ is supported, not %q", u.Host)
	}
	q := u.Query()
	key, err := decodeBase32(q.Get("secret"))
	if err != nil {
		return Seed{}, err
	}
	seed := Seed{key: key, digits: 6, period: 30 * time.Second, algo: sha1.New}
	if d := q.Get("digits"); d != "" {
		n, err := strconv.Atoi(d)
		if err != nil || (n != 6 && n != 8) {
			return Seed{}, fmt.Errorf("digits must be 6 or 8, not %q", d)
		}
		seed.digits = n
	}
	if p := q.Get("period"); p != "" {
		n, err := strconv.Atoi(p)
		if err != nil || n <= 0 {
			return Seed{}, fmt.Errorf("period must be a positive number of seconds, not %q", p)
		}
		seed.period = time.Duration(n) * time.Second
	}
	switch a := strings.ToUpper(q.Get("algorithm")); a {
	case "", "SHA1":
	case "SHA256":
		seed.algo = sha256.New
	case "SHA512":
		seed.algo = sha512.New
	default:
		return Seed{}, fmt.Errorf("algorithm must be SHA1, SHA256 or SHA512, not %q", a)
	}
	return seed, nil
}

func decodeBase32(s string) ([]byte, error) {
	s = strings.ToUpper(strings.ReplaceAll(s, " ", ""))
	s = strings.TrimRight(s, "=")
	if s == "" {
		return nil, fmt.Errorf("the seed is empty")
	}
	key, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(s)
	if err != nil {
		return nil, fmt.Errorf("the seed is not base32: %w", err)
	}
	return key, nil
}

// Code returns the Code that is valid at the given instant.
func (s Seed) Code(at time.Time) string {
	counter := uint64(at.Unix()) / uint64(s.period/time.Second)
	var msg [8]byte
	binary.BigEndian.PutUint64(msg[:], counter)
	mac := hmac.New(s.algo, s.key)
	mac.Write(msg[:])
	sum := mac.Sum(nil)
	offset := sum[len(sum)-1] & 0x0f
	code := binary.BigEndian.Uint32(sum[offset:offset+4]) & 0x7fffffff
	mod := uint32(1)
	for i := 0; i < s.digits; i++ {
		mod *= 10
	}
	return fmt.Sprintf("%0*d", s.digits, code%mod)
}

// Remaining reports how long the Code valid at the given instant stays valid;
// it is never zero, because at the boundary a fresh Code has just begun.
func (s Seed) Remaining(at time.Time) time.Duration {
	elapsed := time.Duration(at.Unix()%int64(s.period/time.Second)) * time.Second
	return s.period - elapsed
}
