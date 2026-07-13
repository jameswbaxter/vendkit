package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeFixtureDocs lays down a tiny docs tree exercising headings, a fenced
// code block, an internal doc-to-doc link, and a GFM table.
func writeFixtureDocs(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "intro.md"), `# Introduction

Welcome to the fixture. See the [CLI spec](specs/cli.md#usage) for details,
and the [external site](https://example.com/x.md) which must not be rewritten.

`+"```go\nfmt.Println(\"hi\")\n```"+`

| Flag | Meaning |
|------|---------|
| --v  | version |
`)
	mustWrite(t, filepath.Join(dir, "specs", "cli.md"), `# Spec: CLI

The `+"`vendkit`"+` command. Back to the [intro](../intro.md).
`)
	return dir
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestGenerateAccumulatesVersions(t *testing.T) {
	docs := writeFixtureDocs(t)
	out := t.TempDir()

	for _, v := range []string{"v0.1.0", "v0.2.0"} {
		if err := Generate(Options{DocsDir: docs, OutDir: out, Version: v}); err != nil {
			t.Fatalf("Generate(%s): %v", v, err)
		}
	}

	// Both version dirs and the latest alias exist.
	for _, dir := range []string{"v0.1.0", "v0.2.0", "latest"} {
		if _, err := os.Stat(filepath.Join(out, dir, "index.html")); err != nil {
			t.Errorf("expected %s/index.html: %v", dir, err)
		}
	}

	page := readFile(t, filepath.Join(out, "v0.2.0", "intro.html"))

	// Rendered Markdown: heading and fenced code block.
	if !strings.Contains(page, "<h1") || !strings.Contains(page, "Introduction</h1>") {
		t.Error("rendered page missing <h1> heading")
	}
	if !strings.Contains(page, "<pre>") {
		t.Error("rendered page missing code block")
	}
	if !strings.Contains(page, "<table>") {
		t.Error("rendered page missing GFM table")
	}

	// Internal .md link rewritten to .html (fragment preserved); external left alone.
	if !strings.Contains(page, `href="specs/cli.html#usage"`) {
		t.Error("internal .md link was not rewritten to .html#usage")
	}
	if !strings.Contains(page, `href="https://example.com/x.md"`) {
		t.Error("external .md link should not have been rewritten")
	}

	// Version-selector markup present with the correct current version and
	// baked options newest-first.
	if !strings.Contains(page, `class="vk-vers"`) || !strings.Contains(page, `data-current="v0.2.0"`) {
		t.Error("version selector markup missing or wrong current version")
	}
	if !strings.Contains(page, `<option value="v0.2.0" selected>v0.2.0 (latest)</option>`) {
		t.Error("expected v0.2.0 option marked selected + latest")
	}
	if !strings.Contains(page, `<option value="v0.1.0">v0.1.0</option>`) {
		t.Error("expected v0.1.0 option present")
	}
	// The selector re-fetches versions.json at runtime.
	if !strings.Contains(page, "versions.json") {
		t.Error("expected page to reference versions.json for the selector")
	}

	// versions.json is valid JSON with the expected newest-first, latest shape.
	var m manifest
	if err := json.Unmarshal(readFileBytes(t, filepath.Join(out, "versions.json")), &m); err != nil {
		t.Fatalf("versions.json is not valid JSON: %v", err)
	}
	if len(m.Versions) != 2 {
		t.Fatalf("expected 2 versions, got %d", len(m.Versions))
	}
	if m.Versions[0].Version != "v0.2.0" || !m.Versions[0].Latest {
		t.Errorf("expected v0.2.0 newest+latest, got %+v", m.Versions[0])
	}
	if m.Versions[1].Version != "v0.1.0" || m.Versions[1].Latest {
		t.Errorf("expected v0.1.0 older, not latest, got %+v", m.Versions[1])
	}

	// latest/ mirrors the newest version's content.
	if latestPage := readFile(t, filepath.Join(out, "latest", "intro.html")); latestPage != page {
		t.Error("latest/intro.html should mirror v0.2.0/intro.html byte-for-byte")
	}
}

func TestGenerateIsDeterministic(t *testing.T) {
	docs := writeFixtureDocs(t)
	render := func() string {
		out := t.TempDir()
		if err := Generate(Options{DocsDir: docs, OutDir: out, Version: "v1.0.0"}); err != nil {
			t.Fatal(err)
		}
		return readFile(t, filepath.Join(out, "v1.0.0", "intro.html"))
	}
	if render() != render() {
		t.Error("generator output is not deterministic")
	}
}

func TestRewriteMdLink(t *testing.T) {
	cases := []struct {
		in       string
		want     string
		wantChng bool
	}{
		{"specs/cli.md", "specs/cli.html", true},
		{"../intro.md#top", "../intro.html#top", true},
		{"foo.md?x=1", "foo.html?x=1", true},
		{"https://example.com/a.md", "https://example.com/a.md", false},
		{"/abs/path.md", "/abs/path.md", false},
		{"#anchor", "#anchor", false},
		{"image.png", "image.png", false},
		{"mailto:x@y.md", "mailto:x@y.md", false},
	}
	for _, c := range cases {
		got, chng := rewriteMdLink(c.in)
		if got != c.want || chng != c.wantChng {
			t.Errorf("rewriteMdLink(%q) = (%q,%v), want (%q,%v)", c.in, got, chng, c.want, c.wantChng)
		}
	}
}

func TestCompareSemver(t *testing.T) {
	cases := []struct {
		a, b string
		want int // sign
	}{
		{"v0.2.0", "v0.1.0", 1},
		{"v0.1.0", "v0.2.0", -1},
		{"v1.0.0", "v1.0.0", 0},
		{"v1.2.0", "v1.10.0", -1},   // numeric, not lexical
		{"v1.0.0", "v1.0.0-rc1", 1}, // release beats prerelease
		{"v1.0.0-rc2", "v1.0.0-rc1", 1},
		{"v2.0.0", "v1.9.9", 1},
	}
	for _, c := range cases {
		got := compareSemver(c.a, c.b)
		if sign(got) != c.want {
			t.Errorf("compareSemver(%q,%q) sign = %d, want %d", c.a, c.b, sign(got), c.want)
		}
	}
}

func sign(n int) int {
	switch {
	case n > 0:
		return 1
	case n < 0:
		return -1
	default:
		return 0
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	return string(readFileBytes(t, path))
}

func readFileBytes(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
