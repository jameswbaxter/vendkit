// Machine-tier commands — behaviour and output are contractually identical
// to the reference CLI (DR-0017).
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	vendkitassets "example.org/vendkit"
	"example.org/vendkit/internal/ci"
	"example.org/vendkit/internal/core"
)

// -- publisher ------------------------------------------------------------------

func cmdGenerate(args []string, surface ci.Surface) (int, error) {
	fs := newFlagSet("generate")
	var c commonFlags
	check := fs.Bool("check", false, "")
	root := fs.String("root", ".", "")
	addCommon(fs, &c, true, false, false)
	if err := parseFlags(fs, args); err != nil {
		return 0, err
	}
	decl, err := core.LoadExportDecl(c.ExportDecl)
	if err != nil {
		return 0, err
	}
	fresh, err := core.BuildPublisherManifest(decl, *root)
	if err != nil {
		return 0, err
	}
	manifestPath := filepath.Join(*root, decl.ManifestName)
	if *check {
		committed, err := core.LoadManifest(manifestPath)
		if err != nil {
			if _, isUsage := err.(*core.UsageError); isUsage {
				surface.EmitError(decl.ManifestName + " missing — run generate")
				return 1, nil
			}
			return 0, err
		}
		if !core.ManifestsEqual(fresh, committed) {
			surface.EmitError(decl.ManifestName + " is stale — run generate")
			surface.EmitOutput("fresh", "false")
			return 1, nil
		}
		surface.EmitOutput("fresh", "true")
		return 0, nil
	}
	if err := core.DumpManifest(fresh, manifestPath); err != nil {
		return 0, err
	}
	entries := len(fresh["entries"].([]any))
	surface.EmitOutput("entries", fmt.Sprint(entries))
	return 0, nil
}

func cmdRelease(args []string, surface ci.Surface) (int, error) {
	fs := newFlagSet("release")
	var c commonFlags
	bump := fs.String("bump", "", "")
	version := fs.String("version", "", "")
	summary := fs.String("summary", "", "")
	noMigrations := fs.Bool("no-migrations-needed", false, "")
	dryRun := fs.Bool("dry-run", false, "")
	root := fs.String("root", ".", "")
	addCommon(fs, &c, true, false, false)
	if err := parseFlags(fs, args); err != nil {
		return 0, err
	}
	decl, err := core.LoadExportDecl(c.ExportDecl)
	if err != nil {
		return 0, err
	}
	result, err := core.Cut(*root, decl, *bump, *version, *summary,
		*noMigrations, *dryRun)
	if err != nil {
		return 0, err
	}
	surface.EmitOutput("version", result.Version)
	surface.EmitOutput("previous", result.Previous)
	surface.EmitOutput("surface-delta",
		fmt.Sprintf("+%d/-%d", result.Added, result.Removed))
	return 0, nil
}

// -- consumer PR path -------------------------------------------------------------

func cmdGate(args []string, surface ci.Surface) (int, error) {
	fs := newFlagSet("gate")
	var c commonFlags
	strict := fs.Bool("strict", false, "")
	fs.Bool("all", true, "")
	manifest := fs.String("manifest", "", "")
	addCommon(fs, &c, false, true, false)
	if err := parseFlags(fs, args); err != nil {
		return 0, err
	}
	var paths []string
	if *manifest != "" {
		paths = []string{*manifest}
	} else {
		paths = core.DiscoverManifests(c.ConsumerRoot)
	}
	if len(paths) == 0 {
		return 0, core.Usagef("no manifests found under .vendkit/")
	}
	report, err := core.GateCheck(c.ConsumerRoot, paths)
	if err != nil {
		return 0, err
	}
	for _, f := range report.Findings {
		fmt.Println(strings.TrimRight(fmt.Sprintf("%s: %s [%s] %s",
			f.Kind, f.ConsumerPath, f.SliceName, f.Detail), " "))
	}
	surface.EmitOutput("findings", fmt.Sprint(len(report.Findings)))
	surface.EmitOutput("checked", fmt.Sprint(report.Checked))
	if c.JSON {
		findings := report.Findings
		if findings == nil {
			findings = []core.Finding{}
		}
		doc, _ := json.Marshal(findings)
		fmt.Println(string(doc))
	}
	if len(report.Findings) > 0 && *strict {
		surface.EmitError(fmt.Sprintf(
			"gate: %d finding(s) across %d manifest(s) — vendored files may "+
				"only change via sync PRs", len(report.Findings), len(paths)))
		return 1, nil
	}
	return 0, nil
}

