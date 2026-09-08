package generator_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/fbriansyah/go-password-manager/internal/generator"
)

func TestGeneratePanjangDanKeragaman(t *testing.T) {
	o := generator.Default()
	seen := map[string]bool{}
	for i := 0; i < 50; i++ {
		pw, err := generator.Generate(o)
		if err != nil {
			t.Fatalf("Generate: %v", err)
		}
		if len(pw) != o.Length {
			t.Fatalf("panjang = %d, mau %d", len(pw), o.Length)
		}
		for name, set := range map[string]string{
			"huruf kecil": generator.Lower,
			"huruf besar": generator.Upper,
			"angka":       generator.Digits,
			"simbol":      generator.Symbols,
		} {
			if !strings.ContainsAny(pw, set) {
				t.Fatalf("%q tidak mengandung %s", pw, name)
			}
		}
		seen[pw] = true
	}
	if len(seen) != 50 {
		t.Fatalf("hanya %d password unik dari 50; keacakan mencurigakan", len(seen))
	}
}

func TestGenerateMenghormatiKelasYangDimatikan(t *testing.T) {
	pw, err := generator.Generate(generator.Options{Length: 16})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if strings.ContainsAny(pw, generator.Upper+generator.Digits+generator.Symbols) {
		t.Fatalf("%q memuat kelas yang dimatikan", pw)
	}
}

func TestGenerateMenolakPanjangTidakAman(t *testing.T) {
	if _, err := generator.Generate(generator.Options{Length: 4}); !errors.Is(err, generator.ErrTooShort) {
		t.Fatalf("err = %v, mau ErrTooShort", err)
	}
}
