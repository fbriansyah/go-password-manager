// These tests are inside the package on purpose. The seam over the system
// clipboard is unexported because nothing outside this package supplies one —
// the three adapters that matter are wl-copy, xclip and pbcopy, and they all
// live here (docs/adr/0014). An exported constructor taking a fake would be
// surface that only tests use.
package clipboard

import (
	"errors"
	"reflect"
	"testing"
)

// fakeSystem is a clipboard held in a string: no tool, no process, and a value
// that can change behind our back the way a real one does.
type fakeSystem struct {
	value  string
	getErr error
	putErr error
	wipes  int
	gets   int
}

func (f *fakeSystem) put(value string) error {
	if f.putErr != nil {
		return f.putErr
	}
	f.value = value
	return nil
}

func (f *fakeSystem) get() (string, error) {
	f.gets++
	if f.getErr != nil {
		return "", f.getErr
	}
	return f.value, nil
}

func (f *fakeSystem) wipe() error {
	f.wipes++
	f.value = ""
	return nil
}

// newClipboard builds the rules over f, with a schedule that records the
// fingerprint it was handed instead of spawning anything.
func newClipboard(f *fakeSystem) (clipboard, *[]string) {
	var scheduled []string
	c := clipboard{
		system: f,
		schedule: func(fp string) error {
			scheduled = append(scheduled, fp)
			return nil
		},
	}
	return c, &scheduled
}

func TestCopyPutsTheValueAndSchedulesItsClear(t *testing.T) {
	f := &fakeSystem{}
	c, scheduled := newClipboard(f)
	if err := c.copy("s3cret-value"); err != nil {
		t.Fatalf("copy: %v", err)
	}
	if f.value != "s3cret-value" {
		t.Fatalf("clipboard holds %q, want the copied value", f.value)
	}
	if len(*scheduled) != 1 || (*scheduled)[0] != fingerprint("s3cret-value") {
		t.Fatalf("scheduled = %v, want one clear for the value's fingerprint", *scheduled)
	}
}

// The fingerprint is what travels to the detached process, so it must never be
// the value itself.
func TestTheScheduledClearDoesNotCarryTheValue(t *testing.T) {
	f := &fakeSystem{}
	c, scheduled := newClipboard(f)
	if err := c.copy("s3cret-value"); err != nil {
		t.Fatalf("copy: %v", err)
	}
	for _, fp := range *scheduled {
		if fp == "s3cret-value" {
			t.Fatal("the value was scheduled instead of its fingerprint")
		}
	}
}

// A value that never reached the clipboard must not have a clear scheduled for
// it: the clear would run against whatever the user copied instead.
func TestCopyThatFailsSchedulesNothing(t *testing.T) {
	f := &fakeSystem{putErr: errors.New("wl-copy failed: exit status 1")}
	c, scheduled := newClipboard(f)
	if err := c.copy("s3cret-value"); err == nil {
		t.Fatal("want the error from the tool")
	}
	if len(*scheduled) != 0 {
		t.Fatalf("scheduled = %v, want nothing", *scheduled)
	}
}

func TestClearWipesTheValueWeCopied(t *testing.T) {
	f := &fakeSystem{}
	c, _ := newClipboard(f)
	if err := c.copy("s3cret-value"); err != nil {
		t.Fatalf("copy: %v", err)
	}
	if err := c.clear(fingerprint("s3cret-value")); err != nil {
		t.Fatalf("clear: %v", err)
	}
	if f.wipes != 1 || f.value != "" {
		t.Fatalf("wipes = %d, clipboard = %q, want it emptied once", f.wipes, f.value)
	}
}

// The guard this module exists for: something else is in the clipboard now, so
// the scheduled clear must leave it alone rather than wipe the user's own copy.
func TestClearLeavesAValueTheUserCopiedAfterwards(t *testing.T) {
	f := &fakeSystem{}
	c, _ := newClipboard(f)
	if err := c.copy("s3cret-value"); err != nil {
		t.Fatalf("copy: %v", err)
	}
	f.value = "an address the user copied since"
	if err := c.clear(fingerprint("s3cret-value")); err != nil {
		t.Fatalf("clear: %v", err)
	}
	if f.wipes != 0 {
		t.Fatal("the clipboard was wiped although it no longer held our value")
	}
	if f.value != "an address the user copied since" {
		t.Fatalf("clipboard = %q, want the user's own value untouched", f.value)
	}
}

