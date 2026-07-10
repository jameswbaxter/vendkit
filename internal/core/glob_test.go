// Ported from tests/test_units.py — the one glob matcher (util.path_match).

package core

import "testing"

func TestPathMatchCrossesSegments(t *testing.T) {
	if !PathMatch("docs/a/b/c.md", "docs/**") {
		t.Error(`PathMatch("docs/a/b/c.md", "docs/**") should match`)
	}
	if !PathMatch("docs/x.md", "docs/*.md") {
		t.Error(`PathMatch("docs/x.md", "docs/*.md") should match`)
	}
	if PathMatch("docs/x.md", "src/*") {
		t.Error(`PathMatch("docs/x.md", "src/*") should not match`)
	}
}
