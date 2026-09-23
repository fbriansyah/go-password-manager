// Package importer moves Secrets from another password manager into a Vault.
// It is split in two: a Source Format knows one file shape and nothing else,
// while this file holds everything that is true of every import — resolving
// titles, validating, and writing (docs/adr/0012).
package importer

import (
	"errors"
	"fmt"
	"strings"

	"github.com/fbriansyah/go-password-manager/internal/secret"
	"github.com/fbriansyah/go-password-manager/internal/vault"
)

// Format is one shape of file Import understands. It is handed the file's raw
// bytes, so this package never assumes any file shape of its own: a CSV
// format parses CSV, a future zip format opens a zip.
type Format interface {
	// Name is how a user asks for this format with --from.
	Name() string
	// Sniff reports whether this Format recognises the file.
	Sniff(data []byte) bool
	// Read turns the file into Secrets. It knows nothing about the Vault:
	// titles may collide and are left exactly as the file had them.
	Read(data []byte) ([]secret.Secret, error)
}

// formats is every Source Format this build understands, in the order Detect
// tries them and errors list them.
var formats = []Format{onePassword{}}

// ErrUnknownFormat is returned when no Source Format recognises a file.
var ErrUnknownFormat = errors.New("no known format recognises this file")

// Detect finds the Source Format that recognises data.
func Detect(data []byte) (Format, error) {
	for _, f := range formats {
		if f.Sniff(data) {
			return f, nil
		}
	}
	return nil, fmt.Errorf("%w; gopm knows %s", ErrUnknownFormat, strings.Join(Names(), ", "))
}

// FormatNamed returns the Source Format a user asked for by name. It is the
// way past Detect for an export whose header has drifted.
func FormatNamed(name string) (Format, error) {
	for _, f := range formats {
		if strings.EqualFold(f.Name(), name) {
			return f, nil
		}
	}
	return nil, fmt.Errorf("unknown format %q; gopm knows %s", name, strings.Join(Names(), ", "))
}

// Names lists every Source Format, for naming them in a message.
func Names() []string {
	names := make([]string, 0, len(formats))
	for _, f := range formats {
		names = append(names, f.Name())
	}
	return names
}

// Plan is everything an import would do, worked out before anything is
// written. Build produces one; Apply carries it out (docs/adr/0012).
type Plan struct {
	// Secrets are ready to store: validated, with titles already free of
	// collisions.
	Secrets []secret.Secret
	// Renamed lists every title the plan had to shift, so the report can name
	// them and the user can tidy up afterwards.
	Renamed []Rename
}

// Rename is one title an import had to shift to keep a Secret out of another
// one's file name.
type Rename struct{ From, To string }

// Build is the first phase: read the file, resolve every title against the
// Vault and against the rows before it, then check every Secret. It never
// writes, so a file it refuses leaves the Vault untouched (docs/adr/0012).
func Build(f Format, data []byte, v vault.Vault) (*Plan, error) {
	secrets, err := f.Read(data)
	if err != nil {
		return nil, err
	}

	taken, err := takenSlugs(v)
	if err != nil {
		return nil, err
	}

	plan := &Plan{Secrets: secrets}
	for i := range plan.Secrets {
		original := plan.Secrets[i].Meta.Title
		title, slug := free(original, taken)
		if title != original {
			plan.Secrets[i].Meta.Title = title
			plan.Renamed = append(plan.Renamed, Rename{From: original, To: title})
		}
		taken[slug] = true
		if err := plan.Secrets[i].Validate(); err != nil {
			return nil, fmt.Errorf("%q: %w", original, err)
		}
	}
	return plan, nil
}

// takenSlugs reads the file names already in the Vault. It needs no
// decryption: a slug is the one part of a Secret readable without unlocking
// (docs/adr/0004).
func takenSlugs(v vault.Vault) (map[string]bool, error) {
	slugs, err := v.List()
	if err != nil {
		return nil, err
	}
	taken := make(map[string]bool, len(slugs))
	for _, s := range slugs {
		taken[s] = true
	}
	return taken, nil
}

// free finds the first title starting from want whose slug nobody has claimed,
// counting up — "Packtpub", "Packtpub 2", "Packtpub 3". The title is what
// moves, not only the file name, so what the list shows matches what is on
// disk (docs/adr/0012).
func free(want string, taken map[string]bool) (title, slug string) {
	for n := 1; ; n++ {
		title = want
		if n > 1 {
			// The suffix joins onto the trimmed title: a source title with a
			// trailing space is stored as it came, but must not grow a double
			// space when this counter is added to it.
			title = fmt.Sprintf("%s %d", strings.TrimRight(want, " "), n)
		}
		if slug = vault.Slug(title); !taken[slug] {
			return title, slug
		}
	}
}

// Apply is the second phase: store every Secret. It returns how many were
// stored, which on failure is how far it got — a partly applied Plan is
// reported, never rolled back (docs/adr/0012).
func (p *Plan) Apply(v vault.Vault) (int, error) {
	for i := range p.Secrets {
		if _, err := v.Create(&p.Secrets[i]); err != nil {
			return i, fmt.Errorf("%q: %w", p.Secrets[i].Meta.Title, err)
		}
	}
	return len(p.Secrets), nil
}
