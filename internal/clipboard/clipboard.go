// Package clipboard copies values through the system clipboard tool.
//
// Copying is deliberately delegated to a separate process (wl-copy, xclip,
// pbcopy) instead of being handled in-process: on X11 and Wayland the clipboard
// contents are owned by the process that copied them, so the value would vanish
// the moment the TUI exits if we held it ourselves.
package clipboard

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"syscall"
	"time"
)

// ClearAfter is how long a copied value stays in the clipboard.
const ClearAfter = 30 * time.Second

// ErrNoTool is returned when no clipboard tool is installed. Copying fails
// loudly rather than quietly doing nothing.
var ErrNoTool = errors.New("no clipboard tool found (install wl-clipboard or xclip)")

type tool struct {
	copy  []string
	paste []string
	clear []string
}

func detect() (tool, error) {
	if runtime.GOOS == "darwin" {
		if _, err := exec.LookPath("pbcopy"); err == nil {
			return tool{copy: []string{"pbcopy"}, paste: []string{"pbpaste"}, clear: []string{"pbcopy"}}, nil
		}
	}
	if _, err := exec.LookPath("wl-copy"); err == nil {
		return tool{
			copy:  []string{"wl-copy"},
			paste: []string{"wl-paste", "--no-newline"},
			clear: []string{"wl-copy", "--clear"},
		}, nil
	}
	if _, err := exec.LookPath("xclip"); err == nil {
		return tool{
			copy:  []string{"xclip", "-selection", "clipboard"},
			paste: []string{"xclip", "-selection", "clipboard", "-o"},
			clear: []string{"xclip", "-selection", "clipboard"},
		}, nil
	}
	return tool{}, ErrNoTool
}

// Available reports whether copying is possible on this system.
func Available() bool {
	_, err := detect()
	return err == nil
}

// Copy puts value in the clipboard and schedules its removal by re-running this
// binary as a detached process. The value itself never appears as a process
// argument — only its fingerprint does — so it cannot leak through the process
// list.
func Copy(value string) error {
	t, err := detect()
	if err != nil {
		return err
	}
	if err := run(t.copy, value); err != nil {
		return err
	}
	return scheduleClear(fingerprint(value))
}

// Clear empties the clipboard only if its fingerprint still matches, so a value
// the user copied afterwards is not wiped along with it. An empty fingerprint
// means clear unconditionally.
func Clear(want string) error {
	t, err := detect()
	if err != nil {
		return err
	}
	if want != "" {
		current, err := output(t.paste)
		if err != nil {
			// Clipboard empty or unreadable: nothing to clear.
			return nil
		}
		if fingerprint(current) != want {
			return nil
		}
	}
	return run(t.clear, "")
}

func fingerprint(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func scheduleClear(fp string) error {
	self, err := os.Executable()
	if err != nil {
		return fmt.Errorf("could not schedule the clipboard clear: %w", err)
	}
	cmd := exec.Command(self, "clipboard-clear", "--after", ClearAfter.String(), "--fingerprint", fp)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = nil, nil, nil
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true} // detach from the TUI
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("could not schedule the clipboard clear: %w", err)
	}
	return cmd.Process.Release()
}

func run(argv []string, stdin string) error {
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Stdin = strings.NewReader(stdin)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s failed: %w", argv[0], err)
	}
	return nil
}

func output(argv []string) (string, error) {
	out, err := exec.Command(argv[0], argv[1:]...).Output()
	return string(out), err
}
