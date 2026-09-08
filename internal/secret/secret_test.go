package secret_test

import (
	"strings"
	"testing"

	"github.com/fbriansyah/go-password-manager/internal/secret"
)

func TestUnmarshalMempertahankanTipeAsing(t *testing.T) {
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
		t.Fatal("totp seharusnya belum dikenal")
	}
	if got := tipe.Render(s.Fields[1].Value, false); got != "JBSWY3DPEHPK3PXP" {
		t.Fatalf("tipe asing tidak dirender sebagai teks biasa: %q", got)
	}
	// Menyimpan ulang tidak boleh menghilangkan field yang tidak dikenali.
	out, err := secret.Marshal(s)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if !strings.Contains(string(out), "JBSWY3DPEHPK3PXP") || !strings.Contains(string(out), `"totp"`) {
		t.Fatalf("field asing hilang saat disimpan ulang: %s", out)
	}
}

func TestPasswordDisembunyikanSampaiDiminta(t *testing.T) {
	ps := secret.TypeFor("ps")
	if got := ps.Render("rahasia123", false); strings.Contains(got, "rahasia") {
		t.Fatalf("password bocor tanpa reveal: %q", got)
	}
	if got := ps.Render("rahasia123", true); got != "rahasia123" {
		t.Fatalf("reveal tidak menampilkan nilai: %q", got)
	}
	if got, _ := ps.Copy("rahasia123"); got != "rahasia123" {
		t.Fatalf("Copy = %q, mau nilai mentah", got)
	}
}

func TestTypesHanyaTipeYangBisaDibuat(t *testing.T) {
	var ids []string
	for _, tp := range secret.Types() {
		ids = append(ids, tp.ID)
	}
	if strings.Join(ids, ",") != "tx,ps,ta" {
		t.Fatalf("Types = %v, mau [tx ps ta] sesuai urutan pendaftaran", ids)
	}
}

func TestValidateMenolakFieldTanpaLabel(t *testing.T) {
	s := &secret.Secret{
		Meta:   secret.Meta{Title: "Facebook"},
		Fields: []secret.Field{{Type: "tx", Label: "  ", Value: "x"}},
	}
	if err := s.Validate(); err == nil {
		t.Fatal("mau error untuk field tanpa label")
	}
}
