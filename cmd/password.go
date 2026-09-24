package cmd

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"unicode/utf8"

	"golang.org/x/term"
)

// minMasterPassword is how short a Master Password may be. It is unrelated to
// generator.ErrTooShort's floor, which happens to be the same number but
// governs the length of a generated password rather than the phrase that
// opens the Identity.
const minMasterPassword = 8

// askPassword reads a password without echoing it. If the input is not a
// terminal the read is refused — a password must not arrive through a pipe that
// is easily kept in shell history or logs.
//
// It is a var so a test can put a scripted prompt in its place; nothing
// changes it at runtime.
var askPassword = func(prompt string) (string, error) {
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

// askNewMasterPassword asks for a Master Password and its repeat, and refuses
// one that is too short, mistyped, or — when replacing is given — the same as
// the password it would replace. It is the single place that says what an
// acceptable Master Password is; `init` and `change-master-password` differ
// only in their wording and in whether there is a password being replaced.
//
// The rules are checked in the order the user meets them, so a password that
// is going to be refused is refused before the repeat is asked for rather than
// after it.
func askNewMasterPassword(label, repeatLabel, replacing string) (string, error) {
	password, err := askPassword(label)
	if err != nil {
		return "", err
	}
	// The refusal promises characters, so characters are what get counted: a
	// short passphrase written in a multi-byte script must not pass on the
	// strength of its byte length.
	if utf8.RuneCountInString(password) < minMasterPassword {
		return "", fmt.Errorf("the master password must be at least %d characters", minMasterPassword)
	}
	if replacing != "" && password == replacing {
		return "", errors.New("the new master password is the same as the current one")
	}
	again, err := askPassword(repeatLabel)
	if err != nil {
		return "", err
	}
	if password != again {
		return "", errors.New("the master passwords do not match")
	}
	return password, nil
}
