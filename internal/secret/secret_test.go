package secret_test

import (
	"strings"
	"testing"
	"time"

	"github.com/fbriansyah/go-password-manager/internal/secret"
	"github.com/fbriansyah/go-password-manager/internal/totp"
)

func TestUnmarshalKeepsForeignTypes(t *testing.T) {
	raw := []byte(`{
	  "meta": {"title": "Facebook"},
	  "fields": [
	    {"type": "tx", "label": "Username", "value": "budi"},
	    {"type": "zz", "label": "2FA", "value": "JBSWY3DPEHPK3PXP"}
	  ]
	}`)
	s, err := secret.Unmarshal(raw)
	if err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	tipe := secret.TypeFor(s.Fields[1].Type)
	if tipe.Known() {
		t.Fatal("zz should not be a known type")
	}
	if got := tipe.Render(s.Fields[1].Value, false); got != "JBSWY3DPEHPK3PXP" {
		t.Fatalf("a foreign type was not rendered as plain text: %q", got)
	}
	// Saving again must not drop fields whose type we do not recognise.
	out, err := secret.Marshal(s)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if !strings.Contains(string(out), "JBSWY3DPEHPK3PXP") || !strings.Contains(string(out), `"zz"`) {
		t.Fatalf("a foreign field was lost when saved again: %s", out)
	}
}

func TestPasswordStaysHiddenUntilAskedFor(t *testing.T) {
	ps := secret.TypeFor("ps")
	if got := ps.Render("s3cret-value", false); strings.Contains(got, "s3cret") {
		t.Fatalf("the password leaked without reveal: %q", got)
	}
	if got := ps.Render("s3cret-value", true); got != "s3cret-value" {
		t.Fatalf("reveal did not show the value: %q", got)
	}
	if got, _ := ps.Copy("s3cret-value"); got != "s3cret-value" {
		t.Fatalf("Copy = %q, want the raw value", got)
	}
}

func TestTypesListsOnlyCreatableTypes(t *testing.T) {
	var ids []string
	for _, tp := range secret.Types() {
		ids = append(ids, tp.ID)
	}
	if strings.Join(ids, ",") != "tx,ps,ta,tp" {
		t.Fatalf("Types = %v, want [tx ps ta tp] in registration order", ids)
	}
}

func TestValidateRefusesAFieldWithoutALabel(t *testing.T) {
	s := &secret.Secret{
		Meta:   secret.Meta{Title: "Facebook"},
		Fields: []secret.Field{{Type: "tx", Label: "  ", Value: "x"}},
	}
	if err := s.Validate(); err == nil {
		t.Fatal("want an error for a field without a label")
	}
}

const rfcSeed = "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ" // RFC 6238 Appendix B, SHA1

func at(t *testing.T, unix int64) {
	t.Helper()
	old := totp.Now
	totp.Now = func() time.Time { return time.Unix(unix, 0) }
	t.Cleanup(func() { totp.Now = old })
}

func TestTOTPShowsTheCodeAndCopiesOnlyTheDigits(t *testing.T) {
	at(t, 59) // 287082, one second left
	tp := secret.TypeFor("tp")
	if !tp.Known() {
		t.Fatal("tp is not a registered Field Type")
	}
	if got := tp.Render(rfcSeed, false); got != "287 082  ·  1s" {
		t.Fatalf("Render = %q, want the grouped Code and the countdown", got)
	}
	if got := tp.Render(rfcSeed, true); got != rfcSeed {
		t.Fatalf("reveal did not show the Seed itself: %q", got)
	}
	got, err := tp.Copy(rfcSeed)
	if err != nil || got != "287082" {
		t.Fatalf("Copy = %q, %v; want the bare digits", got, err)
	}
}

func TestTOTPGroupsEightDigitsInFours(t *testing.T) {
	at(t, 59)
	uri := "otpauth://totp/x?secret=GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQGEZA&digits=8&algorithm=SHA256"
	if got := secret.TypeFor("tp").Render(uri, false); got != "4611 9246  ·  1s" {
		t.Fatalf("Render = %q", got)
	}
}

func TestAMalformedSeedRefusesToSaveButDoesNotCrashAnOldFile(t *testing.T) {
	s := &secret.Secret{
		Meta:   secret.Meta{Title: "Bank"},
		Fields: []secret.Field{{Type: "tp", Label: "2FA", Value: "not base32!"}},
	}
	err := s.Validate()
	if err == nil || !strings.Contains(err.Error(), "2FA") {
		t.Fatalf("Validate = %v, want an error naming the Field", err)
	}
	tp := secret.TypeFor("tp")
	if got := tp.Render("not base32!", false); got != "invalid seed" {
		t.Fatalf("Render of a bad seed = %q", got)
	}
	if _, err := tp.Copy("not base32!"); err == nil {
		t.Fatal("Copy of a bad seed did not fail")
	}
}
