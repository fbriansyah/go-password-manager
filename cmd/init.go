package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/fbriansyah/go-password-manager/internal/config"
	"github.com/fbriansyah/go-password-manager/internal/crypto"
)

func initCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Create the keypair and the initial configuration",
		Long: "Creates the Identity (a private key encrypted with the master password) and the Recipient (a public key),\n" +
			"then writes the configuration that points at both.",
		Args: cobra.NoArgs,
		RunE: runInit,
	}
}

func runInit(cmd *cobra.Command, _ []string) error {
	cfg, cfgPath, err := config.Defaults()
	if err != nil {
		return err
	}
	if _, err := os.Stat(cfgPath); err == nil {
		return fmt.Errorf("%s already exists; remove it yourself if you really want to start over", cfgPath)
	}

	out := cmd.OutOrStdout()
	fmt.Fprintln(out, "The master password protects the private key that opens every secret.")
	fmt.Fprintln(out, "There is no way to recover it: losing the private key or forgetting the password")
	fmt.Fprintln(out, "means every secret is gone permanently.")
	fmt.Fprintln(out)

	password, err := askNewMasterPassword(
		"Master password: ", "Repeat the master password: ", "")
	if err != nil {
		return err
	}

	if err := crypto.GenerateKeypair(cfg.PrivateKeyPath, cfg.PublicKeyPath, password); err != nil {
		return err
	}
	if err := config.Write(cfgPath, cfg); err != nil {
		return err
	}

	fmt.Fprintln(out)
	fmt.Fprintf(out, "Identity   : %s\n", cfg.PrivateKeyPath)
	fmt.Fprintf(out, "Recipient  : %s\n", cfg.PublicKeyPath)
	fmt.Fprintf(out, "Config     : %s\n", cfgPath)
	fmt.Fprintln(out)
	fmt.Fprintln(out, "Back up the identity file now. Without it, secrets can never be opened again.")
	fmt.Fprintln(out, "Run `gopm` in any folder to start storing secrets.")
	return nil
}
