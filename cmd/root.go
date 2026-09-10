// Package cmd assembles the command line interface.
package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	tea "charm.land/bubbletea/v2"
	"github.com/spf13/cobra"

	"github.com/fbriansyah/go-password-manager/internal/config"
	"github.com/fbriansyah/go-password-manager/internal/tui"
)

var directory string

// Execute runs the application.
func Execute() error {
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
	root.AddCommand(initCmd(), clipboardClearCmd())
	return root.Execute()
}

// vaultDir decides the Vault folder: the value of -d when given, otherwise the
// working directory. A folder that does not exist is refused, not silently
// created — a mistyped path is better failing than becoming a new, empty Vault.
func vaultDir() (string, error) {
	dir := directory
	if dir == "" {
		wd, err := os.Getwd()
		if err != nil {
			return "", fmt.Errorf("working directory is unknown: %w", err)
		}
		dir = wd
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", fmt.Errorf("invalid vault path: %w", err)
	}
	info, err := os.Stat(abs)
	if errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("vault folder %s does not exist", abs)
	}
	if err != nil {
		return "", fmt.Errorf("vault folder cannot be opened: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("%s is not a folder", abs)
	}
	return abs, nil
}

func runTUI(cmd *cobra.Command, _ []string) error {
	dir, err := vaultDir()
	if err != nil {
		return err
	}
	// The .gopm.yaml override is looked for in the Vault folder, not in $PWD, so
	// the keys always follow the Secrets being opened (docs/adr/0001).
	cfg, err := config.Load(dir)
	if err != nil {
		return err
	}
	if _, err := os.Stat(cfg.PrivateKeyPath); err != nil {
		return fmt.Errorf("identity at %s cannot be opened: %w", cfg.PrivateKeyPath, err)
	}
	p := tea.NewProgram(tui.New(dir, cfg))
	_, err = p.Run()
	return err
}
