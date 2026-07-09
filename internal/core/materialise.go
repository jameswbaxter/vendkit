// Materialisation — the sync lane's core (sync spec §2). A pure function of
// (publisher tree, declaration, consumer profile, current consumer manifest)
// — INV-2. `check` predicts `apply` exactly — INV-3. Never deletes consumer
// files — INV-4/DR-0010.

package core

import (
	"bytes"
	"os"
	"path/filepath"
	"sort"
)

type SyncReport struct {
	Updated         []string
	RemovedUpstream []string
	Added           []string
	Seeded          []string // DR-0013
	SeedRetired     []string // template gone upstream
	TemplateUpdated []string // informational only
}

// Changed: template_updated is deliberately excluded — an upstream template
// change never forces a PR for a diverged, consumer-owned copy.
func (r *SyncReport) Changed() bool {
	return len(r.Updated)+len(r.RemovedUpstream)+len(r.Added)+
		len(r.Seeded)+len(r.SeedRetired) > 0
}

// render: (post-adapter bytes, consumer_path, exec) for one exported file.
func render(decl *ExportDecl, publisherRoot, rel, profile string) ([]byte, string, bool, error) {
	src := filepath.Join(publisherRoot, filepath.FromSlash(rel))
	raw, err := os.ReadFile(src)
	if err != nil {
		return nil, "", false, Errf("read %s: %v", rel, err)
	}
	data, err := ApplyAdapters(decl, rel, raw, profile)
	if err != nil {
		return nil, "", false, err
	}
	cpath, err := decl.ConsumerPath(rel)
	if err != nil {
		return nil, "", false, err
	}
	exec, err := isExec(src)
	if err != nil {
		return nil, "", false, err
	}
	return data, cpath, exec, nil
}

func treeMatches(consumerRoot, cpath string, data []byte, execBit bool) bool {
	f := filepath.Join(consumerRoot, filepath.FromSlash(cpath))
	existing, err := os.ReadFile(f)
	if err != nil || !bytes.Equal(existing, data) {
		return false
	}
	exec, err := isExec(f)
	return err == nil && exec == execBit
}

func writeVendored(consumerRoot, cpath string, data []byte, execBit bool) error {
	f := filepath.Join(consumerRoot, filepath.FromSlash(cpath))
	if err := os.MkdirAll(filepath.Dir(f), 0o755); err != nil {
		return Errf("mkdir for %s: %v", cpath, err)
	}
	if err := os.WriteFile(f, data, 0o644); err != nil {
		return Errf("write %s: %v", cpath, err)
	}
	fi, err := os.Stat(f)
	if err != nil {
		return Errf("stat %s: %v", cpath, err)
	}
	mode := fi.Mode()
	if execBit {
		mode |= 0o111
	} else {
		mode &^= 0o111
	}
	if err := os.Chmod(f, mode); err != nil {
		return Errf("chmod %s: %v", cpath, err)
	}
	return nil
}