func cmdMigrationsVerify(args []string, surface ci.Surface) (int, error) {
	fs := newFlagSet("migrations-verify")
	var c commonFlags
	obligations := fs.String("obligations", "", "")
	addCommon(fs, &c, false, true, false)
	if err := parseFlags(fs, args); err != nil {
		return 0, err
	}
	if *obligations == "" {
		return 0, core.Usagef("--obligations is required")
	}
	doc, err := core.LoadObligations(*obligations)
	if err != nil {
		return 0, err
	}
	report, err := core.VerifyMigrations(c.ConsumerRoot, doc)
	if err != nil {
		return 0, err
	}
	for _, failure := range report.Failures {
		fmt.Printf("unmet: %s\n", failure)
	}
	surface.EmitOutput("obligations", fmt.Sprint(report.Checked))
	surface.EmitOutput("unmet", fmt.Sprint(len(report.Failures)))
	if len(report.Failures) > 0 {
		surface.EmitError(fmt.Sprintf("%d migration obligation(s) unmet",
			len(report.Failures)))
		return 1, nil
	}
	return 0, nil
}

// -- sync lane ----------------------------------------------------------------------

func cmdSync(args []string, surface ci.Surface) (int, error) {
	fs := newFlagSet("sync")
	var c commonFlags
	apply := fs.Bool("apply", false, "")
	fs.Bool("check", false, "")
	target := fs.String("target", "", "")
	reconcile := fs.Bool("reconcile-scope", false, "")
	porcelain := fs.Bool("porcelain", false, "")
	addCommon(fs, &c, true, true, true)
	if err := parseFlags(fs, args); err != nil {
		return 0, err
	}
	if *target == "" {
		return 0, core.Usagef("--target is required")
	}
	decl, err := loadDeclFrom(c.PublisherRoot, c.ExportDecl)
	if err != nil {
		return 0, err
	}
	report, err := core.Materialise(c.PublisherRoot, c.ConsumerRoot, decl,
		*target, *apply, *reconcile)
	if err != nil {
		return 0, err
	}
	if !*porcelain {
		for _, x := range report.Updated {
			fmt.Printf("updated: %s\n", x)
		}
		for _, x := range report.Added {
			fmt.Printf("added: %s\n", x)
		}
		for _, x := range report.RemovedUpstream {
			fmt.Printf("removed-upstream: %s (left on disk — delete in this PR)\n", x)
		}
		for _, x := range report.Seeded {
			fmt.Printf("seeded: %s (scaffold-once; yours to change from now on)\n", x)
		}
		for _, x := range report.SeedRetired {
			fmt.Printf("seed-retired: %s (template gone upstream; file is yours, nothing to delete)\n", x)
		}
		for _, x := range report.TemplateUpdated {
			fmt.Printf("template-updated: %s (informational; your copy is untouched)\n", x)
		}
	}
	surface.EmitOutput("changed", boolStr(report.Changed()))
	return 0, nil
}

func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

func publisherTag(publisherRoot string) (string, error) {
	cmd := exec.Command("git", "describe", "--tags", "--exact-match", "HEAD")
	cmd.Dir = publisherRoot
	out, err := cmd.Output()
	if err != nil {
		return "", core.Errf("publisher checkout is not at a release tag — " +
			"the sync pipeline must pin the publisher to refs/tags/vX.Y.Z (INV-6)")
	}
	return strings.TrimSpace(string(out)), nil
}

type syncPipelineOpts struct {
	Slice          string
	BaseBranch     string
	ConsumerRepo   string
	ReconcileScope bool
	ConsumerRoot   string
	PublisherRoot  string
	ExportDecl     string
}

