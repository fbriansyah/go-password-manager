package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/fbriansyah/go-password-manager/internal/crypto"
	"github.com/fbriansyah/go-password-manager/internal/keys"
)

func changeMasterPasswordCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "change-master-password",
		Short: "Reseal the Identity with a new master password",
		Long: "Opens the Identity with the current master password and writes it back encrypted with a new one.\n" +
			"Secrets and the Recipient are untouched. Backups made earlier with `gopm export-keys`\n" +
			"keep the master password they were made with.",
		Args: cobra.NoArgs,
		RunE: runChangeMasterPassword,
	}
}

func runChangeMasterPassword(cmd *cobra.Command, _ []string) error {
	loc, err := keys.Locate(directory)
	if err != nil {
		return err
	}
	if err := loc.RequireIdentity(); err != nil {
		return err
	}

	// The current password is verified before the new one is asked for, so a
	// typo here fails fast instead of after two more prompts.
	current, err := askPassword("Current master password: ")
	if err != nil {
		return err
	}
	session, err := loc.Identity(current)
	if err != nil {
		return err
	}

	next, err := askNewMasterPassword(
		"New master password: ", "Repeat the new master password: ", current)
	if err != nil {
		return err
	}

	if err := crypto.ChangePassword(loc.Config.PrivateKeyPath, loc.Config.PublicKeyPath, session, next); err != nil {
		return err
	}

	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "Master password changed for %s\n", loc.Config.PrivateKeyPath)
	fmt.Fprintln(out)
	fmt.Fprintln(out, "Backups made with `gopm export-keys` before now still open with the previous master password.")
	fmt.Fprintln(out, "Run `gopm export-keys` to make a fresh one.")
	return nil
}
