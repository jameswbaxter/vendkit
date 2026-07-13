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
	"runtime"
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
	// Fact is the stable machine key identifying the attested control (empty
	// for non-attested results). Verify carries the structured parameters a
	// fact-verify handler needs to check that control (e.g. the pipeline
	// event). Both are internal plumbing for the --verify-attestations intent
	// — never string-matched from the human Detail — and stay OUT of the fleet
	// interchange document (json:"-"); Detail remains the display string.
	Fact   string            `json:"-"`
	Verify map[string]string `json:"-"`
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

// PinDoc is the engine pin recorded in the slice config (DR-0016). SHA256 is
// the checksum recorded for the audit host's own platform (an empty value is
// an unrecorded/advisory pin) — the fleet interchange carries it per-consumer.
type PinDoc struct {
	Version string `json:"version"`
	SHA256  string `json:"sha256,omitempty"`
}

// ConformanceDoc is the `conformance --json` interchange document (conformance
// spec §5): the fleet-view shape a fleet audit aggregates. PinLag is a *int so
// it serialises as JSON null when it cannot be determined offline (no network):
// core never calls a vendor/SCM API to compute it (spec §5).
type ConformanceDoc struct {
	Slice    string        `json:"slice"`
	Profile  string        `json:"profile"`
	Pin      *PinDoc       `json:"pin,omitempty"`
	PinLag   *int          `json:"pin_lag"`
	GapCount int           `json:"gap_count"`
	Rules    []*RuleResult `json:"rules"`
}

// Document renders the evaluated report into the fleet-view interchange
// document for a given slice config. pin_lag is left nil (JSON null): it is not
// determinable offline and core issues no network call to find it (spec §5).
func (r *ConformanceReport) Document(cfg *SliceConfig) *ConformanceDoc {
	rules := r.Results
	if rules == nil {
		rules = []*RuleResult{}
	}
	doc := &ConformanceDoc{
		Slice:    cfg.SliceName,
		Profile:  cfg.Profile,
		GapCount: len(r.Gaps()),
		Rules:    rules,
	}
	if cfg.EngineVersion != "" {
		doc.Pin = &PinDoc{
			Version: cfg.EngineVersion,
			SHA256:  cfg.EngineSHA256[runtime.GOOS+"/"+runtime.GOARCH],
		}
	}
	return doc
}

// WorstStatus is the most severe rule status in the document, by the
// conformance status ranking (see StatusRank). Empty documents rank as "pass".
func (d *ConformanceDoc) WorstStatus() string {
	worst := "pass"
	for _, r := range d.Rules {
		if StatusRank(r.Status) > StatusRank(worst) {
			worst = r.Status
		}
	}
	return worst
}

// StatusRank orders conformance statuses from clean (0) to worst. Gaps
// (error, fail) rank highest so fleet audits surface worst offenders first;
// unverified assertions (attested) and forfeited enforcement (skipped) rank
// above deliberately accepted (waived) and clean (pass).
func StatusRank(status string) int {
	switch status {
	case "error":
		return 5
	case "fail":
		return 4
	case "attested":
		return 3
	case "skipped":
		return 2
	case "waived":
		return 1
	default: // pass and any unknown status
		return 0
	}
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
				report.Results = append(report.Results, &RuleResult{
					RuleID: rid, Title: title, Severity: severity,
					Status: "waived", Detail: reason})
				continue
			}
			report.Results = append(report.Results, &RuleResult{
				RuleID: rid, Title: title, Severity: severity, Status: "fail",
				Detail: "rule is mandatory and cannot be waived"})
			continue
		}
		status, detail, fact, verify := detect(det, consumerRoot, cfg, manifest, pipelines)
		report.Results = append(report.Results, &RuleResult{
			RuleID: rid, Title: title, Severity: severity,
			Status: status, Detail: detail, Fact: fact, Verify: verify})
	}
	return report
}