func cmdSyncPipeline(args []string, surface ci.Surface) (int, error) {
	fs := newFlagSet("sync-pipeline")
	var c commonFlags
	slice := fs.String("slice", "", "")
	baseBranch := fs.String("base-branch", "main", "")
	consumerRepo := fs.String("consumer-repo", "", "")
	reconcile := fs.Bool("reconcile-scope", true, "")
	addCommon(fs, &c, true, true, true)
	if err := parseFlags(fs, args); err != nil {
		return 0, err
	}
	if *slice == "" {
		return 0, core.Usagef("--slice is required")
	}
	return syncPipeline(syncPipelineOpts{
		Slice: *slice, BaseBranch: *baseBranch, ConsumerRepo: *consumerRepo,
		ReconcileScope: *reconcile, ConsumerRoot: c.ConsumerRoot,
		PublisherRoot: c.PublisherRoot, ExportDecl: c.ExportDecl,
	}, surface)
}

// syncPipeline: full sync-lane orchestration (sync spec §3): resolve
// versions, probe, apply, advance pins, branch, push, hand ONE reviewed-PR
// intent to the configured PR handler (DR-0014).
func syncPipeline(opts syncPipelineOpts, surface ci.Surface) (int, error) {
	decl, err := loadDeclFrom(opts.PublisherRoot, opts.ExportDecl)
	if err != nil {
		return 0, err
	}
	cfg, err := core.FindSliceConfig(opts.ConsumerRoot, opts.Slice)
	if err != nil {
		return 0, err
	}
	if cfg == nil {
		return 0, core.Usagef("no slice config for %q under .vendkit/", opts.Slice)
	}

	// PINNED: the manifest's provenance is authoritative for what is
	// vendored; ReadPin is the pre-first-sync bootstrap fallback.
	manifestPath := filepath.Join(opts.ConsumerRoot, core.VendkitDir, decl.ManifestName)
	manifest, err := core.LoadManifest(manifestPath)
	if err != nil {
		return 0, err
	}
	source := manifest["source"]
	pinned := ""
	if sm, ok := source.(map[string]any); ok {
		if r, ok := sm["release"].(string); ok {
			pinned = r
		}
	}
	if pinned == "" {
		pinned, err = core.ReadPin(opts.ConsumerRoot, cfg)
		if err != nil {
			return 0, err
		}
	}
	// TARGET: the release this pipeline's publisher checkout is pinned to.
	target, err := publisherTag(opts.PublisherRoot)
	if err != nil {
		return 0, err
	}

	// Retractions live at the NEWEST release's declaration (releases spec
	// §4). Union the target declaration's list with a best-effort read of
	// the newest one — over the git protocol, no vendor API (DR-0015).
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

	newer, err := core.IsNewer(pinned, target, retracted)
	if err != nil {
		return 0, err // Refusal (retracted) propagates: exit 3
	}
	if !newer {
		surface.EmitOutput("update-available", "false")
		surface.EmitOutput("changed", "false")
		return 0, nil
	}
	surface.EmitOutput("update-available", "true")

	// Probe (INV-3: a crash can never masquerade as staleness).
	probe, err := core.Materialise(opts.PublisherRoot, opts.ConsumerRoot,
		decl, target, false, false)
	if err != nil {
		return 0, err
	}
	if !probe.Changed() {
		surface.EmitOutput("changed", "false")
		return 0, nil
	}
	report, err := core.Materialise(opts.PublisherRoot, opts.ConsumerRoot,
		decl, target, true, opts.ReconcileScope)
	if err != nil {
		return 0, err
	}
	surface.EmitOutput("changed", "true")

	// Advance every pin line in lockstep (sync spec §3 step 4). Under
	// ci: none there are no pin files — provenance is the pin.
	for _, rel := range cfg.PinFiles {
		f := filepath.Join(opts.ConsumerRoot, filepath.FromSlash(rel))
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		updated := strings.ReplaceAll(string(data),
			"refs/tags/"+pinned, "refs/tags/"+target)
		if err := os.WriteFile(f, []byte(updated), 0o644); err != nil {
			return 0, core.Errf("advance pin in %s: %v", rel, err)
		}
	}

	branch := fmt.Sprintf("vendkit/%s/sync-%s-to-%s", cfg.SliceName, pinned, target)
	entries, err := core.LoadMigrationEntries(opts.PublisherRoot)
	if err != nil {
		return 0, err
	}
	applicable, _, err := core.ResolveMigrations(entries, pinned, target,
		cfg.Profile, nil)
	if err != nil {
		return 0, err
	}
	freshManifest, err := core.LoadManifest(manifestPath)
	if err != nil {
		return 0, err
	}
	sourceMap, _ := freshManifest["source"].(map[string]any)
	body := prBody(cfg.SliceName, pinned, target, report, applicable,
		sourceMap, cfg.SeedNotes)

	git := func(a ...string) error {
		cmd := exec.Command("git", append([]string{
			"-c", "user.name=vendkit-sync",
			"-c", "user.email=vendkit-sync@invalid"}, a...)...)
		cmd.Dir = opts.ConsumerRoot
		out, err := cmd.CombinedOutput()
		if err != nil {
			return core.Errf("git %s: %s", strings.Join(a, " "),
				strings.TrimSpace(string(out)))
		}
		return nil
	}
	if err := git("checkout", "-B", branch); err != nil {
		return 0, err
	}
	if err := git("add", "-A"); err != nil {
		return 0, err
	}
	if err := git("commit", "-m",
		fmt.Sprintf("sync(%s): %s -> %s", cfg.SliceName, pinned, target)); err != nil {
		return 0, err
	}
	if err := git("push", "--force", "origin", branch); err != nil {
		return 0, err
	}

	// PR delivery is a handler concern (DR-0014): the engine composes the
	// intent; the handler owns the vendor API. The deterministic branch
	// name is the idempotency key (protocol spec §3).
	intent := map[string]any{
		"head_branch": branch,
		"base_branch": opts.BaseBranch,
		"title":       fmt.Sprintf("sync(%s): %s → %s", cfg.SliceName, pinned, target),
		"body_md":     body,
		"slice":       cfg.SliceName,
	}
	if opts.ConsumerRepo != "" {
		intent["repo"] = opts.ConsumerRepo
	}
	command := core.ResolveHandler("pr", cfg)
	if command == nil {
		// Unwired (e.g. ci: none, fully manual): the branch is pushed and
		// the intent is printed — the human delivers the PR themselves.
		surface.EmitOutput("pr-delivered", "false")
		doc, _ := json.Marshal(intent)
		surface.EmitOutput("pr-intent", string(doc))
		return 0, nil
	}
	facts, err := core.InvokeHandler(command, "pr", intent, opts.ConsumerRoot)
	if err != nil {
		return 0, err
	}
	surface.EmitOutput("pr-delivered", "true")
	surface.EmitOutput("pr-url", facts["url"])
	return 0, nil
}

