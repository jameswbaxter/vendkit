// Exec-bit decidability (differences ledger #8): on a filesystem that
// cannot represent the POSIX executable bit, freshness comparisons exclude
// the per-entry exec facts — and nothing else.

package core

import "testing"

func execManifest(exec bool, sha string) map[string]any {
	return map[string]any{
		"schema_version": 1, "slice": "docs", "profile": "*",
		"entries": []any{map[string]any{
			"path": "tools/w", "consumer_path": "tools/w",
			"sha256": sha, "exec": exec, "raw": false,
		}},
	}
}

func TestManifestsEquivalentSkipsExecOnlyWhereUndecidable(t *testing.T) {
	posix := execManifest(true, "h1")    // generated where the bit exists
	windows := execManifest(false, "h1") // rebuilt where Stat cannot see it

	if manifestsEquivalentFor(posix, windows, true) {
		t.Error("with a decidable exec bit, an exec difference IS staleness")
	}
	if !manifestsEquivalentFor(posix, windows, false) {
		t.Error("with an undecidable exec bit, an exec difference must not read as stale")
	}
	// The relaxation is exec-only: any content difference stays loud.
	tampered := execManifest(false, "h2")
	if manifestsEquivalentFor(posix, tampered, false) {
		t.Error("a hash difference must stay stale even where exec is undecidable")
	}
}
