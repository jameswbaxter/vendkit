// Conformance: rule spec evaluation with dialect-keyed detector bindings
// (conformance spec; dialect selection by the slice config's `ci:` axis,
// never env-sniffed — DR-0015).

package core

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

var severities = []string{"mandatory", "waivable", "advisory"}

type RuleResult struct {
	RuleID   string `json:"rule_id"`
	Title    string `json:"title"`
	Severity string `json:"severity"`
	Status   string `json:"status"`
	Detail   string `json:"detail"`
}

func (r *RuleResult) IsGap() bool { return r.Status == "fail" || r.Status == "error" }

type ConformanceReport struct {
	Results []*RuleResult
}

func (r *ConformanceReport) Gaps() []*RuleResult {
	var out []*RuleResult
	for _, res := range r.Results {
		if res.IsGap() {
			out = append(out, res)
		}
	}
	return out
}

type RuleSource struct {
	Name string
	Data []byte
}

func LoadRules(sources []RuleSource) ([]map[string]any, error) {
	var rules []map[string]any
	seen := map[string]bool{}
	for _, src := range sources {
		data, err := ParseYAMLMap(src.Data, src.Name)
		if err != nil {
			return nil, err
		}
		if !schemaVersionIs(data, 1) {
			return nil, Usagef("%s: schema_version must be 1", src.Name)
		}
		for _, raw := range getList(data, "rules") {
			rule, _ := raw.(map[string]any)
			rid := getStr(rule, "id")
			if rid == "" || seen[rid] {
				return nil, Usagef("%s: missing or duplicate rule id %q", src.Name, rid)
			}
			if !contains(severities, getStr(rule, "severity")) {
				return nil, Usagef("%s: rule %s: bad severity", src.Name, rid)
			}
			if getStr(getMap(rule, "detector"), "kind") == "" {
				return nil, Usagef("%s: rule %s: detector.kind required", src.Name, rid)
			}
			seen[rid] = true
			rules = append(rules, rule)
		}
	}
	return rules, nil
}

// -- pipeline discovery/parsing (dialect bindings) ------------------------------

type pipelineInfo struct {
	Path string
	Text string
	Data map[string]any
}

func pipelineFiles(consumerRoot, ci string) []string {
	var out []string
	switch ci {
	case "github-actions":
		for _, pat := range []string{"*.yml", "*.yaml"} {
			hits, _ := filepath.Glob(filepath.Join(consumerRoot, ".github", "workflows", pat))
			sort.Strings(hits)
			out = append(out, hits...)
		}
	case "azure-pipelines":
		top := filepath.Join(consumerRoot, "azure-pipelines.yml")
		if _, err := os.Stat(top); err == nil {
			out = append(out, top)
		}
		hits, _ := filepath.Glob(filepath.Join(consumerRoot, "azure-pipelines", "*.yml"))
		sort.Strings(hits)
		out = append(out, hits...)
	}
	return out // ci: none — no pipelines to parse
}

func loadPipelines(consumerRoot, ci string) []pipelineInfo {
	var infos []pipelineInfo
	for _, f := range pipelineFiles(consumerRoot, ci) {
		raw, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		data, err := ParseYAMLMap(raw, f)
		if err != nil {
			data = map[string]any{}
		}
		infos = append(infos, pipelineInfo{Path: f, Text: string(raw), Data: data})
	}
	return infos
}

// A component is "wired" when the pipeline invokes its CLI subcommand —
// true for every scaffold shape (direct call, composite action, or step
// template all bottom out in the invocation). "Pinned" when the file
// carries a release-tag reference.
var invokeRx = map[string]*regexp.Regexp{
	"gate":             regexp.MustCompile(`vendkit(\.cli)?\s+gate\b`),
	"sync":             regexp.MustCompile(`vendkit(\.cli)?\s+sync-pipeline\b`),
	"watch":            regexp.MustCompile(`vendkit(\.cli)?\s+watch\b`),
	"conformance":      regexp.MustCompile(`vendkit(\.cli)?\s+conformance\b`),
	"migration-verify": regexp.MustCompile(`vendkit(\.cli)?\s+migrations-verify\b`),
}

var pinRx = regexp.MustCompile(
	`(refs/tags/v\d+\.\d+\.\d+(-rc\.\d+)?\b)` +
		`|(@v\d+\.\d+\.\d+(-rc\.\d+)?\b)` +
		`|(@[0-9a-f]{40}\b)`)

func wired(info pipelineInfo, component string) (bool, bool) {
	rx, ok := invokeRx[component]
	if !ok || !rx.MatchString(info.Text) {
		return false, false
	}
	return true, pinRx.MatchString(info.Text)
}

