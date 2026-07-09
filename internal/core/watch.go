// Release watch (release-watch spec). Pure detection — no delivery
// (DR-0014); the CLI hands findings to the handoff handler.

package core

import (
	"fmt"
	"path/filepath"
	"strings"
)

type WatchFinding struct {
	SliceName string `json:"slice_name"`
	Kind      string `json:"kind"` // update-available | tag-moved | pin-unreadable | no-releases
	Pinned    string `json:"pinned"`
	Latest    string `json:"latest"`
	Bump      string `json:"bump"`
	Detail    string `json:"detail"`
}

type WatchReport struct {
	Findings []WatchFinding
	Slices   int
}

func (r *WatchReport) Actionable() []WatchFinding {
	var out []WatchFinding
	for _, f := range r.Findings {
		if f.Kind != "no-releases" {
			out = append(out, f)
		}
	}
	return out
}

// RetractedAtNewest reads the retraction list from the NEWEST release's
// declaration (releases spec §4 bootstrapping quirk). Best-effort: an
// unreadable declaration means no retractions, not a crash.
func RetractedAtNewest(url, newestTag string) []string {
	raw, err := ReadFileAt(url, newestTag, DefaultDecl)
	if err != nil {
		return nil
	}
	data, err := ParseYAMLMap(raw, DefaultDecl)
	if err != nil {
		return nil
	}
	retracted, _ := strList(data["retracted"])
	return retracted
}

func Watch(consumerRoot, sliceName string, dryRun bool) (*WatchReport, error) {
	report := &WatchReport{}
	configs, err := DiscoverSliceConfigs(consumerRoot)
	if err != nil {
		return nil, err
	}
	if sliceName != "" {
		var filtered []*SliceConfig
		for _, c := range configs {
			if c.SliceName == sliceName {
				filtered = append(filtered, c)
			}
		}
		if len(filtered) == 0 {
			return nil, Usagef("no slice config for %q", sliceName)
		}
		configs = filtered
	}
	for _, cfg := range configs {
		report.Slices++
		if dryRun {
			continue // PR self-test: no network, empty successful report
		}
		findings, err := watchOne(consumerRoot, cfg)
		if err != nil {
			return nil, err
		}
		report.Findings = append(report.Findings, findings...)
	}
	return report, nil
}

func watchOne(consumerRoot string, cfg *SliceConfig) ([]WatchFinding, error) {
	var findings []WatchFinding
	// 1. PINNED — config rot must be loud (spec §2).
	pinned, err := ReadPin(consumerRoot, cfg)
	if err != nil {
		if _, isUsage := err.(*UsageError); isUsage {
			return []WatchFinding{{SliceName: cfg.SliceName,
				Kind: "pin-unreadable", Detail: err.Error()}}, nil
		}
		return nil, err
	}

	// 2. LATEST over the git protocol (vendor-free — DR-0015).
	url, err := CloneURL(cfg.PublisherSCM, cfg.PublisherRepo)
	if err != nil {
		return nil, err
	}
	tags, err := ListReleaseTags(url)
	if err != nil {
		return nil, err // infrastructure failure: propagate
	}
	names := make([]string, len(tags))
	for i, t := range tags {
		names[i] = t.Name
	}
	newest := Latest(names, "rc", nil)
	var retracted []string
	if newest != "" {
		retracted = RetractedAtNewest(url, newest)
	}
	latest := Latest(names, cfg.Channel, retracted)
	if latest == "" {
		return []WatchFinding{{SliceName: cfg.SliceName, Kind: "no-releases"}}, nil
	}

	// 3. Provenance: pinned tag must still resolve to the recorded commit
	// (security model §3).
	manifestPath := filepath.Join(consumerRoot, VendkitDir, cfg.SliceName+"-manifest.json")
	if manifest, err := LoadManifest(manifestPath); err == nil {
		source := getMap(manifest, "source")
		recorded := getStr(source, "commit")
		if recorded != "" && getStr(source, "release") == pinned {
			now := ""
			for _, t := range tags {
				if t.Name == pinned {
					now = t.Commit
				}
			}
			if now != "" && now != recorded {
				findings = append(findings, WatchFinding{
					SliceName: cfg.SliceName, Kind: "tag-moved", Pinned: pinned,
					Detail: fmt.Sprintf("tag %s resolved to %s, manifest recorded %s",
						pinned, now[:12], recorded[:12]),
				})
			}
		}
	}

	// 4. Compare.
	lk, err := RequireVersion(latest)
	if err != nil {
		return nil, err
	}
	pk, err := RequireVersion(pinned)
	if err != nil {
		return nil, err
	}
	if pk.Less(lk) {
		bump, err := ClassifyBump(pinned, latest)
		if err != nil {
			return nil, err
		}
		findings = append(findings, WatchFinding{
			SliceName: cfg.SliceName, Kind: "update-available",
			Pinned: pinned, Latest: latest, Bump: bump,
		})
	}
	return findings, nil
}

func RenderWatchReport(report *WatchReport) string {
	lines := []string{fmt.Sprintf("# vendkit watch — %d slice(s)", report.Slices), ""}
	if len(report.Findings) == 0 {
		lines = append(lines, "No findings.")
	}
	for _, f := range report.Findings {
		head := fmt.Sprintf("- **%s**: %s", f.SliceName, f.Kind)
		if f.Kind == "update-available" {
			head += fmt.Sprintf(" %s → %s (%s)", f.Pinned, f.Latest, f.Bump)
		}
		if f.Detail != "" {
			head += " — " + f.Detail
		}
		lines = append(lines, head)
	}
	return strings.Join(lines, "\n") + "\n"
}
