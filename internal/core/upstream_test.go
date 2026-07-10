// Ported from tests/test_units.py — upstream coordinates (DR-0015).

package core

import (
	"errors"
	"testing"
)

func TestCloneURLShorthandAndVerbatim(t *testing.T) {
	cases := []struct {
		scm, repo, want string
	}{
		{"github", "o/r", "https://github.com/o/r.git"},
		{"azure-repos", "org/proj/repo", "https://dev.azure.com/org/proj/_git/repo"},
		{"github", "/local/path", "/local/path"},
		{"github", "https://example.com/r.git", "https://example.com/r.git"},
	}
	for _, c := range cases {
		got, err := CloneURL(c.scm, c.repo)
		if err != nil || got != c.want {
			t.Errorf("CloneURL(%q, %q) = %q, %v; want %q", c.scm, c.repo, got, err, c.want)
		}
	}
	_, err := CloneURL("azure-repos", "o/r") // needs org/project/repo
	var usage *UsageError
	if !errors.As(err, &usage) {
		t.Errorf("CloneURL(azure-repos, o/r) = %v, want UsageError", err)
	}
}