// hasEvent: 1 = yes, 0 = no, -1 = not tree-decidable on this CI platform
// (→ attest degradation, conformance spec §3).
func hasEvent(info pipelineInfo, event, ci string) int {
	if ci == "github-actions" {
		// YAML 1.1 parsers may resolve bare `on` as a boolean key; the
		// normaliser stringifies it to "true".
		on, ok := info.Data["on"]
		if !ok {
			on = info.Data["true"]
		}
		keys := map[string]bool{}
		switch t := on.(type) {
		case string:
			keys[t] = true
		case []any:
			for _, item := range t {
				if s, isStr := item.(string); isStr {
					keys[s] = true
				}
			}
		case map[string]any:
			for k := range t {
				keys[k] = true
			}
		}
		want := "schedule"
		if event == "pull_request" {
			want = "pull_request"
		}
		if keys[want] {
			return 1
		}
		return 0
	}
	if event == "schedule" {
		if _, ok := info.Data["schedules"]; ok {
			return 1
		}
		return 0
	}
	return -1 // Azure Repos PR gating is a branch policy: not tree-decidable
}

// -- evaluation ----------------------------------------------------------------

func Evaluate(consumerRoot string, cfg *SliceConfig, rules []map[string]any) *ConformanceReport {
	report := &ConformanceReport{}
	waived := map[string]string{}
	for _, w := range cfg.Waivers {
		waived[getStr(w, "rule")] = getStr(w, "reason")
	}
	manifestPath := filepath.Join(consumerRoot, VendkitDir, cfg.SliceName+"-manifest.json")
	var manifest map[string]any
	if m, err := LoadManifest(manifestPath); err == nil {
		manifest = m
	}
	pipelines := loadPipelines(consumerRoot, cfg.CI)

	for _, rule := range rules {
		rid, severity := getStr(rule, "id"), getStr(rule, "severity")
		det := getMap(rule, "detector")
		title := getStr(rule, "title")
		if reason, isWaived := waived[rid]; isWaived {
			if severity == "waivable" {
				report.Results = append(report.Results,
					&RuleResult{rid, title, severity, "waived", reason})
				continue
			}
			report.Results = append(report.Results, &RuleResult{rid, title,
				severity, "fail", "rule is mandatory and cannot be waived"})
			continue
		}
		status, detail := detect(det, consumerRoot, cfg, manifest, pipelines)
		report.Results = append(report.Results,
			&RuleResult{rid, title, severity, status, detail})
	}
	return report
}

func detect(det map[string]any, consumerRoot string, cfg *SliceConfig,
	manifest map[string]any, pipelines []pipelineInfo) (string, string) {
	kind := getStr(det, "kind")

	switch kind {
	case "file-exists":
		path := strings.ReplaceAll(getStr(det, "path"), "<slice>", cfg.SliceName)
		if fi, err := os.Stat(filepath.Join(consumerRoot, filepath.FromSlash(path))); err == nil && !fi.IsDir() {
			return "pass", ""
		}
		return "fail", path + " missing"

	case "manifest-tracked":
		if manifest == nil {
			return "fail", "slice manifest missing"
		}
		want := getStr(det, "path")
		for _, e := range manifestEntries(manifest) {
			if getStr(e, "consumer_path") == want {
				return "pass", ""
			}
		}
		return "fail", want + " not tracked"

	case "profile-bound":
		if cfg.Profile != "" {
			return "pass", cfg.Profile
		}
		return "fail", "no profile declared in slice config"

	case "codeowners-covers":
		// Ownership is an SCM-axis concern: Azure Repos does not honour
		// CODEOWNERS — the equivalent intent is a required-reviewers branch
		// policy, not tree-decidable → attest (DR-0015).
		if cfg.SCM == "azure-repos" {
			att := "required_reviewers_policy"
			if cfg.Attestations[att] {
				return "attested", att
			}
			return "fail", fmt.Sprintf("CODEOWNERS is not honoured on "+
				"azure-repos; add a required-reviewers policy and record "+
				"attestation %q", att)
		}
		patterns, _ := strList(det["patterns"])
		return codeownersCovers(consumerRoot, patterns)

	case "attest":
		name := getStr(det, "attestation")
		if cfg.Attestations[name] {
			return "attested", name
		}
		return "fail", fmt.Sprintf("attestation %q not recorded", name)

	case "tool":
		tool := filepath.Join(consumerRoot, filepath.FromSlash(getStr(det, "path")))
		if fi, err := os.Stat(tool); err != nil || fi.IsDir() {
			return "skipped", "tool absent: " + getStr(det, "path")
		}
		args, _ := strList(det["args"])
		cmd := exec.Command(tool, args...)
		cmd.Dir = consumerRoot
		if err := cmd.Run(); err != nil {
			code := 1
			if ee, ok := err.(*exec.ExitError); ok {
				code = ee.ExitCode()
			}
			return "fail", fmt.Sprintf("tool exited %d", code)
		}
		return "pass", ""

	case "pipeline-wired":
		if cfg.CI == "none" {
			// Manual mode forfeits automated enforcement — say so, don't
			// hide it: `skipped` is visible in every report.
			return "skipped", "ci is 'none': orchestration is manual"
		}
		return pipelineWired(det, cfg, pipelines)

	case "paths-lockstep":
		if cfg.CI == "none" {
			return "skipped", "ci is 'none': no gate pipeline to filter"
		}
		return pathsLockstep(det, cfg, pipelines, manifest)
	}
	return "error", fmt.Sprintf("unknown detector kind %q", kind)
}

