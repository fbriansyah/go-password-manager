package cmd

import (
	"time"

	"github.com/spf13/cobra"

	"github.com/fbriansyah/go-password-manager/internal/clipboard"
)

// clipboardClearCmd dijalankan sebagai proses lepas oleh TUI untuk menghapus
// clipboard setelah jeda. Ia sengaja tersembunyi: ini detail internal, bukan
// perintah yang perlu dipakai langsung.
func clipboardClearCmd() *cobra.Command {
	var after time.Duration
	var fingerprint string

	cmd := &cobra.Command{
		Use:    "clipboard-clear",
		Short:  "Menghapus clipboard setelah jeda",
		Hidden: true,
		Args:   cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			time.Sleep(after)
			return clipboard.Clear(fingerprint)
		},
	}
	cmd.Flags().DurationVar(&after, "after", clipboard.ClearAfter, "jeda sebelum clipboard dihapus")
	cmd.Flags().StringVar(&fingerprint, "fingerprint", "", "hapus hanya jika isi clipboard masih cocok")
	return cmd
}