// An empty fingerprint means the caller is not claiming to know what is in the
// clipboard, so it is emptied without being read first.
func TestClearWithoutAFingerprintWipesWithoutLooking(t *testing.T) {
	f := &fakeSystem{value: "anything at all"}
	c, _ := newClipboard(f)
	if err := c.clear(""); err != nil {
		t.Fatalf("clear: %v", err)
	}
	if f.gets != 0 {
		t.Fatal("the clipboard was read although no fingerprint was given")
	}
	if f.wipes != 1 {
		t.Fatalf("wipes = %d, want 1", f.wipes)
	}
}

// A clipboard that cannot be read holds nothing we need to clear, and saying so
// is not an error the user should see 30 seconds after copying something.
func TestClearIsQuietWhenTheClipboardCannotBeRead(t *testing.T) {
	f := &fakeSystem{getErr: errors.New("xclip: Error: target STRING not available")}
	c, _ := newClipboard(f)
	if err := c.clear(fingerprint("s3cret-value")); err != nil {
		t.Fatalf("clear: %v", err)
	}
	if f.wipes != 0 {
		t.Fatal("the clipboard was wiped although it could not be read")
	}
}

// The tool order, read from any machine: what an operating system prefers, and
// what it falls back to when the preferred binary is not installed.
func TestEachSystemPrefersItsOwnTool(t *testing.T) {
	cases := []struct {
		name string
		goos string
		have []string // binaries this system has
		want []string // argv the chosen tool copies with
	}{
		{name: "macOS", goos: "darwin", have: []string{"pbcopy", "xclip"}, want: []string{"pbcopy"}},
		{name: "macOS without pbcopy falls through", goos: "darwin", have: []string{"xclip"}, want: []string{"xclip", "-selection", "clipboard"}},
		{name: "Wayland", goos: "linux", have: []string{"wl-copy", "xclip"}, want: []string{"wl-copy"}},
		{name: "X11", goos: "linux", have: []string{"xclip"}, want: []string{"xclip", "-selection", "clipboard"}},
		{name: "pbcopy is not looked for off macOS", goos: "linux", have: []string{"pbcopy"}, want: nil},
		{name: "nothing installed", goos: "linux", have: nil, want: nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, err := open(tc.goos, has(tc.have))
			if tc.want == nil {
				if !errors.Is(err, ErrNoTool) {
					t.Fatalf("err = %v, want ErrNoTool", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("open: %v", err)
			}
			got := c.system.(execTool)
			if !reflect.DeepEqual(got.copy, tc.want) {
				t.Fatalf("copy argv = %v, want %v", got.copy, tc.want)
			}
		})
	}
}

// Every tool must be able to answer all three questions the seam asks, and read
// back the selection it wrote to — a paste argv pointing elsewhere would make
// the fingerprint guard compare against the wrong clipboard.
func TestEveryKnownToolIsFullyWired(t *testing.T) {
	for _, goos := range []string{"darwin", "linux"} {
		for _, tool := range knownTools(goos) {
			if len(tool.copy) == 0 || len(tool.paste) == 0 || len(tool.clear) == 0 {
				t.Fatalf("%s on %s: an argv is missing: %+v", tool.name(), goos, tool)
			}
			if tool.name() != tool.copy[0] {
				t.Fatalf("%s is looked up under a different name than it copies with", tool.name())
			}
			if len(tool.copy) > 1 && !reflect.DeepEqual(tool.copy[1:], tool.paste[1:len(tool.copy)]) {
				t.Fatalf("%s pastes from a different selection than it copies to: %v vs %v", tool.name(), tool.copy, tool.paste)
			}
		}
	}
}

// The exported entry points detect a tool first, so with none installed they
// fail with ErrNoTool rather than running anything.
func TestWithoutAClipboardToolCopyingFailsClearly(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	if err := Copy("s3cret-value"); !errors.Is(err, ErrNoTool) {
		t.Fatalf("Copy err = %v, want ErrNoTool", err)
	}
	if err := Clear(""); !errors.Is(err, ErrNoTool) {
		t.Fatalf("Clear err = %v, want ErrNoTool", err)
	}
}

// has builds an installed that answers for exactly these binaries.
func has(names []string) installed {
	return func(name string) (string, error) {
		for _, n := range names {
			if n == name {
				return "/usr/bin/" + name, nil
			}
		}
		return "", errors.New("executable file not found in $PATH")
	}
}
