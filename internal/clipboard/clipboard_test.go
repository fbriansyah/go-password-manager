package clipboard_test

import (
	"errors"
	"testing"

	"github.com/fbriansyah/go-password-manager/internal/clipboard"
)

// Tanpa tool clipboard, menyalin harus gagal dengan pesan yang menyebut apa
// yang perlu dipasang — bukan diam-diam tidak berfungsi.
func TestTanpaToolClipboardGagalDenganJelas(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	if clipboard.Available() {
		t.Fatal("Available() true padahal tidak ada tool di PATH")
	}
	if err := clipboard.Copy("rahasia123"); !errors.Is(err, clipboard.ErrNoTool) {
		t.Fatalf("err = %v, mau ErrNoTool", err)
	}
	if err := clipboard.Clear(""); !errors.Is(err, clipboard.ErrNoTool) {
		t.Fatalf("Clear err = %v, mau ErrNoTool", err)
	}
}
