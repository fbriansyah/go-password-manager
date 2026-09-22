package importer_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/fbriansyah/go-password-manager/internal/importer"
	"github.com/fbriansyah/go-password-manager/internal/secret"
	"github.com/fbriansyah/go-password-manager/internal/vault"
)

// nopCipher stores plaintext: these tests are about mapping and planning, not
// encryption.
type nopCipher struct{}

func (nopCipher) Encrypt(plaintext []byte) ([]byte, error)  { return plaintext, nil }
func (nopCipher) Decrypt(ciphertext []byte) ([]byte, error) { return ciphertext, nil }

func emptyVault() *vault.Mem { return vault.NewMem(nopCipher{}) }

const onePasswordHeader = "Title,Url,Username,Password,OTPAuth,Favorite,Archived,Tags,Notes\n"

// onePasswordCSV builds an export with the header every 1Password CSV carries.
func onePasswordCSV(rows ...string) []byte {
	out := onePasswordHeader
	for _, r := range rows {
		out += r + "\n"
	}
	return []byte(out)
}

// importInto runs the whole path — detect, plan, apply — the way the command
// does, and fails the test at the first step that will not.
func importInto(t *testing.T, v vault.Vault, data []byte) *importer.Plan {
	t.Helper()
	f, err := importer.Detect(data)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	plan, err := importer.Build(f, data, v)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if _, err := plan.Apply(v); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	return plan
}

func TestImportsOneRowAsOneSecret(t *testing.T) {
	v := emptyVault()
	importInto(t, v, onePasswordCSV(
		`GitHub,https://github.com,octocat,hunter2,otpauth://totp/?secret=JBHB4LQQCADBOM3P,false,false,work,be careful`,
	))

	got, err := v.Load("github")
	if err != nil {
		t.Fatalf("Load(github): %v", err)
	}
	if got.Meta.Title != "GitHub" {
		t.Errorf("title = %q, want %q", got.Meta.Title, "GitHub")
	}
	want := []secret.Field{
		{Type: "tx", Label: "Username", Value: "octocat"},
		{Type: "ps", Label: "Password", Value: "hunter2"},
		{Type: "tp", Label: "TOTP", Value: "otpauth://totp/?secret=JBHB4LQQCADBOM3P"},
		{Type: "tx", Label: "Website", Value: "https://github.com"},
		{Type: "ta", Label: "Notes", Value: "be careful"},
	}
	if len(got.Fields) != len(want) {
		t.Fatalf("got %d fields, want %d: %+v", len(got.Fields), len(want), got.Fields)
	}
	for i, w := range want {
		if got.Fields[i] != w {
			t.Errorf("field %d = %+v, want %+v", i, got.Fields[i], w)
		}
	}
}

func TestEmptyColumnProducesNoField(t *testing.T) {
	v := emptyVault()
	importInto(t, v, onePasswordCSV(`Wifi,,,hunter2,,false,false,,`))

	got, err := v.Load("wifi")
	if err != nil {
		t.Fatalf("Load(wifi): %v", err)
	}
	want := []secret.Field{{Type: "ps", Label: "Password", Value: "hunter2"}}
	if len(got.Fields) != len(want) || got.Fields[0] != want[0] {
		t.Errorf("fields = %+v, want %+v", got.Fields, want)
	}
}

func TestTagsAreSplitAndFlagsBecomeTags(t *testing.T) {
	v := emptyVault()
	importInto(t, v, onePasswordCSV(
		`Fastmail,,,hunter2,,true,false,work;cloud,`,
		`Old Thing,,,hunter2,,false,true,,`,
		`Plain,,,hunter2,,false,false,,`,
	))

	for _, tc := range []struct {
		slug string
		want []string
	}{
		{"fastmail", []string{"work", "cloud", "favorite"}},
		{"old-thing", []string{"archived"}},
		{"plain", nil},
	} {
		got, err := v.Load(tc.slug)
		if err != nil {
			t.Fatalf("Load(%s): %v", tc.slug, err)
		}
		if len(got.Meta.Tags) != len(tc.want) {
			t.Errorf("%s tags = %v, want %v", tc.slug, got.Meta.Tags, tc.want)
			continue
		}
		for i, w := range tc.want {
			if got.Meta.Tags[i] != w {
				t.Errorf("%s tags = %v, want %v", tc.slug, got.Meta.Tags, tc.want)
				break
			}
		}
	}
}

func TestMultiLineNotesSurviveAsOneField(t *testing.T) {
	v := emptyVault()
	importInto(t, v, onePasswordCSV(
		"Router,,,hunter2,,false,false,,\"line one\nline two\nline three\"",
	))

	got, err := v.Load("router")
	if err != nil {
		t.Fatalf("Load(router): %v", err)
	}
	want := secret.Field{Type: "ta", Label: "Notes", Value: "line one\nline two\nline three"}
	if len(got.Fields) != 2 || got.Fields[1] != want {
		t.Errorf("fields = %+v, want a Password and %+v", got.Fields, want)
	}
}

