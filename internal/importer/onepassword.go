package importer

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"io"
	"strings"

	"github.com/fbriansyah/go-password-manager/internal/secret"
)

// onePassword reads the flat login CSV 1Password exports: nine columns, one
// row per login.
type onePassword struct{}

// onePasswordHeader is the exact first line a 1Password login CSV carries.
// Recognising the file by it means a user normally names no format at all.
var onePasswordHeader = []string{
	"Title", "Url", "Username", "Password", "OTPAuth",
	"Favorite", "Archived", "Tags", "Notes",
}

func (onePassword) Name() string { return "1password" }

// Sniff is strict: an exact header match, so a file is only claimed when
// there is no doubt. A header that has drifted is read anyway when the user
// names the format with --from — see Read.
func (onePassword) Sniff(data []byte) bool {
	head, err := csv.NewReader(bytes.NewReader(data)).Read()
	if err != nil || len(head) != len(onePasswordHeader) {
		return false
	}
	for i, want := range onePasswordHeader {
		if head[i] != want {
			return false
		}
	}
	return true
}

// Read maps each row to one Secret. Columns are found by name rather than by
// position, so an export that gained a column, lost one, or reordered them
// still reads — which is what makes --from worth having. Values cross
// unchanged: no URL is normalised and no otpauth URI is rewritten
// (docs/adr/0008, docs/adr/0012).
func (onePassword) Read(data []byte) ([]secret.Secret, error) {
	r := csv.NewReader(bytes.NewReader(data))
	r.FieldsPerRecord = -1 // the header decides the width, not this build
	head, err := r.Read()
	if err != nil {
		return nil, fmt.Errorf("could not read the header: %w", err)
	}
	col := columns(head)
	if _, ok := col["Title"]; !ok {
		return nil, fmt.Errorf("this file has no Title column, so it cannot name a Secret; it has %s",
			strings.Join(head, ", "))
	}

	var out []secret.Secret
	for line := 2; ; line++ {
		row, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", line, err)
		}
		at := func(name string) string {
			i, ok := col[name]
			if !ok || i >= len(row) {
				return ""
			}
			return row[i]
		}
		out = append(out, secret.Secret{
			Meta:   secret.Meta{Title: at("Title"), Tags: onePasswordTags(at)},
			Fields: onePasswordFields(at),
		})
	}
	return out, nil
}

// columns maps each header name to its position. A name appearing twice keeps
// the first column, which is the one a reader would take it to mean.
func columns(head []string) map[string]int {
	col := make(map[string]int, len(head))
	for i, name := range head {
		name = strings.TrimSpace(name)
		if _, seen := col[name]; !seen {
			col[name] = i
		}
	}
	return col
}

// onePasswordFields builds the Fields of one row, in the order a login is
// actually performed. A column with nothing in it — or missing from the file
// altogether — produces no Field at all.
func onePasswordFields(at func(string) string) []secret.Field {
	fields := make([]secret.Field, 0, 5)
	for _, f := range []secret.Field{
		{Type: "tx", Label: "Username", Value: at("Username")},
		{Type: "ps", Label: "Password", Value: at("Password")},
		{Type: "tp", Label: "TOTP", Value: at("OTPAuth")},
		{Type: "tx", Label: "Website", Value: at("Url")},
		{Type: "ta", Label: "Notes", Value: at("Notes")},
	} {
		if f.Value != "" {
			fields = append(fields, f)
		}
	}
	return fields
}

// onePasswordTags reads the Tags column, which separates tags with a
// semicolon, and appends the two flag columns as tags of their own — gopm has
// no concept of a favourite or an archived Secret, so the only way to carry
// them across is as the grouping Tags already are.
func onePasswordTags(at func(string) string) []string {
	var tags []string
	for _, t := range strings.Split(at("Tags"), ";") {
		if t = strings.TrimSpace(t); t != "" {
			tags = append(tags, t)
		}
	}
	for _, flag := range []struct {
		column string
		tag    string
	}{{"Favorite", "favorite"}, {"Archived", "archived"}} {
		if strings.EqualFold(strings.TrimSpace(at(flag.column)), "true") {
			tags = append(tags, flag.tag)
		}
	}
	return tags
}
