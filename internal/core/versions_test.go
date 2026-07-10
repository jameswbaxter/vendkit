// Ported from tests/test_units.py — version grammar, ordering, channels,
// retraction, bump/window (testing.md §1).

package core

import (
	"errors"
	"testing"
)

func mustParse(t *testing.T, v, channel string) VersionKey {
	t.Helper()
	k, ok := ParseVersion(v, channel)
	if !ok {
		t.Fatalf("ParseVersion(%q, %q) not ok", v, channel)
	}
	return k
}

func TestGrammarStableAndRC(t *testing.T) {
	if got := mustParse(t, "v1.2.3", "stable"); got != (VersionKey{1, 2, 3, 1, 0}) {
		t.Errorf("parse v1.2.3 = %+v, want {1 2 3 1 0}", got)
	}
	if _, ok := ParseVersion("v1.2.3-rc.1", "stable"); ok {
		t.Error("rc must be invisible on the stable channel")
	}
	if got := mustParse(t, "v1.2.3-rc.1", "rc"); got != (VersionKey{1, 2, 3, 0, 1}) {
		t.Errorf("parse v1.2.3-rc.1 (rc) = %+v, want {1 2 3 0 1}", got)
	}
	for _, bad := range []string{"1.2.3", "v1.2", "v01.2.3", "v1.2.3-rc.0", "v1.2.3.4"} {
		if _, ok := ParseVersion(bad, "rc"); ok {
			t.Errorf("ParseVersion(%q, rc) accepted a malformed version", bad)
		}
	}
}

func TestOrderingRCBelowStable(t *testing.T) {
	stable := mustRequire(t, "v1.2.3")
	rc := mustRequire(t, "v1.2.3-rc.9")
	if !rc.Less(stable) {
		t.Error("v1.2.3 must sort above v1.2.3-rc.9")
	}
	nextRC := mustRequire(t, "v1.2.4-rc.1")
	if !stable.Less(nextRC) {
		t.Error("v1.2.4-rc.1 must sort above v1.2.3")
	}
}

func mustRequire(t *testing.T, v string) VersionKey {
	t.Helper()
	k, err := RequireVersion(v)
	if err != nil {
		t.Fatalf("RequireVersion(%q): %v", v, err)
	}
	return k
}

func TestIsNewerAndRetraction(t *testing.T) {
	if newer, err := IsNewer("v1.0.0", "v1.0.1", nil); err != nil || !newer {
		t.Errorf("IsNewer(v1.0.0, v1.0.1) = %v, %v; want true, nil", newer, err)
	}
	if newer, err := IsNewer("v1.0.1", "v1.0.1", nil); err != nil || newer {
		t.Errorf("IsNewer(v1.0.1, v1.0.1) = %v, %v; want false, nil", newer, err)
	}
	_, err := IsNewer("v1.0.0", "v1.0.1", []string{"v1.0.1"})
	var refusal *Refusal
	if !errors.As(err, &refusal) || refusal.Reason != "retracted" {
		t.Errorf("retracted target: got err %v, want Refusal{retracted}", err)
	}
}

func TestLatestChannelsAndRetraction(t *testing.T) {
	tags := []string{"v1.0.0", "v1.1.0-rc.1", "junk", "v0.9.0"}
	if got := Latest(tags, "stable", nil); got != "v1.0.0" {
		t.Errorf("Latest(stable) = %q, want v1.0.0", got)
	}
	if got := Latest(tags, "rc", nil); got != "v1.1.0-rc.1" {
		t.Errorf("Latest(rc) = %q, want v1.1.0-rc.1", got)
	}
	if got := Latest(tags, "stable", []string{"v1.0.0"}); got != "v0.9.0" {
		t.Errorf("Latest(stable, retracted v1.0.0) = %q, want v0.9.0", got)
	}
	if got := Latest([]string{"nope"}, "stable", nil); got != "" {
		t.Errorf("Latest(no valid tags) = %q, want empty", got)
	}
}

func TestBumpAndWindow(t *testing.T) {
	for _, c := range []struct{ kind, want string }{
		{"major", "v2.0.0"}, {"minor", "v1.3.0"}, {"patch", "v1.2.4"},
	} {
		got, err := BumpVersion("v1.2.3", c.kind)
		if err != nil || got != c.want {
			t.Errorf("BumpVersion(v1.2.3, %s) = %q, %v; want %q", c.kind, got, err, c.want)
		}
	}
	if got, _ := ClassifyBump("v1.2.3", "v1.3.0"); got != "minor" {
		t.Errorf("ClassifyBump(v1.2.3, v1.3.0) = %q, want minor", got)
	}
	for _, c := range []struct {
		pinned, from, target string
		want                 bool
	}{
		{"v1.0.0", "v1.5.0", "v2.0.0", true},
		{"v1.5.0", "v1.5.0", "v2.0.0", false},
		{"v1.0.0", "v2.1.0", "v2.0.0", false},
	} {
		got, err := InWindow(c.pinned, c.from, c.target)
		if err != nil || got != c.want {
			t.Errorf("InWindow(%s, %s, %s) = %v, %v; want %v",
				c.pinned, c.from, c.target, got, err, c.want)
		}
	}
}
