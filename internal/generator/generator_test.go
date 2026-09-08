package generator_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/fbriansyah/go-password-manager/internal/generator"
)

func TestGenerateLengthAndVariety(t *testing.T) {
	o := generator.Default()
	seen := map[string]bool{}
	for i := 0; i < 50; i++ {
		pw, err := generator.Generate(o)
		if err != nil {
			t.Fatalf("Generate: %v", err)
		}
		if len(pw) != o.Length {
			t.Fatalf("length = %d, want %d", len(pw), o.Length)
		}
		for name, set := range map[string]string{
			"lowercase": generator.Lower,
			"uppercase": generator.Upper,
			"digits":    generator.Digits,
			"simbol":    generator.Symbols,
		} {
			if !strings.ContainsAny(pw, set) {
				t.Fatalf("%q does not contain %s", pw, name)
			}
		}
		seen[pw] = true
	}
	if len(seen) != 50 {
		t.Fatalf("only %d unique passwords out of 50; the randomness looks suspect", len(seen))
	}
}

func TestGenerateHonoursDisabledClasses(t *testing.T) {
	pw, err := generator.Generate(generator.Options{Length: 16})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if strings.ContainsAny(pw, generator.Upper+generator.Digits+generator.Symbols) {
		t.Fatalf("%q contains a disabled class", pw)
	}
}

func TestGenerateRefusesAnUnsafeLength(t *testing.T) {
	if _, err := generator.Generate(generator.Options{Length: 4}); !errors.Is(err, generator.ErrTooShort) {
		t.Fatalf("err = %v, want ErrTooShort", err)
	}
}
