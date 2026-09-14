package totp_test

import (
	"testing"
	"time"

	"github.com/fbriansyah/go-password-manager/internal/totp"
)

// The SHA1 seed of RFC 6238 Appendix B, "12345678901234567890", in base32.
const rfcSeed = "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ"

func TestBareBase32SeedMatchesRFC6238Vectors(t *testing.T) {
	seed, err := totp.Parse(rfcSeed)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	for _, tc := range []struct {
		at   int64
		want string
	}{
		{59, "287082"},
		{1111111109, "081804"},
		{1234567890, "005924"},
		{20000000000, "353130"},
	} {
		if got := seed.Code(time.Unix(tc.at, 0)); got != tc.want {
			t.Errorf("Code at %d = %q, want %q", tc.at, got, tc.want)
		}
	}
}

// The SHA256 seed of RFC 6238 Appendix B, "12345678901234567890123456789012".
const rfcSeed256 = "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQGEZA===="

func TestURIParametersAreHonoured(t *testing.T) {
	seed, err := totp.Parse("otpauth://totp/Bank:budi?secret=" + rfcSeed256 + "&issuer=Bank&digits=8&algorithm=SHA256")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if got := seed.Code(time.Unix(59, 0)); got != "46119246" {
		t.Fatalf("Code = %q, want the RFC SHA256/8-digit vector 46119246", got)
	}

	sixty, err := totp.Parse("otpauth://totp/x?secret=" + rfcSeed + "&period=60")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if a, b := sixty.Code(time.Unix(0, 0)), sixty.Code(time.Unix(59, 0)); a != b {
		t.Fatalf("with period=60 the Code changed inside one minute: %q then %q", a, b)
	}
	if got := sixty.Code(time.Unix(59, 0)); got == "287082" {
		t.Fatal("period=60 produced the 30-second Code; the period was ignored")
	}
}

func TestRemainingCountsDownToTheNextCode(t *testing.T) {
	seed, err := totp.Parse(rfcSeed)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	for _, tc := range []struct {
		at   int64
		want time.Duration
	}{
		{0, 30 * time.Second},
		{1, 29 * time.Second},
		{29, 1 * time.Second},
		{30, 30 * time.Second},
	} {
		if got := seed.Remaining(time.Unix(tc.at, 0)); got != tc.want {
			t.Errorf("Remaining at %d = %v, want %v", tc.at, got, tc.want)
		}
	}
	if seed.Code(time.Unix(29, 0)) == seed.Code(time.Unix(30, 0)) {
		t.Fatal("the Code did not roll over at the period boundary")
	}
}

func TestMalformedSeedsAreRejected(t *testing.T) {
	for _, bad := range []string{
		"",
		"not base32!",
		"otpauth://hotp/x?secret=" + rfcSeed + "&counter=1",
		"otpauth://totp/x",
		"otpauth://totp/x?secret=" + rfcSeed + "&digits=7",
		"otpauth://totp/x?secret=" + rfcSeed + "&algorithm=MD5",
		"otpauth://totp/x?secret=" + rfcSeed + "&period=0",
	} {
		if _, err := totp.Parse(bad); err == nil {
			t.Errorf("Parse(%q) accepted a malformed seed", bad)
		}
	}
}

func TestBareSeedsAreForgivingAboutCaseSpacesAndPadding(t *testing.T) {
	want, _ := totp.Parse(rfcSeed)
	for _, v := range []string{
		"gezd gnbv gy3t qojq gezd gnbv gy3t qojq",
		"  " + rfcSeed + "====\n",
	} {
		seed, err := totp.Parse(v)
		if err != nil {
			t.Fatalf("Parse(%q): %v", v, err)
		}
		if seed.Code(time.Unix(59, 0)) != want.Code(time.Unix(59, 0)) {
			t.Fatalf("Parse(%q) produced a different Code", v)
		}
	}
}
