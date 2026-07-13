package main

import (
	"bytes"
	"encoding/json"
	"os"
	"sort"
	"strconv"
	"strings"
)

// manifestName is the site-root file the in-page version selector fetches.
const manifestName = "versions.json"

// versionEntry is one published version in versions.json.
type versionEntry struct {
	Version string `json:"version"`
	Path    string `json:"path"`
	Latest  bool   `json:"latest"`
}

// manifest is the shape of versions.json: versions newest-first, exactly one
// marked latest.
type manifest struct {
	Versions []versionEntry `json:"versions"`
}

// loadManifest reads an existing versions.json, returning an empty manifest if
// the file is absent (first run).
func loadManifest(path string) (manifest, error) {
	var m manifest
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return m, nil
		}
		return m, err
	}
	if len(bytes.TrimSpace(b)) == 0 {
		return m, nil
	}
	if err := json.Unmarshal(b, &m); err != nil {
		return m, err
	}
	return m, nil
}

// merge adds version (idempotently) and returns the manifest sorted newest-first
// with the single newest version marked latest. Re-running with an existing
// version is a no-op beyond re-sorting.
func (m manifest) merge(version string) manifest {
	seen := false
	out := make([]versionEntry, 0, len(m.Versions)+1)
	for _, v := range m.Versions {
		if v.Version == version {
			seen = true
		}
		out = append(out, versionEntry{Version: v.Version, Path: v.Version})
	}
	if !seen {
		out = append(out, versionEntry{Version: version, Path: version})
	}
	sort.SliceStable(out, func(i, j int) bool {
		return compareSemver(out[i].Version, out[j].Version) > 0 // newest first
	})
	for i := range out {
		out[i].Latest = i == 0
	}
	return manifest{Versions: out}
}

// latest returns the version label currently marked latest, or "".
func (m manifest) latest() string {
	for _, v := range m.Versions {
		if v.Latest {
			return v.Version
		}
	}
	return ""
}

// marshal renders versions.json deterministically (indented, trailing newline).
func (m manifest) marshal() ([]byte, error) {
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(b, '\n'), nil
}

// compareSemver orders two labels of the form [v]MAJOR.MINOR.PATCH[-prerelease].
// Returns >0 if a is newer, <0 if older, 0 if equal. A release outranks any
// prerelease of the same MAJOR.MINOR.PATCH. Non-numeric or malformed labels
// fall back to lexical comparison so ordering is always total and stable.
func compareSemver(a, b string) int {
	am, ap, aok := parseSemver(a)
	bm, bp, bok := parseSemver(b)
	if !aok || !bok {
		return strings.Compare(a, b)
	}
	for i := 0; i < 3; i++ {
		if am[i] != bm[i] {
			if am[i] > bm[i] {
				return 1
			}
			return -1
		}
	}
	// Equal core: absence of prerelease is newer than any prerelease.
	if ap == "" && bp == "" {
		return 0
	}
	if ap == "" {
		return 1
	}
	if bp == "" {
		return -1
	}
	return comparePrerelease(ap, bp)
}

// parseSemver extracts the numeric [MAJOR,MINOR,PATCH] and prerelease string.
func parseSemver(s string) (core [3]int, prerelease string, ok bool) {
	s = strings.TrimPrefix(s, "v")
	if i := strings.IndexByte(s, '+'); i >= 0 { // drop build metadata
		s = s[:i]
	}
	if i := strings.IndexByte(s, '-'); i >= 0 {
		prerelease = s[i+1:]
		s = s[:i]
	}
	parts := strings.Split(s, ".")
	if len(parts) != 3 {
		return core, "", false
	}
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return core, "", false
		}
		core[i] = n
	}
	return core, prerelease, true
}

// comparePrerelease implements SemVer §11 dot-separated identifier precedence.
func comparePrerelease(a, b string) int {
	ai := strings.Split(a, ".")
	bi := strings.Split(b, ".")
	for i := 0; i < len(ai) && i < len(bi); i++ {
		an, aErr := strconv.Atoi(ai[i])
		bn, bErr := strconv.Atoi(bi[i])
		aNum, bNum := aErr == nil, bErr == nil
		switch {
		case aNum && bNum: // both numeric: compare as integers
			if an != bn {
				if an > bn {
					return 1
				}
				return -1
			}
		case aNum: // numeric identifiers have lower precedence than alphanumeric
			return -1
		case bNum:
			return 1
		default: // both alphanumeric: ASCII order
			if c := strings.Compare(ai[i], bi[i]); c != 0 {
				return c
			}
		}
	}
	// Longer prerelease set wins when all preceding identifiers are equal.
	switch {
	case len(ai) > len(bi):
		return 1
	case len(ai) < len(bi):
		return -1
	}
	return 0
}