func TestCollidingTitlesAreSuffixedAndReported(t *testing.T) {
	v := emptyVault()
	plan := importInto(t, v, onePasswordCSV(
		`Packtpub,,alice,one,,false,false,,`,
		`Packtpub,,bob,two,,false,false,,`,
		`Packtpub,,carol,three,,false,false,,`,
	))

	for slug, wantTitle := range map[string]string{
		"packtpub":   "Packtpub",
		"packtpub-2": "Packtpub 2",
		"packtpub-3": "Packtpub 3",
	} {
		got, err := v.Load(slug)
		if err != nil {
			t.Fatalf("Load(%s): %v", slug, err)
		}
		if got.Meta.Title != wantTitle {
			t.Errorf("%s title = %q, want %q", slug, got.Meta.Title, wantTitle)
		}
	}

	want := []importer.Rename{
		{From: "Packtpub", To: "Packtpub 2"},
		{From: "Packtpub", To: "Packtpub 3"},
	}
	if len(plan.Renamed) != len(want) {
		t.Fatalf("renamed = %+v, want %+v", plan.Renamed, want)
	}
	for i, w := range want {
		if plan.Renamed[i] != w {
			t.Errorf("renamed[%d] = %+v, want %+v", i, plan.Renamed[i], w)
		}
	}
}

func TestImportOnlyAddsNeverOverwrites(t *testing.T) {
	v := emptyVault()
	csv := onePasswordCSV(
		`GitHub,,alice,one,,false,false,,`,
		`Fastmail,,bob,two,,false,false,,`,
	)
	importInto(t, v, csv)
	plan := importInto(t, v, csv)

	slugs, err := v.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	want := []string{"fastmail", "fastmail-2", "github", "github-2"}
	if len(slugs) != len(want) {
		t.Fatalf("slugs = %v, want %v", slugs, want)
	}
	for i, w := range want {
		if slugs[i] != w {
			t.Fatalf("slugs = %v, want %v", slugs, want)
		}
	}
	if len(plan.Renamed) != 2 {
		t.Errorf("second import renamed %+v, want both titles shifted", plan.Renamed)
	}

	// The first import's Secrets are untouched by the second.
	got, err := v.Load("github")
	if err != nil {
		t.Fatalf("Load(github): %v", err)
	}
	if got.Meta.Title != "GitHub" {
		t.Errorf("title = %q, want the first import's %q", got.Meta.Title, "GitHub")
	}
}

// A file is either imported whole or not at all: phase one validates
// everything before phase two writes anything (docs/adr/0012).
func TestOneBadRowRefusesTheWholeFile(t *testing.T) {
	for _, tc := range []struct {
		name string
		rows []string
	}{
		{"unparseable TOTP seed", []string{
			`Good,,alice,one,,false,false,,`,
			`Bad,,bob,two,not-a-seed,false,false,,`,
		}},
		{"empty title", []string{
			`Good,,alice,one,,false,false,,`,
			`,,bob,two,,false,false,,`,
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v := emptyVault()
			f, err := importer.Detect(onePasswordCSV(tc.rows...))
			if err != nil {
				t.Fatalf("Detect: %v", err)
			}
			if _, err := importer.Build(f, onePasswordCSV(tc.rows...), v); err == nil {
				t.Fatal("Build accepted the file, want a refusal")
			}
			slugs, err := v.List()
			if err != nil {
				t.Fatalf("List: %v", err)
			}
			if len(slugs) != 0 {
				t.Errorf("vault holds %v, want nothing written", slugs)
			}
		})
	}
}

func TestDetectRefusesAnUnknownFile(t *testing.T) {
	for _, data := range [][]byte{
		[]byte("name,login_uri,login_username,login_password\nGitHub,,alice,one\n"),
		[]byte("not a csv at all"),
		nil,
	} {
		if f, err := importer.Detect(data); err == nil {
			t.Errorf("Detect(%q) = %s, want a refusal", data, f.Name())
		} else if !errors.Is(err, importer.ErrUnknownFormat) {
			t.Errorf("Detect(%q) error = %v, want ErrUnknownFormat", data, err)
		}
	}
}

func TestFormatNamedOverridesDetection(t *testing.T) {
	f, err := importer.FormatNamed("1password")
	if err != nil {
		t.Fatalf("FormatNamed(1password): %v", err)
	}
	if f.Name() != "1password" {
		t.Errorf("name = %q, want %q", f.Name(), "1password")
	}

	// A drifted header Detect would not recognise still reads when the user
	// names the format.
	drifted := []byte("Title,Url,Username,Password,OTPAuth,Favorite,Archived,Tags,Notes,Extra\n" +
		"GitHub,,alice,one,,false,false,,,\n")
	if _, err := importer.Detect(drifted); err == nil {
		t.Fatal("Detect accepted the drifted header; the override would be pointless")
	}

	_, err = importer.FormatNamed("bitwarden")
	if err == nil {
		t.Fatal("FormatNamed(bitwarden) succeeded, want a refusal")
	}
	if !strings.Contains(err.Error(), "1password") {
		t.Errorf("error %q does not name the formats gopm knows", err)
	}
}

