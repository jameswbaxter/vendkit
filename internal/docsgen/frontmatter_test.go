package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStripFrontMatter(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "governed block is removed",
			in:   "---\nid: DR-0001\ntitle: \"A decision\"\n---\n\n# A decision\n",
			want: "\n# A decision\n",
		},
		{
			name: "dot fence closes the block",
			in:   "---\nid: DR-0002\n...\n# Body\n",
			want: "# Body\n",
		},
		{
			name: "CRLF line endings",
			in:   "---\r\nid: DR-0003\r\n---\r\n# Body\r\n",
			want: "# Body\r\n",
		},
		{
			name: "no front matter is untouched",
			in:   "# Body\n\nText.\n",
			want: "# Body\n\nText.\n",
		},
		{
			name: "an unclosed fence is left alone",
			in:   "---\n\nA document opening with a thematic break.\n",
			want: "---\n\nA document opening with a thematic break.\n",
		},
		{
			name: "a later fence is not a block",
			in:   "# Body\n\n---\n\nMore.\n",
			want: "# Body\n\n---\n\nMore.\n",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := string(stripFrontMatter([]byte(c.in))); got != c.want {
				t.Errorf("stripFrontMatter(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

// A governed document's front matter must reach neither the rendered body nor
// the nav title: the title comes from the H1 below the block.
func TestGenerateHidesFrontMatter(t *testing.T) {
	docs := t.TempDir()
	out := t.TempDir()
	mustWrite(t, filepath.Join(docs, "design", "DR-0001-example.md"), `---
id: DR-0001
title: "The front-matter title"
status: current
summary: "A scent sentence that belongs to the index, not the page."
---

# DR-0001 — Example decision

## Context

Prose.
`)
	if err := Generate(Options{DocsDir: docs, OutDir: out, Version: "v0.1.0"}); err != nil {
		t.Fatalf("Generate: %v", err)
	}

	b, err := os.ReadFile(filepath.Join(out, "v0.1.0", "design", "DR-0001-example.html"))
	if err != nil {
		t.Fatal(err)
	}
	page := string(b)
	for _, leaked := range []string{"status_since", "A scent sentence", "id: DR-0001"} {
		if strings.Contains(page, leaked) {
			t.Errorf("front matter leaked into the page: %q", leaked)
		}
	}
	if !strings.Contains(page, "Example decision") {
		t.Error("expected the H1 title in the rendered page")
	}
}
