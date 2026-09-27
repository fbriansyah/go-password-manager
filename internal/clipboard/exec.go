package clipboard

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
)

// execTool is the adapter over one external clipboard tool: three argv lines,
// one per thing the seam above asks for.
type execTool struct {
	copy  []string
	paste []string
	clear []string
}

// name is the binary whose presence decides whether this system has the tool.
func (t execTool) name() string { return t.copy[0] }

func (t execTool) put(value string) error { return run(t.copy, value) }
func (t execTool) get() (string, error)   { return output(t.paste) }
func (t execTool) wipe() error            { return run(t.clear, "") }

// knownTools lists the clipboard tools this project knows, most preferred for
// goos first. pbcopy comes first on macOS because it is always installed there.
// Everywhere else wl-copy is tried before xclip: a Wayland session often has
// Xwayland as well, so xclip being present does not mean X11 is the session in
// charge. A macOS machine still falls through to both, which is what a user
// running a Linux desktop environment on one would have.
//
// The order is the one the old detect() had. Moving it here does not change it.
func knownTools(goos string) []execTool {
	wayland := execTool{
		copy:  []string{"wl-copy"},
		paste: []string{"wl-paste", "--no-newline"},
		clear: []string{"wl-copy", "--clear"},
	}
	x11 := execTool{
		copy:  []string{"xclip", "-selection", "clipboard"},
		paste: []string{"xclip", "-selection", "clipboard", "-o"},
		clear: []string{"xclip", "-selection", "clipboard"},
	}
	if goos == "darwin" {
		mac := execTool{
			copy:  []string{"pbcopy"},
			paste: []string{"pbpaste"},
			clear: []string{"pbcopy"},
		}
		return []execTool{mac, wayland, x11}
	}
	return []execTool{wayland, x11}
}

// scheduleClear re-runs this binary as a detached process that clears the
// clipboard once ClearAfter has passed. Only the fingerprint is passed on, so
// the value never appears in the process list.
func scheduleClear(fp string) error {
	self, err := os.Executable()
	if err != nil {
		return fmt.Errorf("could not schedule the clipboard clear: %w", err)
	}
	cmd := exec.Command(self, ClearCommand, "--after", ClearAfter.String(), "--fingerprint", fp)
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
