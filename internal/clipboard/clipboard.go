// Package clipboard copies values through the system clipboard tool.
//
// Copying is deliberately delegated to a separate process (wl-copy, xclip,
// pbcopy) instead of being handled in-process: on X11 and Wayland the clipboard
// contents are owned by the process that copied them, so the value would vanish
// the moment the TUI exits if we held it ourselves.
//
// Which of those tools this system has is the only thing this package cannot
// decide for itself, so it is the only thing behind a seam (docs/adr/0014). The
// rules — which tool an operating system prefers, and whether the clipboard
// still holds the value we put there — live above it and are exercised without
// a clipboard at all.
package clipboard

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os/exec"
	"runtime"
	"time"
)

// ClearAfter is how long a copied value stays in the clipboard.
const ClearAfter = 30 * time.Second

// ErrNoTool is returned when no clipboard tool is installed. Copying fails
// loudly rather than quietly doing nothing.
var ErrNoTool = errors.New("no clipboard tool found (install wl-clipboard or xclip)")

// ClearCommand is the name this binary answers to when it is re-run to clear
// the clipboard. It is written once here and used at both ends: scheduleClear
// spawns it, and cmd registers it.
const ClearCommand = "clipboard-clear"

// system is the system clipboard itself: put a value in it, read what is in it
// now, empty it. wl-copy, xclip and pbcopy are three genuinely different ways
// to hold a clipboard, and each is an adapter over this.
type system interface {
	put(value string) error
	get() (string, error)
	wipe() error
}

// installed reports where a binary is, or an error if this system does not have
// it. It is exec.LookPath in the application.
type installed func(name string) (string, error)

// clipboard is the rules about a copied value, written above the seam.
//
// schedule arranges the later clear. It is a field rather than a direct call so
// that a copy which succeeds is observable without a process being spawned; the
// scheduling itself is still only done one way (see scheduleClear).
type clipboard struct {
	system   system
	schedule func(fingerprint string) error
}

// Copy puts value in the clipboard and schedules its removal by re-running this
// binary as a detached process. The value itself never appears as a process
// argument — only its fingerprint does — so it cannot leak through the process
// list.
func Copy(value string) error {
	c, err := open(runtime.GOOS, exec.LookPath)
	if err != nil {
		return err
	}
	return c.copy(value)
}

// Clear empties the clipboard only if its fingerprint still matches, so a value
// the user copied afterwards is not wiped along with it. An empty fingerprint
// means clear unconditionally.
func Clear(want string) error {
	c, err := open(runtime.GOOS, exec.LookPath)
	if err != nil {
		return err
	}
	return c.clear(want)
}

// open picks the tool goos prefers among the ones this system has. The
// operating system's name is an argument rather than a read of runtime.GOOS so
// that every branch of that order is reachable from any machine.
func open(goos string, have installed) (clipboard, error) {
	for _, t := range knownTools(goos) {
		if _, err := have(t.name()); err == nil {
			return clipboard{system: t, schedule: scheduleClear}, nil
		}
	}
	return clipboard{}, ErrNoTool
}

func (c clipboard) copy(value string) error {
	if err := c.system.put(value); err != nil {
		return err
	}
	// Only a value that reached the clipboard gets a clear scheduled for it.
	return c.schedule(fingerprint(value))
}

func (c clipboard) clear(want string) error {
	if want != "" {
		current, err := c.system.get()
		if err != nil {
			// Clipboard empty or unreadable: nothing to clear.
			return nil
		}
		if fingerprint(current) != want {
			return nil
		}
	}
	return c.system.wipe()
}

func fingerprint(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}
