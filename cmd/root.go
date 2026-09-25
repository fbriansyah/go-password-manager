// Package cmd assembles the command line interface.
package cmd

import (
	tea "charm.land/bubbletea/v2"
	"github.com/spf13/cobra"

	"github.com/fbriansyah/go-password-manager/internal/keys"
	"github.com/fbriansyah/go-password-manager/internal/tui"
)

var directory string

// Execute runs the application.
func Execute() error { return newRoot().Execute() }

// newRoot assembles the command tree. It is separate from Execute so a test
// can drive the real tree with its own arguments and output.
func newRoot() *cobra.Command {
	root := &cobra.Command{
		Use:   "gopm",
		Short: "File-based password manager for a working folder",
		Long: "gopm stores credentials as encrypted files inside the folder it is run from.\n" +
			"Use -d to point it at another folder without changing directory.",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE:          runTUI,
	}
	root.PersistentFlags().StringVarP(&directory, "directory", "d", "",
		"vault folder (default: the current working directory)")
	root.AddCommand(initCmd(), changeMasterPasswordCmd(), exportKeysCmd(), importKeysCmd(), importSecretsCmd(), clipboardClearCmd())
	return root
}

func runTUI(cmd *cobra.Command, _ []string) error {
	loc, err := keys.Locate(directory)
	if err != nil {
		return err
	}
	if err := loc.RequireIdentity(); err != nil {
		return err
	}
	p := tea.NewProgram(tui.New(loc))
	_, err = p.Run()
	return err
}