// Materialise refreshes the tracked slice from the publisher tree (the
// engine's checkout at the target release). In check mode nothing is
// written; classification is identical (INV-3).
func Materialise(publisherRoot, consumerRoot string, decl *ExportDecl,
	target string, apply, reconcileScope bool) (*SyncReport, error) {
	report := &SyncReport{}
	manifestPath := filepath.Join(consumerRoot, VendkitDir, decl.ManifestName)
	current, err := LoadManifest(manifestPath)
	if err != nil {
		return nil, err
	}
	cfg, err := FindSliceConfig(consumerRoot, decl.SliceName)
	if err != nil {
		return nil, err
	}
	profile := ""
	if cfg != nil {
		profile = cfg.Profile
	}
	if profile != "" && len(decl.Profiles) > 0 {
		if _, ok := decl.Profiles[profile]; !ok {
			return nil, Usagef("profile %q not declared by slice %q",
				profile, decl.SliceName)
		}
	}

	exportedList, err := decl.ExportedFiles(publisherRoot)
	if err != nil {
		return nil, err
	}
	seedList, err := decl.SeededFiles(publisherRoot)
	if err != nil {
		return nil, err
	}
	exported := map[string]bool{}
	for _, e := range exportedList {
		exported[e] = true
	}
	seeds := map[string]bool{}
	for _, s := range seedList {
		seeds[s] = true
	}
	trackedEntries := map[string]map[string]any{}
	var tracked []string
	for _, e := range manifestEntries(current) {
		rel := getStr(e, "path")
		trackedEntries[rel] = e
		tracked = append(tracked, rel)
	}
	var newEntries []any

	emitEntry := func(rel string, bucket *[]string) error {
		data, cpath, execBit, err := render(decl, publisherRoot, rel, profile)
		if err != nil {
			return err
		}
		digest, raw := NormaliseHash(data)
		newEntries = append(newEntries, map[string]any{
			"path": rel, "consumer_path": cpath,
			"sha256": digest, "exec": execBit, "raw": raw,
		})
		if !treeMatches(consumerRoot, cpath, data, execBit) {
			*bucket = append(*bucket, cpath)
			if apply {
				return writeVendored(consumerRoot, cpath, data, execBit)
			}
		}
		return nil
	}

	// Scaffold-once (DR-0013): a tracked seed is NEVER written again — the
	// entry is the 'seeding happened' record, so deletion is respected.
	emitSeed := func(rel string) error {
		data, cpath, execBit, err := render(decl, publisherRoot, rel, profile)
		if err != nil {
			return err
		}
		digest, raw := NormaliseHash(data)
		newEntries = append(newEntries, map[string]any{
			"path": rel, "consumer_path": cpath, "sha256": digest,
			"exec": execBit, "raw": raw, "seed": true,
		})
		prior, isTracked := trackedEntries[rel]
		target := filepath.Join(consumerRoot, filepath.FromSlash(cpath))
		_, statErr := os.Stat(target)
		fileExists := statErr == nil
		if !isTracked {
			// Untracked: seed if absent; adopt (entry only, never clobber)
			// if the consumer already has a file at the target path.
			report.Seeded = append(report.Seeded, cpath)
			if apply && !fileExists {
				return writeVendored(consumerRoot, cpath, data, execBit)
			}
		} else if getStr(prior, "sha256") != digest && fileExists {
			report.TemplateUpdated = append(report.TemplateUpdated, cpath)
		}
		return nil
	}

	// 1. Tracked refresh + 2. removals (report-only; files stay on disk).
	for _, rel := range tracked {
		switch {
		case seeds[rel]:
			if err := emitSeed(rel); err != nil {
				return nil, err
			}
		case exported[rel]:
			// Includes a publisher reclassifying a seed as vendored: the
			// refresh overwrites — a deliberate, PR-visible class change.
			if err := emitEntry(rel, &report.Updated); err != nil {
				return nil, err
			}
		case getBool(trackedEntries[rel], "seed"):
			// Template retired upstream: the consumer's copy is theirs now.
			report.SeedRetired = append(report.SeedRetired, rel)
		default:
			report.RemovedUpstream = append(report.RemovedUpstream, rel)
		}
	}

	// 3. Additions — opt-in, bounded by the profile's export slice (DR-0010).
	if reconcileScope {
		var candidates []string
		for rel := range exported {
			if _, t := trackedEntries[rel]; !t {
				candidates = append(candidates, rel)
			}
		}
		for rel := range seeds {
			if _, t := trackedEntries[rel]; !t {
				candidates = append(candidates, rel)
			}
		}
		sort.Strings(candidates)
		for _, rel := range candidates {
			if !decl.ProfileInScope(profile, rel) {
				continue
			}
			if seeds[rel] {
				if err := emitSeed(rel); err != nil {
					return nil, err
				}
			} else {
				if err := emitEntry(rel, &report.Added); err != nil {
					return nil, err
				}
			}
		}
	}

	// 4. Manifest rewrite with provenance (manifest spec §1).
	if apply {
		sort.Slice(newEntries, func(i, j int) bool {
			return newEntries[i].(map[string]any)["path"].(string) <
				newEntries[j].(map[string]any)["path"].(string)
		})
		commit, err := RunGit([]string{"rev-parse", "HEAD"}, publisherRoot)
		if err != nil {
			return nil, err
		}
		prof := profile
		if prof == "" {
			prof = "*"
		}
		if newEntries == nil {
			newEntries = []any{}
		}
		newManifest := map[string]any{
			"schema_version": SchemaVersion,
			"slice":          decl.SliceName,
			"profile":        prof,
			"normalisation":  Recipe,
			"source": map[string]any{
				"scm": decl.PublisherSCM, "repo": decl.PublisherRepo,
				"release": target, "commit": commit,
			},
			"entries": newEntries,
		}
		if err := DumpManifest(newManifest, manifestPath); err != nil {
			return nil, err
		}
	}
	return report, nil
}

