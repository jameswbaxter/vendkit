// Version grammar, ordering, channels, retraction awareness
// (releases-and-versioning spec §2).

package core

import (
	"fmt"
	"regexp"
	"strconv"
)

var (
	stableRx = regexp.MustCompile(`^v(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)$`)
	rcRx     = regexp.MustCompile(`^v(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)-rc\.([1-9]\d*)$`)
)

// VersionKey orders as (major, minor, patch, is_stable, rc_n): a stable
// release sorts above every rc of the same triple (SemVer §11).
type VersionKey struct{ Major, Minor, Patch, Stable, RC int }

func (k VersionKey) Less(o VersionKey) bool {
	a := [5]int{k.Major, k.Minor, k.Patch, k.Stable, k.RC}
	b := [5]int{o.Major, o.Minor, o.Patch, o.Stable, o.RC}
	for i := range a {
		if a[i] != b[i] {
			return a[i] < b[i]
		}
	}
	return false
}

func atoi(s string) int { n, _ := strconv.Atoi(s); return n }

// ParseVersion parses a tag name; ok=false if not release-shaped for the
// channel ("stable" or "rc").
func ParseVersion(version, channel string) (VersionKey, bool) {
	if m := stableRx.FindStringSubmatch(version); m != nil {
		return VersionKey{atoi(m[1]), atoi(m[2]), atoi(m[3]), 1, 0}, true
	}
	if channel == "rc" {
		if m := rcRx.FindStringSubmatch(version); m != nil {
			return VersionKey{atoi(m[1]), atoi(m[2]), atoi(m[3]), 0, atoi(m[4])}, true
		}
	}
	return VersionKey{}, false
}

// RequireVersion parses accepting the widest grammar; *UsageError if malformed.
func RequireVersion(version string) (VersionKey, error) {
	key, ok := ParseVersion(version, "rc")
	if !ok {
		return VersionKey{}, Usagef("not a release version: %q", version)
	}
	return key, nil
}

// IsNewer: true iff target > pinned. Refuses a retracted target (exit 3).
func IsNewer(pinned, target string, retracted []string) (bool, error) {
	for _, r := range retracted {
		if r == target {
			return false, &Refusal{Reason: "retracted",
				Msg: fmt.Sprintf("target %s is retracted by the publisher", target)}
		}
	}
	t, err := RequireVersion(target)
	if err != nil {
		return false, err
	}
	p, err := RequireVersion(pinned)
	if err != nil {
		return false, err
	}
	return p.Less(t), nil
}

// Latest: greatest qualifying version among tag names; "" when none qualify.
func Latest(tags []string, channel string, retracted []string) string {
	best := ""
	var bestKey VersionKey
	for _, name := range tags {
		skip := false
		for _, r := range retracted {
			if r == name {
				skip = true
				break
			}
		}
		if skip {
			continue
		}
		key, ok := ParseVersion(name, channel)
		if !ok {
			continue
		}
		if best == "" || bestKey.Less(key) {
			best, bestKey = name, key
		}
	}
	return best
}

// ClassifyBump: "patch" | "minor" | "major" for the pinned→target jump.
func ClassifyBump(pinned, target string) (string, error) {
	p, err := RequireVersion(pinned)
	if err != nil {
		return "", err
	}
	t, err := RequireVersion(target)
	if err != nil {
		return "", err
	}
	switch {
	case t.Major != p.Major:
		return "major", nil
	case t.Minor != p.Minor:
		return "minor", nil
	default:
		return "patch", nil
	}
}

func BumpVersion(version, kind string) (string, error) {
	k, err := RequireVersion(version)
	if err != nil {
		return "", err
	}
	if k.Stable != 1 {
		return "", Usagef("cannot bump from a pre-release: %s", version)
	}
	switch kind {
	case "major":
		return fmt.Sprintf("v%d.0.0", k.Major+1), nil
	case "minor":
		return fmt.Sprintf("v%d.%d.0", k.Major, k.Minor+1), nil
	case "patch":
		return fmt.Sprintf("v%d.%d.%d", k.Major, k.Minor, k.Patch+1), nil
	}
	return "", Usagef("unknown bump kind: %q", kind)
}

// InWindow: migration window arithmetic — pinned < applies_from <= target.
func InWindow(pinned, appliesFrom, target string) (bool, error) {
	p, err := RequireVersion(pinned)
	if err != nil {
		return false, err
	}
	a, err := RequireVersion(appliesFrom)
	if err != nil {
		return false, err
	}
	t, err := RequireVersion(target)
	if err != nil {
		return false, err
	}
	return p.Less(a) && (a.Less(t) || a == t), nil
}
