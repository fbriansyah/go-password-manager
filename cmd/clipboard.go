package cmd

import (
	"time"

	"github.com/spf13/cobra"

	"github.com/fbriansyah/go-password-manager/internal/clipboard"
)

// clipboardClearCmd is run as a detached process by the TUI to clear the
// clipboard after a delay. It is hidden on purpose: an internal detail, not a
// command anyone needs to run directly.
func clipboardClearCmd() *cobra.Command {
	var after time.Duration
	var fingerprint string

	cmd := &cobra.Command{
		Use:    "clipboard-clear",
		Short:  "Clear the clipboard after a delay",
		Hidden: true,
		Args:   cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			time.Sleep(after)
			return clipboard.Clear(fingerprint)
		},
	}
	cmd.Flags().DurationVar(&after, "after", clipboard.ClearAfter, "delay before the clipboard is cleared")
	cmd.Flags().StringVar(&fingerprint, "fingerprint", "", "clear only if the clipboard contents still match")
	return cmd
}
