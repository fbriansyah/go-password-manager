package main

import (
	"fmt"
	"os"

	"github.com/fbriansyah/go-password-manager/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "gopm:", err)
		os.Exit(1)
	}
}
