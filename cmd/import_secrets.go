package cmd

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/fbriansyah/go-password-manager/internal/importer"
	"github.com/fbriansyah/go-password-manager/internal/keys"
	"github.com/fbriansyah/go-password-manager/internal/vault"
)

var (
	importSecretsFrom   string
	importSecretsDryRun bool
)

func importSecretsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "import-secrets <file>",
		Short: "Fill the Vault from another password manager's export",
		Long: "Reads a file another password manager produced and stores every login it carries as a Secret.\n" +
			"Import only ever adds: an existing Secret is never overwritten, and a title that would collide\n" +
			"is numbered instead. The whole file is checked before anything is written.\n\n" +
			"The source file is left alone; delete it yourself once the import looks right.",
		Args: cobra.ExactArgs(1),
		RunE: runImportSecrets,
	}
	cmd.Flags().StringVar(&importSecretsFrom, "from", "",
		"source format, for an export this build does not recognise (known: "+
			strings.Join(importer.Names(), ", ")+")")
	cmd.Flags().BoolVar(&importSecretsDryRun, "dry-run", false,
		"show what would be imported and stop without writing anything")
	return cmd
}

func runImportSecrets(cmd *cobra.Command, args []string) error {
	loc, err := keys.Locate(directory)
	if err != nil {
		return err
	}

	data, err := os.ReadFile(args[0])
	if err != nil {
		return fmt.Errorf("could not read %s: %w", args[0], err)
	}
	format, err := resolveFormat(data)
	if err != nil {
		return err
	}

	store, err := importStore(loc)
	if err != nil {
		return err
	}

	plan, err := importer.Build(format, data, store)
	if err != nil {
		return err
	}

	out := cmd.OutOrStdout()
	if importSecretsDryRun {
		reportPlan(out, format, args[0], plan)
		fmt.Fprintln(out)
		fmt.Fprintln(out, "Nothing was written. Run again without --dry-run to import.")
		return nil
	}

	imported, applyErr := plan.Apply(store)
	reportPlan(out, format, args[0], plan)
	fmt.Fprintf(out, "Imported   : %d of %d into %s\n", imported, len(plan.Secrets), store.Dir())
	if applyErr != nil {
		// Whatever was written is correct and stays; only the run stopped
		// (docs/adr/0012).
		return fmt.Errorf("import stopped after %d secrets: %w", imported, applyErr)
	}
	fmt.Fprintln(out)
	fmt.Fprintf(out, "%s still holds these passwords in plain text. Delete it when you are done.\n", args[0])
	return nil
}

// resolveFormat picks the Source Format: the one the user named with --from,
// or the one that recognises the file.
func resolveFormat(data []byte) (importer.Format, error) {
	if importSecretsFrom != "" {
		return importer.FormatNamed(importSecretsFrom)
	}
	f, err := importer.Detect(data)
	if err != nil {
		return nil, fmt.Errorf("%w; name it with --from if you know what it is", err)
	}
	return f, nil
}

// importStore opens the Vault an import writes into. A real import asks for
// the Master Password — adding hundreds of Secrets is an act of ownership, not
// of folder access — while a dry run, which writes nothing, needs no proof of
// ownership at all: the Recipient alone lists the slugs already taken
// (docs/adr/0012).
func importStore(loc keys.Location) (vault.Vault, error) {
	if importSecretsDryRun {
		session, err := loc.Recipient()
		if err != nil {
			return nil, err
		}
		return loc.Vault(session)
	}
	if err := loc.RequireIdentity(); err != nil {
		return nil, err
	}
	password, err := askPassword("Master password: ")
	if err != nil {
		return nil, err
	}
	return loc.Unlock(password)
}

// reportPlan prints what the import found, naming every title it had to shift
// so the user can tidy them up afterwards (docs/adr/0012).
func reportPlan(out io.Writer, f importer.Format, path string, plan *importer.Plan) {
	fmt.Fprintf(out, "Source     : %s (%s)\n", path, f.Name())
	fmt.Fprintf(out, "Secrets    : %d\n", len(plan.Secrets))
	fmt.Fprintf(out, "Renamed    : %d\n", len(plan.Renamed))
	for _, r := range plan.Renamed {
		fmt.Fprintf(out, "             %q is already taken, stored as %q\n", r.From, r.To)
	}
}
