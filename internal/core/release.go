// Release cutting (releases-and-versioning spec §3, DR-0005).

package core

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type CutResult struct {
	Version  string
	Previous string
	Added    int
	Removed  int
}

func remoteReleaseTags(root string) ([]string, error) {
	cmd := exec.Command("git", "ls-remote", "--tags", "--refs", "origin")
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		detail := err.Error()
		if ee, ok := err.(*exec.ExitError); ok {
			detail = strings.TrimSpace(string(ee.Stderr))
		}
		// Hard error: never compute a version from an unknown baseline.
		return nil, Errf("cannot list remote tags: %s", detail)
	}
	var tags []string
	for _, line := range strings.Split(string(out), "\n") {
		if idx := strings.LastIndex(line, "refs/tags/"); idx >= 0 {
			tags = append(tags, line[idx+len("refs/tags/"):])
		}
	}
	return tags, nil
}

func tagExists(root, tag string) (bool, error) {
	cmd := exec.Command("git", "rev-parse", "-q", "--verify", "refs/tags/"+tag)
	cmd.Dir = root
	if cmd.Run() == nil {
		return true, nil
	}
	remote, err := remoteReleaseTags(root)
	if err != nil {
		return false, err
	}
	return contains(remote, tag), nil
}

// surfaceDelta: (added paths, removed non-seed paths) vs the previous
// release's manifest. Seed-entry removals are excluded (DR-0013): retiring
// a template never demands a MAJOR or a migration payload.
func surfaceDelta(root string, decl *ExportDecl, previous string) ([]string, []string, error) {
	cmd := exec.Command("git", "show", previous+":"+decl.ManifestName)
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		return nil, nil, nil // previous release predates a manifest: no gate
	}
	var prev map[string]any
	if jsonErr := json.Unmarshal(out, &prev); jsonErr != nil {
		return nil, nil, Errf("previous manifest unreadable: %v", jsonErr)
	}
	prevPaths := map[string]bool{}
	prevNonseed := map[string]bool{}
	for _, e := range manifestEntries(prev) {
		p := getStr(e, "path")
		prevPaths[p] = true
		if !getBool(e, "seed") {
			prevNonseed[p] = true
		}
	}
	fresh, err := BuildPublisherManifest(decl, root)
	if err != nil {
		return nil, nil, err
	}
	currPaths := map[string]bool{}
	for _, e := range manifestEntries(fresh) {
		currPaths[getStr(e, "path")] = true
	}
	var added, removed []string
	for p := range currPaths {
		if !prevPaths[p] {
			added = append(added, p)
		}
	}
	for p := range prevNonseed {
		if !currPaths[p] {
			removed = append(removed, p)
		}
	}
	return added, removed, nil
}

// Cut cuts a release: freshness pre-gate, baseline, surface-delta bump
// enforcement, migration pre-gate, annotated tag + push.
func Cut(root string, decl *ExportDecl, bumpKind, explicitVersion, summary string,
	noMigrationsNeeded, dryRun bool) (*CutResult, error) {
	// 1. Freshness pre-gate: a release never ships a stale manifest.
	manifestPath := filepath.Join(root, decl.ManifestName)
	if _, err := os.Stat(manifestPath); err != nil {
		return nil, Usagef("missing manifest %s — run generate", decl.ManifestName)
	}
	committed, err := LoadManifest(manifestPath)
	if err != nil {
		return nil, err
	}
	fresh, err := BuildPublisherManifest(decl, root)
	if err != nil {
		return nil, err
	}
	if !ManifestsEqual(committed, fresh) {
		return nil, &Refusal{Reason: "stale-manifest",
			Msg: "committed manifest is stale — run generate and commit"}
	}

	// 2. Baseline and target.
	remote, err := remoteReleaseTags(root)
	if err != nil {
		return nil, err
	}
	latest := Latest(remote, "rc", nil)
	var target string
	if explicitVersion != "" {
		target = explicitVersion
		if _, err := RequireVersion(target); err != nil {
			return nil, err
		}
	} else {
		if bumpKind == "" {
			return nil, Usagef("either --bump or --version is required")
		}
		base := latest
		if base == "" {
			base = "v0.0.0"
		}
		target, err = BumpVersion(base, bumpKind)
		if err != nil {
			return nil, err
		}
	}
	if latest != "" {
		tk, _ := RequireVersion(target)
		lk, _ := RequireVersion(latest)
		if !lk.Less(tk) {
			return nil, &Refusal{Reason: "not-newer",
				Msg: fmt.Sprintf("%s is not newer than latest %s", target, latest)}
		}
	}
	exists, err := tagExists(root, target)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, &Refusal{Reason: "tag-exists",
			Msg: fmt.Sprintf("tag %s already exists (INV-5)", target)}
	}

	// 3. Surface-delta bump enforcement (releases spec §2).
	var added, removed []string
	if latest != "" {
		added, removed, err = surfaceDelta(root, decl, latest)
		if err != nil {
			return nil, err
		}
		implied := "patch"
		if len(removed) > 0 {
			implied = "major"
		} else if len(added) > 0 {
			implied = "minor"
		}
		order := map[string]int{"patch": 0, "minor": 1, "major": 2}
		actual, err := ClassifyBump(latest, target)
		if err != nil {
			return nil, err
		}
		if order[actual] < order[implied] {
			return nil, &Refusal{Reason: "bump-too-small",
				Msg: fmt.Sprintf(
					"surface delta (+%d/-%d) implies at least a %s bump; "+
						"%s -> %s is %s",
					len(added), len(removed), implied, latest, target, actual)}
		}
		// 4. Migration pre-gate: removals demand a payload or an override.
		if len(removed) > 0 && !noMigrationsNeeded {
			entries, err := LoadMigrationEntries(root)
			if err != nil {
				return nil, err
			}
			found := false
			for _, e := range entries {
				if getStr(e, "applies_from") == target {
					found = true
				}
			}
			if !found {
				return nil, &Refusal{Reason: "migration-missing",
					Msg: fmt.Sprintf(
						"release removes %d exported file(s) but ships no "+
							"migrations/ entry with applies_from: %s "+
							"(pass --no-migrations-needed to record an override)",
						len(removed), target)}
			}
		}
	}

	// 5. Annotated tag + push (remote ref rejection = serialisation point).
	if !dryRun {
		var parts []string
		parts = append(parts, fmt.Sprintf("%s %s", decl.SliceName, target))
		if summary != "" {
			parts = append(parts, summary)
		}
		parts = append(parts, fmt.Sprintf("surface-delta: +%d -%d",
			len(added), len(removed)))
		if noMigrationsNeeded {
			parts = append(parts, "no-migrations-needed")
		}
		message := strings.Join(parts, "\n")
		if _, err := RunGit([]string{"tag", "-a", target, "-m", message}, root); err != nil {
			return nil, err
		}
		if _, err := RunGit([]string{"push", "origin", "refs/tags/" + target}, root); err != nil {
			return nil, err
		}
	}
	return &CutResult{Version: target, Previous: latest,
		Added: len(added), Removed: len(removed)}, nil
}