// detect returns (status, detail, fact, verify). fact is the stable
// verification key for an `attested` result (empty otherwise); verify holds
// the structured parameters a fact-verify handler needs. Non-attested results
// carry no fact/verify.
func detect(det map[string]any, consumerRoot string, cfg *SliceConfig,
	manifest map[string]any, pipelines []pipelineInfo) (string, string, string, map[string]string) {
	kind := getStr(det, "kind")

	switch kind {
	case "file-exists":
		path := strings.ReplaceAll(getStr(det, "path"), "<slice>", cfg.SliceName)
		if fi, err := os.Stat(filepath.Join(consumerRoot, filepath.FromSlash(path))); err == nil && !fi.IsDir() {
			return "pass", "", "", nil
		}
		return "fail", path + " missing", "", nil

	case "manifest-tracked":
		if manifest == nil {
			return "fail", "slice manifest missing", "", nil
		}
		want := getStr(det, "path")
		for _, e := range manifestEntries(manifest) {
			if getStr(e, "consumer_path") == want {
				return "pass", "", "", nil
			}
		}
		return "fail", want + " not tracked", "", nil

	case "profile-bound":
		if cfg.Profile != "" {
			return "pass", cfg.Profile, "", nil
		}
		return "fail", "no profile declared in slice config", "", nil

	case "codeowners-covers":
		// Ownership is an SCM-axis concern: Azure Repos does not honour
		// CODEOWNERS — the equivalent intent is a required-reviewers branch
		// policy, not tree-decidable → attest (DR-0015).
		if cfg.SCM == "azure-repos" {
			att := "required_reviewers_policy"
			if cfg.Attestations[att] {
				return "attested", att, att, nil
			}
			return "fail", fmt.Sprintf("CODEOWNERS is not honoured on "+
				"azure-repos; add a required-reviewers policy and record "+
				"attestation %q", att), "", nil
		}
		patterns, _ := strList(det["patterns"])
		status, detail := codeownersCovers(consumerRoot, patterns)
		return status, detail, "", nil

	case "attest":
		name := getStr(det, "attestation")
		if cfg.Attestations[name] {
			return "attested", name, name, nil
		}
		return "fail", fmt.Sprintf("attestation %q not recorded", name), "", nil

	case "tool":
		tool := filepath.Join(consumerRoot, filepath.FromSlash(getStr(det, "path")))
		if fi, err := os.Stat(tool); err != nil || fi.IsDir() {
			return "skipped", "tool absent: " + getStr(det, "path"), "", nil
		}
		args, _ := strList(det["args"])
		cmd := exec.Command(tool, args...)
		cmd.Dir = consumerRoot
		if err := cmd.Run(); err != nil {
			code := 1
			if ee, ok := err.(*exec.ExitError); ok {
				code = ee.ExitCode()
			}
			return "fail", fmt.Sprintf("tool exited %d", code), "", nil
		}
		return "pass", "", "", nil

	case "pipeline-wired":
		if cfg.CI == "none" {
			// Manual mode forfeits automated enforcement — say so, don't
			// hide it: `skipped` is visible in every report.
			return "skipped", "ci is 'none': orchestration is manual", "", nil
		}
		return pipelineWired(det, cfg, pipelines)

	case "paths-lockstep":
		if cfg.CI == "none" {
			return "skipped", "ci is 'none': no gate pipeline to filter", "", nil
		}
		status, detail := pathsLockstep(det, cfg, pipelines, manifest)
		return status, detail, "", nil
	}
	return "error", fmt.Sprintf("unknown detector kind %q", kind), "", nil
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

func pipelineWired(det map[string]any, cfg *SliceConfig, pipelines []pipelineInfo) (string, string, string, map[string]string) {
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
		return "fail", fmt.Sprintf("no pipeline references component %q", component), "", nil
	}
	base := filepath.Base(hit.Path)
	if getBool(det, "pinned") && !pinned {
		return "fail", base + ": reference is not pinned to a release tag", "", nil
	}
	events, _ := strList(det["events"])
	for _, event := range events {
		switch hasEvent(*hit, event, cfg.CI) {
		case 0:
			return "fail", fmt.Sprintf("%s: not wired on %s", base, event), "", nil
		case -1:
			// Not tree-decidable on this CI platform: degrade to
			// attestation (conformance spec §3). The fact key is
			// "<event>_enforcement"; verify carries the event so a
			// fact-verify handler can pick the right policy check.
			att := event + "_enforcement"
			if !cfg.Attestations[att] {
				return "fail", fmt.Sprintf("%s enforcement is not "+
					"tree-decidable on %s; record attestation %q",
					event, cfg.CI, att), "", nil
			}
			return "attested", att, att, map[string]string{"event": event}
		}
	}
	if getBool(det, "required_check") {
		att := "required_check_enforced"
		if !cfg.Attestations[att] {
			return "fail", fmt.Sprintf(
				"record attestation %q (branch protection / policy)", att), "", nil
		}
		return "attested", att, att, nil
	}
	return "pass", base, "", nil
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
