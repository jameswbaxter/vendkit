// Localisation validity checks and the expectation oracle (issue #10,
// export-declaration spec §3.1/§3.2).

package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// localisationFixture: a rules/ tree behind a glob-localise adapter.
//
//	rules/std.md   — paths: src/** (code), manuals/** (docs-only), typo/** (uncatalogued)
//	rules/code.md  — paths: src/** only (empty for every non-code profile)
//	rules/plain.md — no front matter (the adapter no-ops)
//
// Catalogue key `ghost` is not a declared profile and owns nowhere/**, which
// no rule carries (orphan). Declared profiles: code, docs-only, lonely.
func localisationFixture(t *testing.T) (*ExportDecl, string) {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "rules"), 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(rel, body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(rel)),
			[]byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("rules/std.md", "---\npaths:\n  - src/**\n  - manuals/**\n  - typo/**\n---\nstd\n")
	write("rules/code.md", "---\npaths:\n  - src/**\n---\ncode\n")
	write("rules/plain.md", "no front matter\n")
	yaml := `schema_version: 1
slice: {name: rules, title: Rules}
publisher: {scm: github, repo: example-org/pub}
include: ["rules/*.md"]
adapters:
  - kind: glob-localise
    match: "rules/*.md"
    field: paths
    catalogue:
      code: ["src/**"]
      docs-only: ["manuals/**"]
      ghost: ["nowhere/**"]
profiles: {code: {}, docs-only: {}, lonely: {}}
`
	declPath := filepath.Join(root, ".vendkit", "publisher", "export-declaration.yml")
	if err := os.MkdirAll(filepath.Dir(declPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(declPath, []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
	decl, err := LoadExportDecl(declPath)
	if err != nil {
		t.Fatalf("LoadExportDecl: %v", err)
	}
	return decl, root
}

func hasFinding(fs []LocalisationFinding, rule, path, profile, glob string) bool {
	for _, f := range fs {
		if f.Rule == rule && f.Path == path && f.Profile == profile && f.Glob == glob {
			return true
		}
	}
	return false
}

func TestFieldGlobsMatchesAdapterParse(t *testing.T) {
	inline := []byte("---\napplyTo: \"src/**, manuals/**\"\n---\nx\n")
	globs, found := FieldGlobs(inline, "applyTo")
	if !found || strings.Join(globs, "|") != "src/**|manuals/**" {
		t.Errorf("inline FieldGlobs = %v, %v", globs, found)
	}
	block := []byte("---\ndescription: x\npaths:\n  - a/**\n  - \"b/**\"\n---\nx\n")
	globs, found = FieldGlobs(block, "paths")
	if !found || strings.Join(globs, "|") != "a/**|b/**" {
		t.Errorf("block FieldGlobs = %v, %v", globs, found)
	}
	if _, found := FieldGlobs([]byte("no front matter\n"), "paths"); found {
		t.Error("FieldGlobs must report found=false without the field")
	}
	if globs, found := FieldGlobs([]byte("---\npaths:\n---\nx\n"), "paths"); !found || len(globs) != 0 {
		t.Errorf("empty block list: FieldGlobs = %v, %v; want [], true", globs, found)
	}
}

func TestCheckLocalisationFindings(t *testing.T) {
	decl, root := localisationFixture(t)
	findings, err := CheckLocalisation(decl, root)
	if err != nil {
		t.Fatal(err)
	}
	if !hasFinding(findings, "profile-unknown", "", "ghost", "") {
		t.Errorf("missing profile-unknown for ghost: %v", findings)
	}
	if !hasFinding(findings, "glob-uncatalogued", "rules/std.md", "", "typo/**") {
		t.Errorf("missing glob-uncatalogued for typo/**: %v", findings)
	}
	if !hasFinding(findings, "catalogue-glob-orphan", "", "", "nowhere/**") {
		t.Errorf("missing catalogue-glob-orphan for nowhere/**: %v", findings)
	}
	// rules/code.md carries only code-owned globs: empty for every other
	// profile that materialises (docs-only, ghost, lonely).
	for _, p := range []string{"docs-only", "ghost", "lonely"} {
		if !hasFinding(findings, "localisation-empty", "rules/code.md", p, "") {
			t.Errorf("missing localisation-empty for %s on rules/code.md: %v", p, findings)
		}
	}
	if hasFinding(findings, "localisation-empty", "rules/code.md", "code", "") {
		t.Errorf("code profile keeps src/** — must not be empty: %v", findings)
	}
	// rules/std.md keeps the universal typo/** everywhere: never empty.
	for _, f := range findings {
		if f.Rule == "localisation-empty" && f.Path == "rules/std.md" {
			t.Errorf("rules/std.md keeps a universal glob for every profile: %+v", f)
		}
	}
}

func TestCheckLocalisationCleanDeclaration(t *testing.T) {
	decl, root := localisationFixture(t)
	// Repair the fixture's deliberate defects: catalogue ghost's glob onto a
	// rule, declare ghost, catalogue typo/**, give code.md a universal glob.
	os.WriteFile(filepath.Join(root, "rules", "std.md"),
		[]byte("---\npaths:\n  - src/**\n  - manuals/**\n  - typo/**\n  - nowhere/**\n---\nstd\n"), 0o644)
	os.WriteFile(filepath.Join(root, "rules", "code.md"),
		[]byte("---\npaths:\n  - src/**\n  - typo/**\n---\ncode\n"), 0o644)
	decl.Profiles["ghost"] = Profile{Name: "ghost", ExportInclude: []string{"*"}}
	findings, err := CheckLocalisation(decl, root)
	if err != nil {
		t.Fatal(err)
	}
	var unexpected []LocalisationFinding
	for _, f := range findings {
		if f.Rule != "glob-uncatalogued" { // typo/** stays deliberately universal
			unexpected = append(unexpected, f)
		}
	}
	if len(unexpected) > 0 {
		t.Errorf("clean declaration still yields findings: %v", unexpected)
	}
}

func TestCheckLocalisationNoAdapterIsSilent(t *testing.T) {
	decl := declFixture(t, "")
	findings, err := CheckLocalisation(decl, rootOf(decl))
	if err != nil {
		t.Fatal(err)
	}
	if findings != nil {
		t.Errorf("no glob-localise adapter must mean no findings, got %v", findings)
	}
}

// -- the expectation oracle -------------------------------------------------------

func writeExpectations(t *testing.T, root, body string) string {
	t.Helper()
	p := filepath.Join(root, "expected-localisation.yml")
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestVerifyLocalisationClean(t *testing.T) {
	decl, root := localisationFixture(t)
	p := writeExpectations(t, root, `schema_version: 1
expectations:
  code:
    "rules/std.md": ["src/**", "typo/**"]
    "rules/code.md": ["src/**"]
`)
	exp, err := LoadLocalisationExpectations(p)
	if err != nil {
		t.Fatal(err)
	}
	report, err := VerifyLocalisation(decl, root, "", exp, "code")
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Findings) != 0 {
		t.Errorf("findings = %v, want none", report.Findings)
	}
	if report.Checked != 2 {
		t.Errorf("checked = %d, want 2", report.Checked)
	}
}

func TestVerifyLocalisationFindings(t *testing.T) {
	decl, root := localisationFixture(t)
	p := writeExpectations(t, root, `schema_version: 1
expectations:
  code:
    "rules/std.md": ["src/**"]
    "rules/gone.md": ["src/**"]
`)
	exp, err := LoadLocalisationExpectations(p)
	if err != nil {
		t.Fatal(err)
	}
	report, err := VerifyLocalisation(decl, root, "", exp, "code")
	if err != nil {
		t.Fatal(err)
	}
	// std.md actually keeps [src/**, typo/**] — the expectation catches the
	// under-prune the consumer would otherwise silently vendor.
	if !hasFinding(report.Findings, "mismatch", "rules/std.md", "code", "") {
		t.Errorf("missing mismatch for rules/std.md: %v", report.Findings)
	}
	if !hasFinding(report.Findings, "rule-absent", "rules/gone.md", "code", "") {
		t.Errorf("missing rule-absent for rules/gone.md: %v", report.Findings)
	}
	if !hasFinding(report.Findings, "expectation-stale", "rules/code.md", "code", "") {
		t.Errorf("missing expectation-stale for rules/code.md: %v", report.Findings)
	}
	if len(report.Findings) != 3 {
		t.Errorf("findings = %v, want exactly 3", report.Findings)
	}
}

// The 5e5d701 regression, end to end: an adapter that silently no-ops on the
// block-list shape ships the un-pruned union; the oracle is the instrument
// that catches it (unit tests of the adapter did not).
func TestVerifyLocalisationCatchesSilentNoOp(t *testing.T) {
	decl, root := localisationFixture(t)
	p := writeExpectations(t, root, `schema_version: 1
expectations:
  docs-only:
    "rules/std.md": ["manuals/**", "typo/**"]
`)
	exp, err := LoadLocalisationExpectations(p)
	if err != nil {
		t.Fatal(err)
	}
	clean, err := VerifyLocalisation(decl, root, "", exp, "docs-only")
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range clean.Findings {
		if f.Rule == "mismatch" {
			t.Fatalf("healthy adapter must match the expectation: %+v", f)
		}
	}
	// Simulate the no-op by verifying a consumer tree holding the RAW union
	// (what a broken localise() would have materialised).
	consumer := t.TempDir()
	if err := os.MkdirAll(filepath.Join(consumer, "rules"), 0o755); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(root, "rules", "std.md"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(consumer, "rules", "std.md"), raw, 0o644); err != nil {
		t.Fatal(err)
	}
	broken, err := VerifyLocalisation(decl, root, consumer, exp, "docs-only")
	if err != nil {
		t.Fatal(err)
	}
	if !hasFinding(broken.Findings, "mismatch", "rules/std.md", "docs-only", "") {
		t.Errorf("un-pruned union must mismatch the expectation: %v", broken.Findings)
	}
}

func TestVerifyLocalisationUnboundConsumerRefused(t *testing.T) {
	decl, root := localisationFixture(t)
	exp := &LocalisationExpectations{ByProfile: map[string]map[string][]string{}}
	_, err := VerifyLocalisation(decl, root, t.TempDir(), exp, "")
	if err == nil || !strings.Contains(err.Error(), "not bound to a profile") {
		t.Errorf("err = %v, want unbound-consumer usage error", err)
	}
}

func TestWriteExpectedLocalisationRoundTrip(t *testing.T) {
	decl, root := localisationFixture(t)
	p := filepath.Join(root, "expected-localisation.yml")
	entries, err := WriteExpectedLocalisation(decl, root, p)
	if err != nil {
		t.Fatal(err)
	}
	if entries == 0 {
		t.Fatal("no entries written")
	}
	exp, err := LoadLocalisationExpectations(p)
	if err != nil {
		t.Fatalf("written file must load: %v", err)
	}
	report, err := VerifyLocalisation(decl, root, "", exp, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Findings) != 0 {
		t.Errorf("freshly written expectations must verify clean: %v", report.Findings)
	}
	// The refreshed oracle now moves with the publisher, so a rule change
	// (here: a shelf the code profile does not own) is caught as a mismatch.
	if err := os.WriteFile(filepath.Join(root, "rules", "code.md"),
		[]byte("---\npaths:\n  - src/**\n  - manuals/**\n---\ncode\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	report, err = VerifyLocalisation(decl, root, "", exp, "docs-only")
	if err != nil {
		t.Fatal(err)
	}
	if !hasFinding(report.Findings, "mismatch", "rules/code.md", "docs-only", "") {
		t.Errorf("changed rule must mismatch the stored expectation: %v", report.Findings)
	}
}

func TestLoadLocalisationExpectationsRejectsBadShapes(t *testing.T) {
	root := t.TempDir()
	for body, want := range map[string]string{
		"schema_version: 2\nexpectations: {}\n":                             "schema_version must be 1",
		"schema_version: 1\nsurprise: true\n":                               "unknown top-level key",
		"schema_version: 1\nexpectations:\n  code: [x]\n":                   "must map paths",
		"schema_version: 1\nexpectations:\n  code:\n    \"a.md\": {x: 1}\n": "must be a list of glob strings",
	} {
		p := writeExpectations(t, root, body)
		if _, err := LoadLocalisationExpectations(p); err == nil ||
			!strings.Contains(err.Error(), want) {
			t.Errorf("body %q: err = %v, want containing %q", body, err, want)
		}
	}
}
