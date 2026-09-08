// Package cmd menyusun antarmuka baris perintah.
package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/fbriansyah/go-password-manager/internal/config"
	"github.com/fbriansyah/go-password-manager/internal/tui"
)

var directory string

// Execute menjalankan aplikasi.
func Execute() error {
	root := &cobra.Command{
		Use:   "gopm",
		Short: "Password manager berbasis file untuk folder kerja",
		Long: "gopm menyimpan kredensial sebagai file terenkripsi di dalam folder tempat ia dijalankan.\n" +
			"Gunakan -d untuk menunjuk folder lain tanpa berpindah direktori.",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE:          runTUI,
	}
	root.PersistentFlags().StringVarP(&directory, "directory", "d", "",
		"folder vault (default: folder kerja saat ini)")
	root.AddCommand(initCmd(), clipboardClearCmd())
	return root.Execute()
}

// vaultDir menentukan folder Vault: nilai -d bila diberikan, selain itu folder
// kerja. Folder yang tidak ada ditolak, bukan dibuat diam-diam — folder salah
// ketik lebih baik gagal daripada menjadi Vault kosong yang baru.
func vaultDir() (string, error) {
	dir := directory
	if dir == "" {
		wd, err := os.Getwd()
		if err != nil {
			return "", fmt.Errorf("folder kerja tidak diketahui: %w", err)
		}
		dir = wd
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", fmt.Errorf("path vault tidak sah: %w", err)
	}
	info, err := os.Stat(abs)
	if errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("folder vault %s tidak ada", abs)
	}
	if err != nil {
		return "", fmt.Errorf("folder vault tidak dapat dibuka: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("%s bukan folder", abs)
	}
	return abs, nil
}

func runTUI(cmd *cobra.Command, _ []string) error {
	dir, err := vaultDir()
	if err != nil {
		return err
	}
	// Override .gopm.yaml dicari di folder Vault, bukan di $PWD, agar key
	// selalu mengikuti Secret yang dibuka (docs/adr/0001).
	cfg, err := config.Load(dir)
	if err != nil {
		return err
	}
	if _, err := os.Stat(cfg.PrivateKeyPath); err != nil {
		return fmt.Errorf("identity di %s tidak dapat dibuka: %w", cfg.PrivateKeyPath, err)
	}
	p := tea.NewProgram(tui.New(dir, cfg), tea.WithAltScreen())
	_, err = p.Run()
	return err
}