func codeownersCovers(root string, patterns []string) (string, string) {
	for _, loc := range []string{"CODEOWNERS", ".github/CODEOWNERS", "docs/CODEOWNERS"} {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(loc)))
		if err != nil {
			continue
		}
		var owned []string
		for _, ln := range strings.Split(string(data), "\n") {
			ln = strings.TrimSpace(ln)
			if ln == "" || strings.HasPrefix(ln, "#") {
				continue
			}
			owned = append(owned, strings.Fields(ln)[0])
		}
		var missing []string
		for _, p := range patterns {
			covered := false
			for _, o := range owned {
				stem := strings.TrimSuffix(strings.TrimSuffix(
					strings.Trim(o, "/"), "*"), "/")
				if strings.HasPrefix(p, stem) || PathMatch(p, strings.Trim(o, "/")) {
					covered = true
					break
				}
			}
			if !covered {
				missing = append(missing, p)
			}
		}
		if len(missing) == 0 {
			return "pass", ""
		}
		return "fail", "not covered: " + strings.Join(missing, ", ")
	}
	return "fail", "no CODEOWNERS file"
}

func pipelineWired(det map[string]any, cfg *SliceConfig, pipelines []pipelineInfo) (string, string) {
	component := getStr(det, "component")
	var hit *pipelineInfo
	pinned := false
	for i := range pipelines {
		if w, p := wired(pipelines[i], component); w {
			hit, pinned = &pipelines[i], p
			break
		}
	}
	if hit == nil {
		return "fail", fmt.Sprintf("no pipeline references component %q", component)
	}
	base := filepath.Base(hit.Path)
	if getBool(det, "pinned") && !pinned {
		return "fail", base + ": reference is not pinned to a release tag"
	}
	events, _ := strList(det["events"])
	for _, event := range events {
		switch hasEvent(*hit, event, cfg.CI) {
		case 0:
			return "fail", fmt.Sprintf("%s: not wired on %s", base, event)
		case -1:
			// Not tree-decidable on this CI platform: degrade to
			// attestation (conformance spec §3).
			att := event + "_enforcement"
			if !cfg.Attestations[att] {
				return "fail", fmt.Sprintf("%s enforcement is not "+
					"tree-decidable on %s; record attestation %q",
					event, cfg.CI, att)
			}
			return "attested", att
		}
	}
	if getBool(det, "required_check") {
		att := "required_check_enforced"
		if !cfg.Attestations[att] {
			return "fail", fmt.Sprintf(
				"record attestation %q (branch protection / policy)", att)
		}
		return "attested", att
	}
	return "pass", base
}

// pathsLockstep: if the gate pipeline path-filters, the filter must cover
// every consumer_path. No filter (the scaffolded default) is a pass.
func pathsLockstep(det map[string]any, cfg *SliceConfig,
	pipelines []pipelineInfo, manifest map[string]any) (string, string) {
	if manifest == nil {
		return "fail", "slice manifest missing"
	}
	component := getStr(det, "component")
	if component == "" {
		component = "gate"
	}
	var hit *pipelineInfo
	for i := range pipelines {
		if w, _ := wired(pipelines[i], component); w {
			hit = &pipelines[i]
			break
		}
	}
	if hit == nil {
		return "fail", "gate pipeline not found"
	}
	var filters []string
	if cfg.CI == "github-actions" {
		on, ok := hit.Data["on"].(map[string]any)
		if !ok {
			on, _ = hit.Data["true"].(map[string]any)
		}
		if pr, isMap := on["pull_request"].(map[string]any); isMap {
			filters, _ = strList(pr["paths"])
		}
	} else {
		if pr, isMap := hit.Data["pr"].(map[string]any); isMap {
			filters, _ = strList(getMap(pr, "paths")["include"])
		}
	}
	if len(filters) == 0 {
		return "pass", "gate runs unfiltered"
	}
	// Seed entries are exempt: the gate never hash-checks them, so filter
	// coverage is moot (DR-0013).
	var uncovered []string
	for _, e := range manifestEntries(manifest) {
		if getBool(e, "seed") {
			continue
		}
		cpath := getStr(e, "consumer_path")
		if !MatchAny(cpath, filters) {
			uncovered = append(uncovered, cpath)
		}
	}
	if len(uncovered) == 0 {
		return "pass", ""
	}
	return "fail", fmt.Sprintf("filter misses %d path(s), e.g. %s",
		len(uncovered), uncovered[0])
}
