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
		Short: "Membuat keypair dan konfigurasi awal",
		Long: "Membuat Identity (private key terenkripsi master password) dan Recipient (public key),\n" +
			"lalu menulis konfigurasi yang menunjuk ke keduanya.",
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
		return fmt.Errorf("%s sudah ada; hapus sendiri jika memang ingin memulai dari nol", cfgPath)
	}

	out := cmd.OutOrStdout()
	fmt.Fprintln(out, "Master password melindungi private key yang membuka semua secret.")
	fmt.Fprintln(out, "Tidak ada cara memulihkannya: kehilangan private key atau lupa password")
	fmt.Fprintln(out, "berarti semua secret hilang permanen.")
	fmt.Fprintln(out)

	password, err := askPassword("Master password: ")
	if err != nil {
		return err
	}
	if len(password) < 8 {
		return fmt.Errorf("master password minimal 8 karakter")
	}
	ulang, err := askPassword("Ulangi master password: ")
	if err != nil {
		return err
	}
	if password != ulang {
		return fmt.Errorf("master password tidak sama")
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
	fmt.Fprintf(out, "Konfigurasi: %s\n", cfgPath)
	fmt.Fprintln(out)
	fmt.Fprintln(out, "Cadangkan file identity sekarang. Tanpa file itu, secret tidak dapat dibuka lagi.")
	fmt.Fprintln(out, "Jalankan `gopm` di folder mana pun untuk mulai menyimpan secret.")
	return nil
}

// askPassword membaca password tanpa menampilkannya. Jika input bukan
// terminal, pembacaan ditolak — password tidak boleh datang dari pipe yang
// gampang tersimpan di riwayat atau log.
func askPassword(prompt string) (string, error) {
	fd := int(os.Stdin.Fd())
	if !term.IsTerminal(fd) {
		return "", fmt.Errorf("master password harus diketik di terminal")
	}
	fmt.Fprint(os.Stderr, prompt)
	b, err := term.ReadPassword(fd)
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return "", fmt.Errorf("gagal membaca master password: %w", err)
	}
	return strings.TrimSpace(string(b)), nil
}
