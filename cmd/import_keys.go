package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/fbriansyah/go-password-manager/internal/config"
	"github.com/fbriansyah/go-password-manager/internal/export"
	"github.com/fbriansyah/go-password-manager/internal/keys"
)

func importKeysCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "import-keys <zip>",
		Short: "Install an Identity and Recipient from an export-keys zip",
		Long: "Installs the Identity and Recipient carried by a zip made with `gopm export-keys`.\n" +
			"Refuses if a keypair already exists at the destination; writes a configuration\n" +
			"if none exists yet, the same way `gopm init` would.",
		Args: cobra.ExactArgs(1),
		RunE: runImportKeys,
	}
}

func runImportKeys(cmd *cobra.Command, args []string) error {
	// import-keys installs the very keypair a configuration points at, so it
	// is the one command that may run before there is any configuration at
	// all (docs/adr/0010).
	loc, err := keys.LocateOrDefaults(directory)
	if err != nil {
		return err
	}

	zipData, err := os.ReadFile(args[0])
	if err != nil {
		return fmt.Errorf("could not read %s: %w", args[0], err)
	}

	password, err := askPassword("Master password: ")
	if err != nil {
		return err
	}

	if err := export.Import(zipData, loc.Config, password); err != nil {
		return err
	}
	if !loc.Configured {
		if err := config.Write(loc.ConfigPath, loc.Config); err != nil {
			os.Remove(loc.Config.PrivateKeyPath)
			os.Remove(loc.Config.PublicKeyPath)
			return err
		}
	}

	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "Identity   : %s\n", loc.Config.PrivateKeyPath)
	fmt.Fprintf(out, "Recipient  : %s\n", loc.Config.PublicKeyPath)
	if !loc.Configured {
		fmt.Fprintf(out, "Config     : %s\n", loc.ConfigPath)
	}
	fmt.Fprintln(out)
	fmt.Fprintln(out, "Run `gopm` in any folder to start storing secrets.")
	return nil
}
