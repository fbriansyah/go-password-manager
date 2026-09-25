package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/fbriansyah/go-password-manager/internal/export"
	"github.com/fbriansyah/go-password-manager/internal/keys"
)

var exportKeysOutput string

func exportKeysCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "export-keys",
		Short: "Bundle the Identity and the Recipient into a zip",
		Long: "Verifies the master password against the Identity, then writes a zip containing the Identity,\n" +
			"the Recipient, and a manifest, ready to copy to another drive. The Vault itself is untouched.",
		Args: cobra.NoArgs,
		RunE: runExportKeys,
	}
	cmd.Flags().StringVarP(&exportKeysOutput, "output", "o", "",
		"destination zip path (default: gopm-keys-<timestamp>.zip in the current directory)")
	return cmd
}

func runExportKeys(cmd *cobra.Command, _ []string) error {
	loc, err := keys.Locate(directory)
	if err != nil {
		return err
	}
	if err := loc.RequireIdentity(); err != nil {
		return err
	}

	out := exportKeysOutput
	if out == "" {
		out = fmt.Sprintf("gopm-keys-%s.zip", time.Now().Format("20060102-150405"))
	}
	if _, err := os.Stat(out); err == nil {
		return fmt.Errorf("%s already exists; remove it or choose another path with -o", out)
	}

	password, err := askPassword("Master password: ")
	if err != nil {
		return err
	}

	f, err := os.OpenFile(out, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return fmt.Errorf("could not create %s: %w", out, err)
	}
	if err := export.Keys(loc.Config, password, f); err != nil {
		f.Close()
		os.Remove(out)
		return err
	}
	if err := f.Close(); err != nil {
		os.Remove(out)
		return fmt.Errorf("could not finish writing %s: %w", out, err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Keys exported to %s\n", out)
	return nil
}
