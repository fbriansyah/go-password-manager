package cmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/fbriansyah/go-password-manager/internal/config"
	"github.com/fbriansyah/go-password-manager/internal/export"
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
	dir, err := vaultDir()
	if err != nil {
		return err
	}
	cfg, cfgPath, isNew, err := resolveImportConfig(dir)
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

	if err := export.Import(zipData, cfg, password); err != nil {
		return err
	}
	if isNew {
		if err := config.Write(cfgPath, cfg); err != nil {
			os.Remove(cfg.PrivateKeyPath)
			os.Remove(cfg.PublicKeyPath)
			return err
		}
	}

	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "Identity   : %s\n", cfg.PrivateKeyPath)
	fmt.Fprintf(out, "Recipient  : %s\n", cfg.PublicKeyPath)
	if isNew {
		fmt.Fprintf(out, "Config     : %s\n", cfgPath)
	}
	fmt.Fprintln(out)
	fmt.Fprintln(out, "Run `gopm` in any folder to start storing secrets.")
	return nil
}

// resolveImportConfig picks the config import-keys writes into: the existing
// configuration for dir when there is one (so a Vault's .gopm.yaml override
// is honored, docs/adr/0001), or fresh defaults — the same ones `gopm init`
// would choose — when there is no configuration at all yet.
func resolveImportConfig(dir string) (cfg config.Config, cfgPath string, isNew bool, err error) {
	cfg, err = config.Load(dir)
	if err == nil {
		return cfg, cfg.Source, false, nil
	}
	if !errors.Is(err, config.ErrNotConfigured) {
		return config.Config{}, "", false, err
	}
	cfg, cfgPath, err = config.Defaults()
	if err != nil {
		return config.Config{}, "", false, err
	}
	return cfg, cfgPath, true, nil
}