func prBody(sliceName, pinned, target string, report *core.SyncReport,
	applicable []map[string]any, source map[string]any, seedNotes string) string {
	commit := ""
	if source != nil {
		if c, ok := source["commit"].(string); ok && len(c) >= 12 {
			commit = c[:12]
		}
	}
	lines := []string{
		fmt.Sprintf("Sync of slice `%s`: **%s → %s**.", sliceName, pinned, target),
		"",
		fmt.Sprintf("- updated: %d", len(report.Updated)),
		fmt.Sprintf("- added (scope): %d", len(report.Added)),
		fmt.Sprintf("- removed upstream (left on disk — delete here): %d",
			len(report.RemovedUpstream)),
		fmt.Sprintf("- source commit: `%s`", commit),
	}
	for _, c := range report.RemovedUpstream {
		lines = append(lines, fmt.Sprintf("  - `%s`", c))
	}
	if len(report.Seeded) > 0 {
		lines = append(lines, "", "**Seeded in this PR** (scaffold-once — "+
			"yours to change from now on, the gate never checks them):")
		for _, c := range report.Seeded {
			lines = append(lines, fmt.Sprintf("- `%s`", c))
		}
	}
	if len(report.SeedRetired) > 0 {
		lines = append(lines, "", "**Seed templates retired upstream** (your "+
			"copies are unaffected; they simply stop being tracked):")
		for _, c := range report.SeedRetired {
			lines = append(lines, fmt.Sprintf("- `%s`", c))
		}
	}
	if seedNotes == "informational" && len(report.TemplateUpdated) > 0 {
		lines = append(lines, "", "**Seeded files whose upstream template "+
			"changed** — your copies are yours and were not touched; review "+
			"the publisher's template manually if interested:")
		for _, c := range report.TemplateUpdated {
			lines = append(lines, fmt.Sprintf("- `%s`", c))
		}
	}
	if len(applicable) > 0 {
		lines = append(lines, "", "**Applicable migrations in this window** "+
			"(judgment-bearing; see work items):")
		for _, m := range applicable {
			id, _ := m["id"].(string)
			kind, _ := m["kind"].(string)
			summary, _ := m["summary"].(string)
			lines = append(lines, fmt.Sprintf("- `%s` (%s): %s", id, kind, summary))
		}
	}
	lines = append(lines, "", "Review per your normal rules; the gate lane "+
		"re-verifies this PR (INV-1). Never auto-merged (INV-10).")
	return strings.Join(lines, "\n")
}

