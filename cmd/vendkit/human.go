// Human tier — compositions over the machine tier, never a parallel code
// path (cli spec). Output formatting here is exempt from the key=value
// stability promise. Plus init, whose prompts are the human path.
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"unicode/utf8"

	vendkitassets "example.org/vendkit"
	"example.org/vendkit/internal/ci"
	"example.org/vendkit/internal/core"
)

// -- init ------------------------------------------------------------------------

// inferSCM: the origin remote discriminates the SCM host (DR-0015).
func inferSCM(consumerRoot string) string {
	cmd := exec.Command("git", "-C", consumerRoot, "remote", "get-url", "origin")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	url := strings.TrimSpace(string(out))
	if strings.Contains(url, "github.com") {
		return "github"
	}
	if strings.Contains(url, "dev.azure.com") || strings.Contains(url, "visualstudio.com") {
		return "azure-repos"
	}
	return ""
}

func isInteractive() bool {
	for _, f := range []*os.File{os.Stdin, os.Stdout} {
		fi, err := f.Stat()
		if err != nil || fi.Mode()&os.ModeCharDevice == 0 {
			return false
		}
	}
	return true
}

// prompt: TTY-only prompt for init's human path.
func prompt(question string, choices []string, allowEmpty bool) string {
	reader := bufio.NewReader(os.Stdin)
	hint := ""
	if len(choices) > 0 {
		hint = " [" + strings.Join(choices, "/") + "]"
	}
	for {
		fmt.Printf("%s%s: ", question, hint)
		answer, _ := reader.ReadString('\n')
		answer = strings.TrimSpace(answer)
		if answer == "" && allowEmpty {
			return ""
		}
		if answer != "" && (len(choices) == 0 || containsStr(choices, answer)) {
			return answer
		}
	}
}