// failingVault is a Vault whose disk gives out after failAfter Creates.
type failingVault struct {
	*vault.Mem
	failAfter int
	created   int
}

func (v *failingVault) Create(s *secret.Secret) (string, error) {
	if v.created >= v.failAfter {
		return "", errors.New("no space left on device")
	}
	v.created++
	return v.Mem.Create(s)
}

// A write that fails part way through stops and reports how far it got. It
// does not roll back: those Secrets are correct, and deleting them is the
// more dangerous move (docs/adr/0012).
func TestApplyStopsAtAFailedWriteAndKeepsWhatItWrote(t *testing.T) {
	v := &failingVault{Mem: emptyVault(), failAfter: 2}
	data := onePasswordCSV(
		`One,,alice,a,,false,false,,`,
		`Two,,bob,b,,false,false,,`,
		`Three,,carol,c,,false,false,,`,
		`Four,,dave,d,,false,false,,`,
	)
	f, err := importer.Detect(data)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	plan, err := importer.Build(f, data, v)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	imported, err := plan.Apply(v)
	if err == nil {
		t.Fatal("Apply succeeded, want the write failure reported")
	}
	if imported != 2 {
		t.Errorf("imported = %d, want 2 — the count before the failure", imported)
	}
	slugs, err := v.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(slugs) != 2 {
		t.Errorf("vault holds %v, want the two Secrets that were written kept", slugs)
	}
}

func TestApplyReportsHowManyItStored(t *testing.T) {
	v := emptyVault()
	data := onePasswordCSV(
		`One,,alice,a,,false,false,,`,
		`Two,,bob,b,,false,false,,`,
	)
	f, _ := importer.Detect(data)
	plan, err := importer.Build(f, data, v)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	imported, err := plan.Apply(v)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if imported != 2 {
		t.Errorf("imported = %d, want 2", imported)
	}
}

// A title arriving with trailing space must not gain a double space when it
// is suffixed. The title crosses unchanged; only the suffix is ours.
func TestSuffixDoesNotDoubleASpace(t *testing.T) {
	v := emptyVault()
	plan := importInto(t, v, onePasswordCSV(
		`[STG] Merchant Portal ,,alice,one,,false,false,,`,
		`[STG] Merchant Portal ,,bob,two,,false,false,,`,
	))

	if len(plan.Renamed) != 1 {
		t.Fatalf("renamed = %+v, want one shift", plan.Renamed)
	}
	if got, want := plan.Renamed[0].To, "[STG] Merchant Portal 2"; got != want {
		t.Errorf("renamed to %q, want %q", got, want)
	}
}

// --from exists for an export whose header has drifted, so the named Format
// must actually read one: columns are found by name, not by position.
func TestNamedFormatReadsADriftedHeader(t *testing.T) {
	f, err := importer.FormatNamed("1password")
	if err != nil {
		t.Fatalf("FormatNamed: %v", err)
	}

	for _, tc := range []struct {
		name string
		csv  string
	}{
		{"an extra column", "Title,Url,Username,Password,OTPAuth,Favorite,Archived,Tags,Notes,Extra\n" +
			"GitHub,https://github.com,octocat,hunter2,,false,false,work,note,ignored\n"},
		{"columns reordered", "Notes,Password,Title,Username,Tags,Url,Favorite,Archived,OTPAuth\n" +
			"note,hunter2,GitHub,octocat,work,https://github.com,false,false,\n"},
		{"a column missing", "Title,Url,Username,Password,Favorite,Archived,Tags\n" +
			"GitHub,https://github.com,octocat,hunter2,false,false,work\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v := emptyVault()
			plan, err := importer.Build(f, []byte(tc.csv), v)
			if err != nil {
				t.Fatalf("Build: %v", err)
			}
			if _, err := plan.Apply(v); err != nil {
				t.Fatalf("Apply: %v", err)
			}
			got, err := v.Load("github")
			if err != nil {
				t.Fatalf("Load(github): %v", err)
			}
			if got.Meta.Title != "GitHub" {
				t.Errorf("title = %q, want %q", got.Meta.Title, "GitHub")
			}
			if len(got.Meta.Tags) != 1 || got.Meta.Tags[0] != "work" {
				t.Errorf("tags = %v, want [work]", got.Meta.Tags)
			}
			if got.Fields[1].Value != "hunter2" {
				t.Errorf("password = %q, want %q", got.Fields[1].Value, "hunter2")
			}
		})
	}
}

// A file with no Title column cannot produce Secrets at all, and says so
// rather than importing a pile of untitled rows.
func TestNamedFormatRefusesAFileWithNoTitle(t *testing.T) {
	f, _ := importer.FormatNamed("1password")
	_, err := importer.Build(f, []byte("Url,Username,Password\nx,alice,one\n"), emptyVault())
	if err == nil {
		t.Fatal("Build accepted a file with no Title column")
	}
	if !strings.Contains(err.Error(), "Title") {
		t.Errorf("error %q does not name the missing column", err)
	}
}
