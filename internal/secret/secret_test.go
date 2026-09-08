package secret_test

import (
	"strings"
	"testing"

	"github.com/fbriansyah/go-password-manager/internal/secret"
)

func TestUnmarshalKeepsForeignTypes(t *testing.T) {
	raw := []byte(`{
	  "meta": {"title": "Facebook"},
	  "fields": [
	    {"type": "tx", "label": "Username", "value": "budi"},
	    {"type": "totp", "label": "2FA", "value": "JBSWY3DPEHPK3PXP"}
	  ]
	}`)
	s, err := secret.Unmarshal(raw)
	if err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	tipe := secret.TypeFor(s.Fields[1].Type)
	if tipe.Known() {
		t.Fatal("totp should not be a known type yet")
	}
	if got := tipe.Render(s.Fields[1].Value, false); got != "JBSWY3DPEHPK3PXP" {
		t.Fatalf("a foreign type was not rendered as plain text: %q", got)
	}
	// Saving again must not drop fields whose type we do not recognise.
	out, err := secret.Marshal(s)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if !strings.Contains(string(out), "JBSWY3DPEHPK3PXP") || !strings.Contains(string(out), `"totp"`) {
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
	if strings.Join(ids, ",") != "tx,ps,ta" {
		t.Fatalf("Types = %v, want [tx ps ta] in registration order", ids)
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