// -- watch / migrations / conformance -------------------------------------------

func cmdWatch(args []string, surface ci.Surface) (int, error) {
	fs := newFlagSet("watch")
	var c commonFlags
	slice := fs.String("slice", "", "")
	dryRun := fs.Bool("dry-run", false, "")
	noHandoff := fs.Bool("no-handoff", false, "")
	addCommon(fs, &c, false, true, false)
	if err := parseFlags(fs, args); err != nil {
		return 0, err
	}
	report, err := core.Watch(c.ConsumerRoot, *slice, *dryRun)
	if err != nil {
		return 0, err
	}
	markdown := core.RenderWatchReport(report)
	surface.EmitSummary(markdown)
	actionable := report.Actionable()
	surface.EmitOutput("findings", fmt.Sprint(len(actionable)))
	if c.JSON {
		findings := report.Findings
		if findings == nil {
			findings = []core.WatchFinding{}
		}
		doc, _ := json.Marshal(findings)
		fmt.Println(string(doc))
	}
	if *dryRun || *noHandoff || len(actionable) == 0 {
		return 0, nil
	}
	configs, err := core.DiscoverSliceConfigs(c.ConsumerRoot)
	if err != nil {
		return 0, err
	}
	byName := map[string]*core.SliceConfig{}
	for _, cfg := range configs {
		byName[cfg.SliceName] = cfg
	}
	unwired := false
	for _, f := range actionable {
		cfg := byName[f.SliceName]
		command := core.ResolveHandler("handoff", cfg)
		if command == nil {
			unwired = true
			continue // report-only: findings are on stdout/summary already
		}
		key := cfg.HandoffDedupKey
		if f.Kind != "update-available" {
			key += "-integrity" // integrity findings never share the update item
		}
		title := fmt.Sprintf("vendkit(%s): %s", f.SliceName, f.Kind)
		if f.Kind == "update-available" {
			title = fmt.Sprintf("vendkit(%s): update available %s → %s",
				f.SliceName, f.Pinned, f.Latest)
		}
		facts, err := core.InvokeHandler(command, "handoff", map[string]any{
			"dedup_key": key, "title": title, "body_md": markdown,
			"slice": f.SliceName}, c.ConsumerRoot)
		if err != nil {
			return 0, err
		}
		surface.EmitOutput("item-"+f.SliceName, facts["url"])
	}
	if unwired {
		surface.EmitOutput("handoff", "unwired")
	} else {
		surface.EmitOutput("handoff", "delivered")
	}
	return 0, nil
}

