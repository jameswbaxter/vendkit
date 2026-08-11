// Manifest build/load/diff and the gate lane (manifest-and-gate spec).

package core

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
)

const (
	SchemaVersion = 1
	VendkitDir    = ".vendkit"
	// The two halves of .vendkit/ are separated so neither is reachable by the
	// other's discovery globs, and so a repository that both publishes and
	// consumes keeps the roles legible (DR-0019). Publisher-side: the export
	// declaration, the generated manifest, the conformance rule set, the
	// subscribers list. Consumer-side: slice configs and vendored manifests.
	PublisherSubdir = "publisher"
	ConsumerSubdir  = "consumer"
)

func isExec(path string) (bool, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return false, Errf("stat %s: %v", path, err)
	}
	return fi.Mode()&0o100 != 0, nil
}

// ExecBitDecidable: whether this platform's filesystem represents the POSIX
// executable bit. On Windows it does not — Stat never reports 0o100 — so the
// bit is not tree-decidable there (differences ledger #8): exec facts in
// manifests are recorded where POSIX generation runs, and comparisons
// against a Windows working tree skip the bit instead of false-alarming.
var ExecBitDecidable = runtime.GOOS != "windows"

// stripExec: a copy of the manifest with per-entry exec facts removed — the
// comparison shape for platforms where the bit is not decidable.
func stripExec(m map[string]any) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = v
	}
	entries := []any{}
	for _, e := range getList(m, "entries") {
		em, ok := e.(map[string]any)
		if !ok {
			entries = append(entries, e)
			continue
		}
		ne := make(map[string]any, len(em))
		for k, v := range em {
			if k != "exec" {
				ne[k] = v
			}
		}
		entries = append(entries, ne)
	}
	out["entries"] = entries
	return out
}

func manifestsEquivalentFor(a, b map[string]any, execDecidable bool) bool {
	if execDecidable {
		return ManifestsEqual(a, b)
	}
	return ManifestsEqual(stripExec(a), stripExec(b))
}

// ManifestsEquivalent is the staleness comparison for `generate --check` and
// the release freshness pre-gate: ManifestsEqual, except that where the
// filesystem cannot represent the exec bit (ledger #8) the per-entry exec
// facts are excluded — a Windows checkout must not read as stale purely
// because Stat cannot see a bit the tree does carry. A manifest WRITTEN on
// such a platform still records what Stat saw, so it turns loudly stale on
// POSIX CI rather than silently dropping the bit for everyone.
func ManifestsEquivalent(a, b map[string]any) bool {
	return manifestsEquivalentFor(a, b, ExecBitDecidable)
}

// BuildPublisherManifest: publisher-side manifest of the working tree.
// Hashes are of the file as vendored for an unbound consumer.
func BuildPublisherManifest(decl *ExportDecl, root string) (map[string]any, error) {
	vendored, err := decl.ExportedFiles(root)
	if err != nil {
		return nil, err
	}
	seeded, err := decl.SeededFiles(root)
	if err != nil {
		return nil, err
	}
	isSeed := map[string]bool{}
	for _, s := range seeded {
		isSeed[s] = true
	}
	for _, v := range vendored {
		delete(isSeed, v)
	}
	var entries []any
	seenConsumer := map[string]string{}
	all := append(append([]string{}, vendored...), seeded...)
	for _, rel := range all {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			return nil, Errf("read %s: %v", rel, err)
		}
		digest, raw := NormaliseHash(data)
		consumerPath, err := decl.ConsumerPath(rel)
		if err != nil {
			return nil, err
		}
		if prev, dup := seenConsumer[consumerPath]; dup {
			return nil, Usagef(
				"consumer_path collision inside slice: %s (%s vs %s)",
				consumerPath, prev, rel)
		}
		seenConsumer[consumerPath] = rel
		exec, err := isExec(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			return nil, err
		}
		entry := map[string]any{
			"path": rel, "consumer_path": consumerPath,
			"sha256": digest, // for seeds: the template's hash (DR-0013)
			"exec":   exec, "raw": raw,
		}
		if isSeed[rel] {
			entry["seed"] = true
		}
		entries = append(entries, entry)
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].(map[string]any)["path"].(string) <
			entries[j].(map[string]any)["path"].(string)
	})
	if entries == nil {
		entries = []any{}
	}
	return map[string]any{
		"schema_version": SchemaVersion,
		"slice":          decl.SliceName,
		"profile":        "*",
		"normalisation":  Recipe,
		"entries":        entries,
	}, nil
}

