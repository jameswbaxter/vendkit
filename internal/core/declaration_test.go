// Ported from tests/test_units.py — declaration surface, adapters, seed,
// profile export slice (export-declaration spec, DR-0009, DR-0013).

package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// declFixture mirrors the Python _decl helper: a docs/ tree plus an export
// declaration, with `extra` appended to the YAML.
func declFixture(t *testing.T, extra string) *ExportDecl {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(rel, body string) {
		if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(rel)),
			[]byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("docs/a.md", "# a\n")
	write("docs/TEMPLATE.md", "t\n")
	yaml := `schema_version: 1
slice: {name: docs, title: Docs}
publisher: {scm: github, repo: example-org/pub}
include: ["docs/**/*.md"]
exclude: ["**/TEMPLATE.md"]
` + extra + "\n"
	declPath := filepath.Join(root, "vendkit-export.yml")
	if err := os.WriteFile(declPath, []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
	decl, err := LoadExportDecl(declPath)
	if err != nil {
		t.Fatalf("LoadExportDecl: %v", err)
	}
	return decl
}

// declFixtureErr loads the declaration expecting a UsageError whose message
// contains `want`.
func declFixtureErr(t *testing.T, extra, want string) {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(root, "docs", "a.md"), []byte("# a\n"), 0o644)
	os.WriteFile(filepath.Join(root, "docs", "TEMPLATE.md"), []byte("t\n"), 0o644)
	yaml := `schema_version: 1
slice: {name: docs, title: Docs}
publisher: {scm: github, repo: example-org/pub}
include: ["docs/**/*.md"]
exclude: ["**/TEMPLATE.md"]
` + extra + "\n"
	declPath := filepath.Join(root, "vendkit-export.yml")
	os.WriteFile(declPath, []byte(yaml), 0o644)
	_, err := LoadExportDecl(declPath)
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Fatalf("LoadExportDecl error = %v, want containing %q", err, want)
	}
}

func rootOf(d *ExportDecl) string { return filepath.Dir(d.Path) }

func TestDeclarationSurface(t *testing.T) {
	decl := declFixture(t, "")
	got, err := decl.ExportedFiles(rootOf(decl))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != "docs/a.md" {
		t.Errorf("ExportedFiles = %v, want [docs/a.md]", got)
	}
}

func TestUnknownAdapterKindIsHardError(t *testing.T) {
	declFixtureErr(t, "adapters: [{kind: mystery, match: '*'}]", "unknown kind")
}

func TestUnknownTopLevelKeyRejected(t *testing.T) {
	declFixtureErr(t, "surprise: true", "unknown top-level key")
}

func TestPrefixNamespaceConsumerPath(t *testing.T) {
	decl := declFixture(t,
		"adapters: [{kind: prefix-namespace, match: 'docs/*.md', prefix: 'vnd-'}]")
	if got, _ := decl.ConsumerPath("docs/a.md"); got != "docs/vnd-a.md" {
		t.Errorf("ConsumerPath(docs/a.md) = %q, want docs/vnd-a.md", got)
	}
	if got, _ := decl.ConsumerPath("docs/vnd-a.md"); got != "docs/vnd-a.md" {
		t.Errorf("ConsumerPath must be idempotent, got %q", got)
	}
}

func TestGlobLocalisePrunesOtherProfiles(t *testing.T) {
	decl := declFixture(t, `adapters:
  - kind: glob-localise
    match: "docs/*.md"
    field: applyTo
    catalogue:
      code: ["src/**"]
      docs-only: ["manuals/**"]
profiles: {code: {}, docs-only: {}}`)
	body := []byte("---\napplyTo: \"src/**, manuals/**, **/*.py\"\n---\nx\n")
	out, err := ApplyAdapters(decl, "docs/a.md", body, "code")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), `applyTo: "src/**, **/*.py"`) {
		t.Errorf("other-profile glob not dropped: %s", out)
	}
	unbound, err := ApplyAdapters(decl, "docs/a.md", body, "")
	if err != nil {
		t.Fatal(err)
	}
	if string(unbound) != string(body) {
		t.Errorf("unbound consumer must keep the union: %s", unbound)
	}
}