// PreviewChange is one file the next apply would write (human diff).
type PreviewChange struct {
	ConsumerPath string
	Old          []byte // nil = new file
	HasOld       bool
	New          []byte
}

// Preview mirrors Materialise's classification exactly: tracked seeds are
// never rewritten; unwritten upstream removals do not appear.
func Preview(publisherRoot, consumerRoot string, decl *ExportDecl,
	reconcileScope bool) ([]PreviewChange, error) {
	manifestPath := filepath.Join(consumerRoot, VendkitDir, decl.ManifestName)
	current, err := LoadManifest(manifestPath)
	if err != nil {
		return nil, err
	}
	cfg, err := FindSliceConfig(consumerRoot, decl.SliceName)
	if err != nil {
		return nil, err
	}
	profile := ""
	if cfg != nil {
		profile = cfg.Profile
	}
	exportedList, err := decl.ExportedFiles(publisherRoot)
	if err != nil {
		return nil, err
	}
	seedList, err := decl.SeededFiles(publisherRoot)
	if err != nil {
		return nil, err
	}
	exported := map[string]bool{}
	for _, e := range exportedList {
		exported[e] = true
	}
	seeds := map[string]bool{}
	for _, s := range seedList {
		seeds[s] = true
	}
	tracked := map[string]bool{}
	var trackedOrder []string
	for _, e := range manifestEntries(current) {
		tracked[getStr(e, "path")] = true
		trackedOrder = append(trackedOrder, getStr(e, "path"))
	}
	var out []PreviewChange
	emit := func(rel string, seedOnlyIfAbsent bool) error {
		data, cpath, _, err := render(decl, publisherRoot, rel, profile)
		if err != nil {
			return err
		}
		f := filepath.Join(consumerRoot, filepath.FromSlash(cpath))
		old, readErr := os.ReadFile(f)
		hasOld := readErr == nil
		if seedOnlyIfAbsent && hasOld {
			return nil // adopt, never clobber (DR-0013)
		}
		if !hasOld || !bytes.Equal(old, data) {
			out = append(out, PreviewChange{cpath, old, hasOld, data})
		}
		return nil
	}
	for _, rel := range trackedOrder {
		if exported[rel] && !seeds[rel] {
			if err := emit(rel, false); err != nil {
				return nil, err
			}
		}
	}
	if reconcileScope {
		var candidates []string
		for rel := range exported {
			if !tracked[rel] {
				candidates = append(candidates, rel)
			}
		}
		for rel := range seeds {
			if !tracked[rel] {
				candidates = append(candidates, rel)
			}
		}
		sort.Strings(candidates)
		for _, rel := range candidates {
			if !decl.ProfileInScope(profile, rel) {
				continue
			}
			if err := emit(rel, seeds[rel]); err != nil {
				return nil, err
			}
		}
	}
	return out, nil
}

// SeedEmptyManifest: onboarding seed — an empty tracked slice for reconcile
// to expand.
func SeedEmptyManifest(consumerRoot string, decl *ExportDecl) error {
	path := filepath.Join(consumerRoot, VendkitDir, decl.ManifestName)
	if _, err := os.Stat(path); err == nil {
		return Usagef("manifest already exists: %s", path)
	}
	return DumpManifest(map[string]any{
		"schema_version": SchemaVersion,
		"slice":          decl.SliceName,
		"profile":        "*",
		"normalisation":  Recipe,
		"entries":        []any{},
	}, path)
}