func cmdMigrations(args []string, surface ci.Surface) (int, error) {
	fs := newFlagSet("migrations")
	var c commonFlags
	pinned := fs.String("pinned", "", "")
	target := fs.String("target", "", "")
	profile := fs.String("profile", "", "")
	addCommon(fs, &c, false, false, true)
	if err := parseFlags(fs, args); err != nil {
		return 0, err
	}
	if *pinned == "" || *target == "" {
		return 0, core.Usagef("--pinned and --target are required")
	}
	entries, err := core.LoadMigrationEntries(c.PublisherRoot)
	if err != nil {
		return 0, err
	}
	applicable, obligations, err := core.ResolveMigrations(entries, *pinned,
		*target, *profile, nil)
	if err != nil {
		return 0, err
	}
	surface.EmitOutput("count", fmt.Sprint(len(applicable)))
	var ids []string
	for _, m := range applicable {
		if id, ok := m["id"].(string); ok {
			ids = append(ids, id)
		}
	}
	surface.EmitOutput("ids", strings.Join(ids, ","))
	stripped := make([]map[string]any, 0, len(applicable))
	for _, m := range applicable {
		s := map[string]any{}
		for k, v := range m {
			if !strings.HasPrefix(k, "_") {
				s[k] = v
			}
		}
		stripped = append(stripped, s)
	}
	doc, _ := json.MarshalIndent(map[string]any{
		"applicable": stripped, "obligations": obligations}, "", "  ")
	fmt.Println(string(doc))
	return 0, nil
}

func cmdConformance(args []string, surface ci.Surface) (int, error) {
	fs := newFlagSet("conformance")
	var c commonFlags
	slice := fs.String("slice", "", "")
	strict := fs.Bool("strict", false, "")
	rulesPath := fs.String("rules", "", "")
	verifyAttest := fs.Bool("verify-attestations", false, "")
	addCommon(fs, &c, false, true, false)
	if err := parseFlags(fs, args); err != nil {
		return 0, err
	}
	if *slice == "" {
		return 0, core.Usagef("--slice is required")
	}
	cfg, err := core.FindSliceConfig(c.ConsumerRoot, *slice)
	if err != nil {
		return 0, err
	}
	if cfg == nil {
		return 0, core.Usagef("no slice config for %q under .vendkit/", *slice)
	}
	coreRules, err := vendkitassets.FS.ReadFile("conformance/core-rules.yml")
	if err != nil {
		return 0, core.Errf("embedded core rules: %v", err)
	}
	sources := []core.RuleSource{{Name: "core-rules.yml", Data: coreRules}}
	if *rulesPath != "" {
		data, err := os.ReadFile(*rulesPath)
		if err != nil {
			return 0, core.Usagef("cannot read %s: %v", *rulesPath, err)
		}
		sources = append(sources, core.RuleSource{Name: *rulesPath, Data: data})
	}
	rules, err := core.LoadRules(sources)
	if err != nil {
		return 0, err
	}
	report := core.Evaluate(c.ConsumerRoot, cfg, rules)
	if *verifyAttest {
		// Promote attested → pass via the fact-verify handler; a false
		// verdict is a fail; unknown stays attested (conformance spec §4).
		command := core.ResolveHandler("fact-verify", cfg)
		if command == nil {
			return 0, core.Usagef("--verify-attestations needs a fact-verify " +
				"handler (handlers.fact-verify in the slice config)")
		}
		for _, r := range report.Results {
			if r.Status != "attested" {
				continue
			}
			facts, err := core.InvokeHandler(command, "fact-verify",
				map[string]any{"fact": r.Detail, "slice": cfg.SliceName},
				c.ConsumerRoot)
			if err != nil {
				return 0, err
			}
			switch facts["verdict"] {
			case "true":
				r.Status, r.Detail = "pass", r.Detail+" (verified)"
			case "false":
				r.Status, r.Detail = "fail", r.Detail+" (verification refuted)"
			}
		}
	}
	for _, r := range report.Results {
		fmt.Println(strings.TrimRight(
			fmt.Sprintf("%-9s %-28s %s", r.Status, r.RuleID, r.Detail), " "))
	}
	gaps := report.Gaps()
	surface.EmitOutput("gap-count", fmt.Sprint(len(gaps)))
	if c.JSON {
		doc, _ := json.Marshal(report.Results)
		fmt.Println(string(doc))
	}
	if len(gaps) > 0 && *strict {
		surface.EmitError(fmt.Sprintf("%d conformance gap(s)", len(gaps)))
		return 1, nil
	}
	return 0, nil
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