func TestGlobLocalisePrunesBlockList(t *testing.T) {
	decl := declFixture(t, `adapters:
  - kind: glob-localise
    match: ".claude/rules/*.md"
    field: paths
    catalogue:
      code-repo: ["docs/specifications/**", "docs/standards/**"]
      solution-docs: ["docs/applications/**", "docs/domain/**"]
profiles: {code-repo: {}, solution-docs: {}}`)
	// The current rule shape: `paths:` as a YAML block list (not a comma string).
	body := []byte("---\ndescription: x\npaths:\n" +
		"  - docs/**\n" + // owned by no profile -> universal, always kept
		"  - docs/specifications/**\n" +
		"  - docs/standards/**\n" +
		"  - docs/applications/**\n" +
		"  - docs/domain/**\n---\nbody\n")
	out, err := ApplyAdapters(decl, ".claude/rules/a.md", body, "code-repo")
	if err != nil {
		t.Fatal(err)
	}
	got := string(out)
	for _, kept := range []string{"- docs/**", "- docs/specifications/**", "- docs/standards/**"} {
		if !strings.Contains(got, kept) {
			t.Errorf("glob %q should be kept:\n%s", kept, got)
		}
	}
	for _, pruned := range []string{"docs/applications/**", "docs/domain/**"} {
		if strings.Contains(got, pruned) {
			t.Errorf("other-profile glob %q should be pruned:\n%s", pruned, got)
		}
	}
	if !strings.Contains(got, "description: x") || !strings.HasSuffix(got, "---\nbody\n") {
		t.Errorf("front-matter/body structure must be preserved:\n%s", got)
	}
	unbound, err := ApplyAdapters(decl, ".claude/rules/a.md", body, "")
	if err != nil {
		t.Fatal(err)
	}
	if string(unbound) != string(body) {
		t.Errorf("unbound consumer must keep the union verbatim:\n%s", unbound)
	}
}

func TestSeedSurfaceAndOverlapError(t *testing.T) {
	// Seed surface: templates/notes.md is seeded, docs/a.md is exported.
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "docs"), 0o755)
	os.MkdirAll(filepath.Join(root, "templates"), 0o755)
	os.WriteFile(filepath.Join(root, "docs", "a.md"), []byte("# a\n"), 0o644)
	os.WriteFile(filepath.Join(root, "docs", "TEMPLATE.md"), []byte("t\n"), 0o644)
	os.WriteFile(filepath.Join(root, "templates", "notes.md"), []byte("starter\n"), 0o644)
	writeDecl := func(extra string) string {
		yaml := `schema_version: 1
slice: {name: docs, title: Docs}
publisher: {scm: github, repo: example-org/pub}
include: ["docs/**/*.md"]
exclude: ["**/TEMPLATE.md"]
` + extra + "\n"
		p := filepath.Join(root, "vendkit-export.yml")
		os.WriteFile(p, []byte(yaml), 0o644)
		return p
	}

	decl, err := LoadExportDecl(writeDecl(`seed: ["templates/*.md"]`))
	if err != nil {
		t.Fatal(err)
	}
	seeded, err := decl.SeededFiles(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(seeded) != 1 || seeded[0] != "templates/notes.md" {
		t.Errorf("SeededFiles = %v, want [templates/notes.md]", seeded)
	}
	exported, err := decl.ExportedFiles(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(exported) != 1 || exported[0] != "docs/a.md" {
		t.Errorf("ExportedFiles = %v, want [docs/a.md]", exported)
	}

	// A path matched by both include and seed is a hard error (DR-0013).
	overlapping, err := LoadExportDecl(writeDecl(`seed: ["docs/a.md"]`))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := overlapping.ExportedFiles(root); err == nil ||
		!strings.Contains(err.Error(), "both include and seed") {
		t.Errorf("overlap error = %v, want 'both include and seed'", err)
	}
}

func TestSeedRespectsPrefixNamespace(t *testing.T) {
	decl := declFixture(t, `seed: ["templates/*.md"]
adapters: [{kind: prefix-namespace, match: 'templates/*.md', prefix: 'vnd-'}]`)
	if got, _ := decl.ConsumerPath("templates/notes.md"); got != "templates/vnd-notes.md" {
		t.Errorf("ConsumerPath(templates/notes.md) = %q, want templates/vnd-notes.md", got)
	}
}

func TestProfileExportSlice(t *testing.T) {
	decl := declFixture(t, `profiles:
  lean:
    export_slice: {include: ["docs/*"], exclude: ["docs/a.md"]}`)
	if decl.ProfileInScope("lean", "docs/a.md") {
		t.Error("docs/a.md must be out of scope for the lean profile")
	}
	if !decl.ProfileInScope("", "docs/a.md") {
		t.Error("an unbound consumer takes the whole surface")
	}
}
