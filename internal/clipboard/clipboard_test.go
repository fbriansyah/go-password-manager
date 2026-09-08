package clipboard_test

import (
	"errors"
	"testing"

	"github.com/fbriansyah/go-password-manager/internal/clipboard"
)

// With no clipboard tool, copying must fail with a message naming what needs
// to be installed — not quietly do nothing.
func TestWithoutAClipboardToolCopyingFailsClearly(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	if clipboard.Available() {
		t.Fatal("Available() is true with no tool on PATH")
	}
	if err := clipboard.Copy("s3cret-value"); !errors.Is(err, clipboard.ErrNoTool) {
		t.Fatalf("err = %v, want ErrNoTool", err)
	}
	if err := clipboard.Clear(""); !errors.Is(err, clipboard.ErrNoTool) {
		t.Fatalf("Clear err = %v, want ErrNoTool", err)
	}
}