func LoadManifest(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, Usagef("manifest not found: %s", path)
		}
		return nil, Errf("manifest unreadable: %s: %v", path, err)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, Errf("manifest unreadable: %s: %v", path, err)
	}
	if !schemaVersionIs(m, SchemaVersion) {
		return nil, Errf("%s: unsupported schema_version %v (engine supports %d)",
			path, m["schema_version"], SchemaVersion)
	}
	return m, nil
}

func DumpManifest(m map[string]any, path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return Errf("mkdir for %s: %v", path, err)
	}
	if err := os.WriteFile(path, CanonicalJSON(m), 0o644); err != nil {
		return Errf("write %s: %v", path, err)
	}
	return nil
}

func ManifestsEqual(a, b map[string]any) bool {
	return string(CanonicalJSON(a)) == string(CanonicalJSON(b))
}

func manifestEntries(m map[string]any) []map[string]any {
	var out []map[string]any
	for _, e := range getList(m, "entries") {
		if em, ok := e.(map[string]any); ok {
			out = append(out, em)
		}
	}
	return out
}

// -- gate lane -------------------------------------------------------------------

type Finding struct {
	Manifest     string `json:"manifest"`
	SliceName    string `json:"slice_name"`
	ConsumerPath string `json:"consumer_path"`
	Kind         string `json:"kind"` // changed | removed | collision
	Detail       string `json:"detail"`
}

type GateReport struct {
	Findings []Finding
	Checked  int
}

// DiscoverManifests: the fixed discovery convention (DR-0012).
func DiscoverManifests(consumerRoot string) []string {
	pattern := filepath.Join(consumerRoot, VendkitDir, ConsumerSubdir, "*-manifest.json")
	hits, _ := filepath.Glob(pattern)
	sort.Strings(hits)
	return hits
}

// GateCheck verifies vendored files against their manifests; enforces INV-7.
func GateCheck(consumerRoot string, manifestPaths []string) (*GateReport, error) {
	report := &GateReport{}
	claimed := map[string]string{}
	for _, mpath := range manifestPaths {
		manifest, err := LoadManifest(mpath)
		if err != nil {
			return nil, err
		}
		sliceName := getStr(manifest, "slice")
		if sliceName == "" {
			sliceName = "?"
		}
		for _, entry := range manifestEntries(manifest) {
			cpath := getStr(entry, "consumer_path")
			report.Checked++
			if prev, dup := claimed[cpath]; dup {
				report.Findings = append(report.Findings, Finding{
					mpath, sliceName, cpath, "collision",
					fmt.Sprintf("also tracked by %s", prev)})
				continue
			}
			claimed[cpath] = mpath
			if getBool(entry, "seed") {
				// Seeded files are consumer-owned after materialisation
				// (DR-0013): free to diverge, free to delete. They still
				// claim their consumer_path above (INV-7).
				continue
			}
			fpath := filepath.Join(consumerRoot, filepath.FromSlash(cpath))
			data, err := os.ReadFile(fpath)
			if err != nil {
				if os.IsNotExist(err) {
					report.Findings = append(report.Findings,
						Finding{mpath, sliceName, cpath, "removed", ""})
					continue
				}
				return nil, Errf("read %s: %v", cpath, err)
			}
			digest := HashAsRecorded(data, getBool(entry, "raw"))
			if digest != getStr(entry, "sha256") {
				report.Findings = append(report.Findings,
					Finding{mpath, sliceName, cpath, "changed", "content differs"})
				continue
			}
			if !ExecBitDecidable {
				continue // ledger #8: the bit is invisible here, not drifted
			}
			exec, err := isExec(fpath)
			if err != nil {
				return nil, err
			}
			if exec != getBool(entry, "exec") {
				report.Findings = append(report.Findings,
					Finding{mpath, sliceName, cpath, "changed", "exec bit differs"})
			}
		}
	}
	return report, nil
}