func containsStr(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

func cmdInit(args []string, surface ci.Surface) (int, error) {
	fs := newFlagSet("init")
	var c commonFlags
	ciHost := fs.String("ci", "", "")
	scm := fs.String("scm", "", "")
	version := fs.String("version", "", "")
	profile := fs.String("profile", "", "")
	mode := fs.String("mode", "primary", "")
	baseBranch := fs.String("base-branch", "main", "")
	prTokenSecret := fs.String("pr-token-secret", "VENDKIT_PR_TOKEN", "")
	codeowners := fs.String("codeowners", "", "")
	addCommon(fs, &c, true, true, true)
	if err := parseFlags(fs, args); err != nil {
		return 0, err
	}
	decl, err := loadDeclFrom(c.PublisherRoot, c.ExportDecl)
	if err != nil {
		return 0, err
	}
	interactive := isInteractive()
	if *ciHost == "" {
		if !interactive {
			return 0, core.Usagef("--ci is required (github-actions|azure-pipelines|none)")
		}
		*ciHost = prompt("CI host ('none' = fully manual orchestration)",
			[]string{"github-actions", "azure-pipelines", "none"}, false)
	}
	if *version == "" {
		if !interactive {
			return 0, core.Usagef("--version is required (the release to pin)")
		}
		*version = prompt("Publisher release to pin (e.g. v1.2.3)", nil, false)
	}
	if *profile == "" && len(decl.Profiles) > 0 && interactive {
		*profile = prompt("Profile", sortedKeys(decl.Profiles), true)
	}
	resolvedSCM := *scm
	if resolvedSCM == "" {
		resolvedSCM = inferSCM(c.ConsumerRoot)
	}
	if resolvedSCM == "" {
		if interactive {
			resolvedSCM = prompt("SCM host (no origin remote to infer from)",
				[]string{"github", "azure-repos"}, false)
		} else {
			return 0, core.Usagef("cannot infer the SCM host from the " +
				"consumer's origin remote — pass --scm github|azure-repos")
		}
	}
	result, err := core.Onboard(c.PublisherRoot, c.ConsumerRoot, decl,
		core.OnboardParams{
			CI: *ciHost, SCM: resolvedSCM, Version: *version,
			Profile: *profile, Mode: *mode, BaseBranch: *baseBranch,
			PRTokenSecret: *prTokenSecret, Codeowners: *codeowners,
		}, vendkitassets.FS)
	if err != nil {
		return 0, err
	}
	for _, path := range result.Written {
		fmt.Printf("wrote: %s\n", path)
	}
	fmt.Printf("vendored: %d file(s)\n", result.Vendored)
	fmt.Println()
	fmt.Println(result.ManualSteps)
	return 0, nil
}

// -- shared human-tier helpers ------------------------------------------------------

// sliceOrOnly: --slice, or the sole configured slice; ambiguity is a usage
// error.
func sliceOrOnly(consumerRoot, slice string) (*core.SliceConfig, error) {
	configs, err := core.DiscoverSliceConfigs(consumerRoot)
	if err != nil {
		return nil, err
	}
	if len(configs) == 0 {
		return nil, core.Usagef("no slice configs under .vendkit/ — run `vendkit init`")
	}
	if slice != "" {
		for _, cfg := range configs {
			if cfg.SliceName == slice {
				return cfg, nil
			}
		}
		return nil, core.Usagef("no slice config for %q", slice)
	}
	if len(configs) == 1 {
		return configs[0], nil
	}
	var names []string
	for _, cfg := range configs {
		names = append(names, cfg.SliceName)
	}
	return nil, core.Usagef("%d slices configured — pass --slice (%s)",
		len(configs), strings.Join(names, ", "))
}

func pinnedRelease(consumerRoot string, cfg *core.SliceConfig) (string, error) {
	mpath := filepath.Join(consumerRoot, core.VendkitDir, cfg.SliceName+"-manifest.json")
	if manifest, err := core.LoadManifest(mpath); err == nil {
		if release := stringAt(manifest, "source", "release"); release != "" {
			return release, nil
		}
	}
	return core.ReadPin(consumerRoot, cfg)
}

func stringAt(m map[string]any, keys ...string) string {
	cur := any(m)
	for _, k := range keys {
		cm, ok := cur.(map[string]any)
		if !ok {
			return ""
		}
		cur = cm[k]
	}
	s, _ := cur.(string)
	return s
}

func latestRelease(cfg *core.SliceConfig) (string, error) {
	url, err := core.CloneURL(cfg.PublisherSCM, cfg.PublisherRepo)
	if err != nil {
		return "", err
	}
	tags, err := core.ListReleaseTags(url)
	if err != nil {
		return "", err
	}
	names := make([]string, len(tags))
	for i, t := range tags {
		names[i] = t.Name
	}
	newest := core.Latest(names, "rc", nil)
	var retracted []string
	if newest != "" {
		retracted = core.RetractedAtNewest(url, newest)
	}
	return core.Latest(names, cfg.Channel, retracted), nil
}

// fetchedPublisher clones the publisher at target into a temp dir; the
// caller must os.RemoveAll it.
func fetchedPublisher(cfg *core.SliceConfig, target string) (string, error) {
	url, err := core.CloneURL(cfg.PublisherSCM, cfg.PublisherRepo)
	if err != nil {
		return "", err
	}
	tmp, err := os.MkdirTemp("", "vendkit-publisher-")
	if err != nil {
		return "", core.Errf("mkdtemp: %v", err)
	}
	if err := core.FetchPublisher(url, target, tmp); err != nil {
		os.RemoveAll(tmp)
		return "", err
	}
	return tmp, nil
}

// -- status --------------------------------------------------------------------------

// cmdStatus: the human entry point — where is every slice, and does
// anything need attention?
func cmdStatus(args []string, surface ci.Surface) (int, error) {
	fs := newFlagSet("status")
	var c commonFlags
	slice := fs.String("slice", "", "")
	addCommon(fs, &c, false, true, false)
	if err := parseFlags(fs, args); err != nil {
		return 0, err
	}
	configs, err := core.DiscoverSliceConfigs(c.ConsumerRoot)
	if err != nil {
		return 0, err
	}
	if *slice != "" {
		var filtered []*core.SliceConfig
		for _, cfg := range configs {
			if cfg.SliceName == *slice {
				filtered = append(filtered, cfg)
			}
		}
		if len(filtered) == 0 {
			return 0, core.Usagef("no slice config for %q", *slice)
		}
		configs = filtered
	}
	if len(configs) == 0 {
		return 0, core.Usagef("no slice configs under .vendkit/ — run `vendkit init`")
	}
	type row struct {
		Slice       string `json:"slice"`
		CI          string `json:"ci"`
		SCM         string `json:"scm"`
		Profile     string `json:"profile"`
		Pinned      string `json:"pinned"`
		PinError    string `json:"pin_error,omitempty"`
		Latest      string `json:"latest"`
		LatestError string `json:"latest_error,omitempty"`
		Update      bool   `json:"update"`
		Bump        string `json:"bump"`
		Drift       int    `json:"drift"`
		HasDrift    bool   `json:"-"`
	}
	var rows []row
	for _, cfg := range configs {
		r := row{Slice: cfg.SliceName, CI: cfg.CI, SCM: cfg.SCM,
			Profile: cfg.Profile, Drift: -1}
		if pinned, err := pinnedRelease(c.ConsumerRoot, cfg); err == nil {
			r.Pinned = pinned
		} else {
			r.PinError = err.Error()
		}
		if latest, err := latestRelease(cfg); err == nil {
			r.Latest = latest
		} else {
			r.LatestError = err.Error()
		}
		if r.Pinned != "" && r.Latest != "" {
			lk, err1 := core.RequireVersion(r.Latest)
			pk, err2 := core.RequireVersion(r.Pinned)
			if err1 == nil && err2 == nil && pk.Less(lk) {
				r.Update = true
				r.Bump, _ = core.ClassifyBump(r.Pinned, r.Latest)
			}
		}
		mpath := filepath.Join(c.ConsumerRoot, core.VendkitDir,
			cfg.SliceName+"-manifest.json")
		if _, err := os.Stat(mpath); err == nil {
			if report, err := core.GateCheck(c.ConsumerRoot, []string{mpath}); err == nil {
				r.Drift = len(report.Findings)
			}
		}
		rows = append(rows, r)
	}
	if c.JSON {
		doc, _ := json.MarshalIndent(rows, "", "  ")
		fmt.Println(string(doc))
		return 0, nil
	}
	for _, r := range rows {
		pinned := r.Pinned
		if pinned == "" {
			pinned = "?"
		}
		line := fmt.Sprintf("%-12s pinned %-10s", r.Slice, pinned)
		if r.Latest != "" {
			line += fmt.Sprintf(" latest %-10s", r.Latest)
			if r.Update {
				line += fmt.Sprintf(" UPDATE AVAILABLE (%s)", r.Bump)
			} else {
				line += " up to date"
			}
		} else {
			line += " latest unknown"
		}
		if r.Drift > 0 {
			line += fmt.Sprintf("  DRIFT: %d finding(s) — run `vendkit gate`", r.Drift)
		} else if r.Drift == 0 {
			line += "  clean"
		}
		line += fmt.Sprintf("  [ci: %s]", r.CI)
		fmt.Println(line)
		for _, e := range []string{r.PinError, r.LatestError} {
			if e != "" {
				fmt.Printf("  ! %s\n", e)
			}
		}
	}
	return 0, nil
}

// -- diff ----------------------------------------------------------------------------

// cmdDiff: what would `update` change? Unified diff of every file apply
// would write, against a throwaway checkout of the target release.
func cmdDiff(args []string, surface ci.Surface) (int, error) {
	fs := newFlagSet("diff")
	var c commonFlags
	slice := fs.String("slice", "", "")
	target := fs.String("target", "", "")
	addCommon(fs, &c, false, true, false)
	if err := parseFlags(fs, args); err != nil {
		return 0, err
	}
	cfg, err := sliceOrOnly(c.ConsumerRoot, *slice)
	if err != nil {
		return 0, err
	}
	resolvedTarget := *target
	if resolvedTarget == "" {
		resolvedTarget, err = latestRelease(cfg)
		if err != nil {
			return 0, err
		}
		if resolvedTarget == "" {
			return 0, core.Usagef("publisher has no qualifying releases")
		}
	}
	pinned, err := pinnedRelease(c.ConsumerRoot, cfg)
	if err != nil {
		return 0, err
	}
	if *target == "" && resolvedTarget == pinned {
		fmt.Printf("%s: up to date at %s\n", cfg.SliceName, pinned)
		return 0, nil
	}
	pub, err := fetchedPublisher(cfg, resolvedTarget)
	if err != nil {
		return 0, err
	}
	defer os.RemoveAll(pub)
	decl, err := core.LoadExportDecl(filepath.Join(pub, core.DefaultDecl))
	if err != nil {
		return 0, err
	}
	changes, err := core.Preview(pub, c.ConsumerRoot, decl, true)
	if err != nil {
		return 0, err
	}
	fmt.Printf("# %s: %s → %s (%d file(s) would change)\n",
		cfg.SliceName, pinned, resolvedTarget, len(changes))
	for _, change := range changes {
		if !utf8.Valid(change.New) || (change.HasOld && !utf8.Valid(change.Old)) {
			fmt.Printf("Binary file %s differs\n", change.ConsumerPath)
			continue
		}
		fromFile := "a/" + change.ConsumerPath
		if !change.HasOld {
			fromFile += " (new file)"
		}
		fmt.Print(core.UnifiedDiff(string(change.Old), string(change.New),
			fromFile, "b/"+change.ConsumerPath))
	}
	return 0, nil
}

// -- update --------------------------------------------------------------------------

// cmdUpdate: the whole upgrade, human-invoked. --local (default): apply to
// the working tree + advance pins, you review and commit. --pr: the full
// sync lane against a fetched checkout. NOTE: the human tier runs the
// INSTALLED engine against the fetched target tree — a documented INV-6
// relaxation guarded by schema-version gating (cli spec).
func cmdUpdate(args []string, surface ci.Surface) (int, error) {
	fs := newFlagSet("update")
	var c commonFlags
	slice := fs.String("slice", "", "")
	target := fs.String("target", "", "")
	pr := fs.Bool("pr", false, "")
	fs.Bool("local", true, "")
	baseBranch := fs.String("base-branch", "main", "")
	addCommon(fs, &c, false, true, false)
	if err := parseFlags(fs, args); err != nil {
		return 0, err
	}
	cfg, err := sliceOrOnly(c.ConsumerRoot, *slice)
	if err != nil {
		return 0, err
	}
	pinned, err := pinnedRelease(c.ConsumerRoot, cfg)
	if err != nil {
		return 0, err
	}
	resolvedTarget := *target
	if resolvedTarget == "" {
		resolvedTarget, err = latestRelease(cfg)
		if err != nil {
			return 0, err
		}
		if resolvedTarget == "" {
			return 0, core.Usagef("publisher has no qualifying releases")
		}
	}
	if resolvedTarget == pinned {
		fmt.Printf("%s: already at %s\n", cfg.SliceName, resolvedTarget)
		return 0, nil
	}
	pub, err := fetchedPublisher(cfg, resolvedTarget)
	if err != nil {
		return 0, err
	}
	defer os.RemoveAll(pub)
	if *pr {
		return syncPipeline(syncPipelineOpts{
			Slice: cfg.SliceName, BaseBranch: *baseBranch,
			ReconcileScope: true, ConsumerRoot: c.ConsumerRoot,
			PublisherRoot: pub, ExportDecl: core.DefaultDecl,
		}, surface)
	}
	decl, err := core.LoadExportDecl(filepath.Join(pub, core.DefaultDecl))
	if err != nil {
		return 0, err
	}
	retracted := append([]string{}, decl.Retracted...)
	if url, err := core.CloneURL(cfg.PublisherSCM, cfg.PublisherRepo); err == nil {
		if tags, err := core.ListReleaseTags(url); err == nil {
			names := make([]string, len(tags))
			for i, t := range tags {
				names[i] = t.Name
			}
			if newest := core.Latest(names, "rc", nil); newest != "" {
				retracted = append(retracted, core.RetractedAtNewest(url, newest)...)
			}
		}
	}
	newer, err := core.IsNewer(pinned, resolvedTarget, retracted)
	if err != nil {
		return 0, err
	}
	if !newer {
		fmt.Printf("%s: %s is not newer than %s\n",
			cfg.SliceName, resolvedTarget, pinned)
		return 0, nil
	}
	report, err := core.Materialise(pub, c.ConsumerRoot, decl,
		resolvedTarget, true, true)
	if err != nil {
		return 0, err
	}
	for _, rel := range cfg.PinFiles {
		f := filepath.Join(c.ConsumerRoot, filepath.FromSlash(rel))
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		updated := strings.ReplaceAll(string(data),
			"refs/tags/"+pinned, "refs/tags/"+resolvedTarget)
		if err := os.WriteFile(f, []byte(updated), 0o644); err != nil {
			return 0, core.Errf("advance pin in %s: %v", rel, err)
		}
	}
	for _, group := range []struct {
		Label string
		Paths []string
	}{
		{"updated", report.Updated},
		{"added", report.Added},
		{"seeded", report.Seeded},
		{"removed upstream (delete when you commit)", report.RemovedUpstream},
		{"seed retired (file is yours)", report.SeedRetired},
		{"template updated (informational)", report.TemplateUpdated},
	} {
		for _, p := range group.Paths {
			fmt.Printf("%s: %s\n", group.Label, p)
		}
	}
	fmt.Printf("\n%s: %s → %s applied to the working tree (manifest + pins "+
		"advanced). Review and commit; the gate re-verifies your PR (INV-1).\n",
		cfg.SliceName, pinned, resolvedTarget)
	return 0, nil
}

// -- explain -------------------------------------------------------------------------

var explanations = map[string]string{
	// gate findings
	"changed": "A vendored file's content or exec bit differs from the " +
		"manifest. Vendored files change only via sync PRs (INV-10). Fix: " +
		"revert the edit (`git checkout -- <path>`); to change it for real, " +
		"contribute upstream and let a release deliver it.",
	"removed": "A manifest-tracked file is missing. Restore it, or if the " +
		"publisher retired it, a sync PR will drop it from tracking.",
	"collision": "Two slices claim the same consumer path (INV-7). One " +
		"publisher must rename or exclude; a prefix-namespace adapter is the " +
		"usual fix.",
	// watch findings
	"update-available": "The publisher released something newer than your " +
		"pin. Run `vendkit diff` to inspect, `vendkit update` to adopt, or " +
		"wait for the scheduled sync PR.",
	"tag-moved": "The pinned tag no longer resolves to the commit your " +
		"manifest recorded — possible tampering (INV-5). Sync refuses until " +
		"resolved. Contact the publisher; a legitimate fix ships as a NEW " +
		"release, never a re-tag.",
	"pin-unreadable": "The pin pattern found no parsable version in " +
		"pin.files[0]. Fix the reference line or the pattern in " +
		".vendkit/<slice>.yml.",
	"no-releases": "The publisher has no qualifying release tags on your " +
		"channel. Benign for new publishers; check watch.channel otherwise.",
	// refusals
	"retracted": "The target release was retracted by the publisher — do " +
		"not adopt it. Wait for (or request) the fixed, newer release.",
	"stale-manifest": "The committed publisher manifest does not match the " +
		"tree. Run `vendkit generate`, commit, and cut the release again.",
	"tag-exists": "That version is already released; tags are immutable " +
		"(INV-5). Pick the next version.",
	"bump-too-small": "The surface delta demands a bigger version bump " +
		"(removals ⇒ MAJOR, additions ⇒ MINOR).",
	"migration-missing": "The release removes exported files but ships no " +
		"migrations/ entry for this version. Add one, or record an override " +
		"with --no-migrations-needed.",
	"not-newer": "The requested version does not exceed the latest release.",
	// sync report classes
	"removed-upstream": "The target release no longer exports this file. " +
		"Sync left it on disk; delete it in the sync PR under review " +
		"(INV-4: nothing is auto-deleted).",
	"seeded": "A scaffold-once template landed because the path did not " +
		"exist. It is yours now: edit or delete freely (DR-0013).",
	"seed-retired": "The publisher retired a seed template. Your copy is " +
		"untouched; it simply stops being tracked.",
	"template-updated": "The upstream template behind one of your seeded " +
		"files changed. Informational only — your copy was not touched.",
	// conformance statuses
	"attested": "The rule depends on a platform fact that is not decidable " +
		"from the tree; your slice config asserts it. Verify via " +
		"`conformance --verify-attestations` with a fact-verify handler.",
	"waived": "You waived this (waivable) rule with a recorded reason in " +
		"the slice config.",
	"skipped": "Not applicable in this configuration (e.g. pipeline rules " +
		"under ci: none — manual mode forfeits automated enforcement).",
}

func cmdExplain(args []string, surface ci.Surface) (int, error) {
	fs := newFlagSet("explain")
	var c commonFlags
	addCommon(fs, &c, false, false, false)
	if err := parseFlags(fs, args); err != nil {
		return 0, err
	}
	topic := ""
	if fs.NArg() > 0 {
		topic = fs.Arg(0)
	}
	if topic == "" || topic == "list" {
		for _, key := range sortedKeys(explanations) {
			fmt.Println(key)
		}
		return 0, nil
	}
	text, ok := explanations[topic]
	if !ok {
		return 0, core.Usagef("unknown topic %q — `vendkit explain list`", topic)
	}
	fmt.Printf("%s: %s\n", topic, text)
	return 0, nil
}
