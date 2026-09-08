// Package secret defines the shape of a Secret and how it is serialised.
// This package knows nothing about encryption, the filesystem, or the TUI.
package secret

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// Meta describes a Secret: it is used to find one, never as a credential
// itself.
type Meta struct {
	Title       string   `json:"title"`
	Description string   `json:"description,omitempty"`
	Tags        []string `json:"tags,omitempty"`
}

// Field is one label/value pair inside a Secret. Type points at the Field Type
// that decides how the value is rendered, edited, and copied.
type Field struct {
	Type  string `json:"type"`
	Label string `json:"label"`
	Value string `json:"value"`
}

// Secret is one set of credentials: Meta plus a series of Fields.
type Secret struct {
	Meta   Meta    `json:"meta"`
	Fields []Field `json:"fields"`
}

// ErrEmptyTitle is returned when a Secret has no title; the title is required
// because the Slug is derived from it.
var ErrEmptyTitle = errors.New("a secret needs a title")

// Validate checks that a Secret is complete enough to be stored.
func (s *Secret) Validate() error {
	if strings.TrimSpace(s.Meta.Title) == "" {
		return ErrEmptyTitle
	}
	for i, f := range s.Fields {
		if strings.TrimSpace(f.Label) == "" {
			return fmt.Errorf("field %d has no label", i+1)
		}
		if f.Type == "" {
			return fmt.Errorf("field %q has no type", f.Label)
		}
	}
	return nil
}

// Marshal produces the JSON form that gets encrypted.
func Marshal(s *Secret) ([]byte, error) {
	if s.Fields == nil {
		s.Fields = []Field{}
	}
	return json.MarshalIndent(s, "", "  ")
}

// Unmarshal reads back the JSON form Marshal produced. Fields with unknown
// types are loaded as they are; see TypeFor.
func Unmarshal(data []byte) (*Secret, error) {
	var s Secret
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("secret is unreadable: %w", err)
	}
	if s.Fields == nil {
		s.Fields = []Field{}
	}
	return &s, nil
}
