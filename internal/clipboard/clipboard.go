// Package clipboard menyalin nilai lewat tool clipboard sistem.
//
// Penyalinan sengaja didelegasikan ke proses terpisah (wl-copy, xclip, pbcopy)
// dan bukan ditangani di dalam proses ini: di X11 dan Wayland isi clipboard
// dimiliki oleh proses yang menyalinnya, sehingga nilai akan lenyap begitu TUI
// ditutup jika kita memegangnya sendiri.
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

// ClearAfter adalah jeda sebelum nilai yang disalin dihapus dari clipboard.
const ClearAfter = 30 * time.Second

// ErrNoTool dikembalikan saat tidak ada tool clipboard yang terpasang. Fitur
// salin gagal dengan jelas, bukan diam-diam tidak berfungsi.
var ErrNoTool = errors.New("tidak ada tool clipboard (pasang wl-clipboard atau xclip)")

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

// Available melaporkan apakah menyalin bisa dilakukan di sistem ini.
func Available() bool {
	_, err := detect()
	return err == nil
}

// Copy menaruh value di clipboard dan menjadwalkan penghapusannya dengan
// menjalankan ulang binary ini sebagai proses lepas. Nilainya sendiri tidak
// pernah muncul sebagai argumen proses — hanya sidik jarinya — sehingga tidak
// bocor lewat daftar proses.
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

// Clear menghapus isi clipboard hanya jika sidik jarinya masih cocok, sehingga
// nilai yang disalin pengguna sesudahnya tidak ikut terhapus. Sidik jari
// kosong berarti hapus tanpa syarat.
func Clear(want string) error {
	t, err := detect()
	if err != nil {
		return err
	}
	if want != "" {
		current, err := output(t.paste)
		if err != nil {
			// Clipboard kosong atau tidak terbaca: tidak ada yang perlu dihapus.
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
		return fmt.Errorf("gagal menjadwalkan pembersihan clipboard: %w", err)
	}
	cmd := exec.Command(self, "clipboard-clear", "--after", ClearAfter.String(), "--fingerprint", fp)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = nil, nil, nil
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true} // lepas dari TUI
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("gagal menjadwalkan pembersihan clipboard: %w", err)
	}
	return cmd.Process.Release()
}

func run(argv []string, stdin string) error {
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Stdin = strings.NewReader(stdin)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s gagal: %w", argv[0], err)
	}
	return nil
}

func output(argv []string) (string, error) {
	out, err := exec.Command(argv[0], argv[1:]...).Output()
	return string(out), err
}
