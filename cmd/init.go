package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"

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

	password, err := askPassword("Master password: ")
	if err != nil {
		return err
	}
	if len(password) < 8 {
		return fmt.Errorf("the master password must be at least 8 characters")
	}
	again, err := askPassword("Repeat the master password: ")
	if err != nil {
		return err
	}
	if password != again {
		return fmt.Errorf("the master passwords do not match")
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

// askPassword reads a password without echoing it. If the input is not a
// terminal the read is refused — a password must not arrive through a pipe that
// is easily kept in shell history or logs.
func askPassword(prompt string) (string, error) {
	fd := int(os.Stdin.Fd())
	if !term.IsTerminal(fd) {
		return "", fmt.Errorf("the master password must be typed in a terminal")
	}
	fmt.Fprint(os.Stderr, prompt)
	b, err := term.ReadPassword(fd)
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return "", fmt.Errorf("could not read the master password: %w", err)
	}
	return strings.TrimSpace(string(b)), nil
}
