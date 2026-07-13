// Scaffolder (onboarding spec §2): vendor + scaffold + report. Templates
// come from the embedded assets (DR-0016: the binary is self-contained).

package core

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type scaffoldOutput struct {
	Template    string
	OutRel      string
	PrimaryOnly bool
}

var scaffoldOutputs = map[string][]scaffoldOutput{
	"github-actions": {
		{"gate.yml.tmpl", ".github/workflows/vendkit-gate.yml", true},
		{"watch.yml.tmpl", ".github/workflows/vendkit-watch.yml", true},
		{"conformance.yml.tmpl", ".github/workflows/vendkit-conformance.yml", true},
		{"sync.yml.tmpl", ".github/workflows/__SLICE__-sync.yml", false},
	},
	"azure-pipelines": {
		{"gate.yml.tmpl", "azure-pipelines/vendkit-gate.yml", true},
		{"watch.yml.tmpl", "azure-pipelines/vendkit-watch.yml", true},
		{"conformance.yml.tmpl", "azure-pipelines/vendkit-conformance.yml", true},
		{"sync.yml.tmpl", "azure-pipelines/__SLICE__-sync.yml", false},
	},
	"none": {},
}

// HandlerModules: consumer-scm → the `vendkit handler <arg>` reference
// handler built into the binary (handler-protocol spec §6, DR-0016). Any
// protocol-honouring executable can replace it in the slice config.
var HandlerModules = map[string]string{
	"github":      "github",
	"azure-repos": "ado",
}

var placeholderRx = regexp.MustCompile(`__[A-Z][A-Z0-9_]*__`)

type OnboardResult struct {
	Written     []string
	Vendored    int
	ManualSteps string
}

func renderTemplate(template string, subs map[string]string) (string, error) {
	out := template
	for key, value := range subs {
		out = strings.ReplaceAll(out, "__"+key+"__", value)
	}
	if left := placeholderRx.FindString(out); left != "" {
		// Fail loudly on any unresolved placeholder (onboarding spec §2).
		return "", Usagef("unresolved scaffold placeholder: %s", left)
	}
	return out, nil
}

// pushHintReceiver is the GHA repository_dispatch receiver block, or empty when
// --push-hint is off (platform-integration spec §4). The block sits under the
// workflow's `on:` key, so it carries two-space indentation.
func pushHintReceiver(p OnboardParams) string {
	if !p.PushHint {
		return ""
	}
	return "  # Optional push hint (DR-0006): the publisher's release workflow may\n" +
		"  # dispatch this event; the schedule above remains the reconciler.\n" +
		"  repository_dispatch:\n" +
		"    types: [vendkit-release]"
}

// pushHintTrigger is the ADO resources.pipelines trigger block, or empty when
// --push-hint is off. It nests under `resources:`, so two-space indentation.
// `source:` names the consumer's own publisher-release pipeline — an inherently
// consumer-local value they must set (angle-free so it is not a placeholder).
func pushHintTrigger(p OnboardParams) string {
	if !p.PushHint {
		return ""
	}
	return "  pipelines:\n" +
		"    - pipeline: publisher-release\n" +
		"      source: publisher-release-pipeline # set to your publisher's release pipeline name\n" +
		"      trigger: true"
}

type OnboardParams struct {
	CI            string
	SCM           string
	Version       string
	Profile       string
	Mode          string
	BaseBranch    string
	PRTokenSecret string
	Codeowners    string
	// PushHint gates the optional early-trigger receiver in the sync pipeline
	// (platform-integration spec §4): the GHA repository_dispatch receiver, or
	// the ADO resources.pipelines trigger. Off by default — the schedule is
	// always the reconciler; the receiver is a latency optimisation (DR-0006).
	PushHint bool
}

func Onboard(publisherRoot, consumerRoot string, decl *ExportDecl,
	p OnboardParams, scaffoldFS fs.FS) (*OnboardResult, error) {
	outputs, ok := scaffoldOutputs[p.CI]
	if !ok {
		return nil, Usagef("unknown ci %q", p.CI)
	}
	if _, ok := HandlerModules[p.SCM]; !ok {
		return nil, Usagef("unknown scm %q", p.SCM)
	}
	if p.Mode != "primary" && p.Mode != "additive" {
		return nil, Usagef("mode must be 'primary' or 'additive'")
	}
	if p.Profile != "" && len(decl.Profiles) > 0 {
		if _, ok := decl.Profiles[p.Profile]; !ok {
			return nil, Usagef("profile %q not declared by the publisher", p.Profile)
		}
	}
	if p.Codeowners != "" && p.SCM != "github" {
		return nil, Usagef("--codeowners is GitHub-only: Azure Repos does " +
			"not honour CODEOWNERS — add a required-reviewers branch policy " +
			"covering .vendkit/** instead (it is on the manual-steps checklist)")
	}
	result := &OnboardResult{}

	subs := map[string]string{
		"SLICE":           decl.SliceName,
		"SLICE_TITLE":     decl.SliceTitle,
		"PUBLISHER_REPO":  decl.PublisherRepo,
		"PUBLISHER_SCM":   decl.PublisherSCM,
		"VERSION":         p.Version,
		"BASE_BRANCH":     p.BaseBranch,
		"PR_TOKEN_SECRET": p.PRTokenSecret,
		// Push-hint receiver — flag-gated (platform-integration spec §4). When
		// off these expand to empty and the sync pipeline is schedule-only.
		"PUSH_HINT_RECEIVER": pushHintReceiver(p),
		"PUSH_HINT_TRIGGER":  pushHintTrigger(p),
	}

	// 1. Slice config first (materialise reads the profile from it).
	cfgPath := filepath.Join(consumerRoot, VendkitDir, decl.SliceName+".yml")
	if _, err := os.Stat(cfgPath); err == nil {
		return nil, Usagef("slice already onboarded: %s", cfgPath)
	}
	var pinFiles []string
	if p.CI != "none" {
		pinFile := strings.ReplaceAll(outputs[3].OutRel, "__SLICE__", decl.SliceName)
		// Every scaffolded workflow this slice pins gets advanced in
		// lockstep by the sync PR (sync spec §3 step 4). Additive slices
		// own only their sync pipeline.
		pinFiles = []string{pinFile}
		if p.Mode == "primary" {
			for _, o := range outputs[:3] {
				pinFiles = append(pinFiles, o.OutRel)
			}
		}
	}
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0o755); err != nil {
		return nil, Errf("mkdir: %v", err)
	}
	if err := os.WriteFile(cfgPath,
		[]byte(sliceConfigYAML(decl, p, pinFiles)), 0o644); err != nil {
		return nil, Errf("write %s: %v", cfgPath, err)
	}
	result.Written = append(result.Written, cfgPath)

	// 2. Vendor: empty manifest + reconcile-scope expansion (spec §2).
	if err := SeedEmptyManifest(consumerRoot, decl); err != nil {
		return nil, err
	}
	report, err := Materialise(publisherRoot, consumerRoot, decl,
		p.Version, true, true)
	if err != nil {
		return nil, err
	}
	result.Vendored = len(report.Added)
	result.Written = append(result.Written,
		filepath.Join(consumerRoot, VendkitDir, decl.ManifestName))

	// 3. Scaffold pipelines (none under ci: none).
	for _, o := range outputs {
		if o.PrimaryOnly && p.Mode == "additive" {
			continue
		}
		tmpl, err := fs.ReadFile(scaffoldFS, "scaffold/"+p.CI+"/"+o.Template)
		if err != nil {
			return nil, Errf("scaffold template %s: %v", o.Template, err)
		}
		outPath := filepath.Join(consumerRoot,
			filepath.FromSlash(strings.ReplaceAll(o.OutRel, "__SLICE__", decl.SliceName)))
		if _, err := os.Stat(outPath); err == nil && o.PrimaryOnly {
			continue // additive repair: shared pipelines already exist
		}
		rendered, err := renderTemplate(string(tmpl), subs)
		if err != nil {
			return nil, err
		}
		if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
			return nil, Errf("mkdir: %v", err)
		}
		if err := os.WriteFile(outPath, []byte(rendered), 0o644); err != nil {
			return nil, Errf("write %s: %v", outPath, err)
		}
		result.Written = append(result.Written, outPath)
	}

	// 4. Ownership — opt-in, SCM-axis (DR-0015).
	if p.Codeowners != "" {
		coPath := filepath.Join(consumerRoot, "CODEOWNERS")
		stanza := "/" + VendkitDir + "/ " + p.Codeowners + "\n"
		existing := ""
		if data, err := os.ReadFile(coPath); err == nil {
			existing = string(data)
		}
		if !strings.Contains(existing, stanza) {
			if err := os.WriteFile(coPath, []byte(existing+stanza), 0o644); err != nil {
				return nil, Errf("write CODEOWNERS: %v", err)
			}
			result.Written = append(result.Written, coPath)
		}
	}

	result.ManualSteps = manualSteps(decl, p)
	return result, nil
}

// manualSteps: the irreducible checklist (onboarding spec §4). Each step
// maps to a conformance rule, so 'fully onboarded' == strict passes.
func manualSteps(decl *ExportDecl, p OnboardParams) string {
	var steps []string
	steps = append(steps, fmt.Sprintf(
		"Grant the CI identity read access on %s (checkout + engine resolution).",
		decl.PublisherRepo))
	if p.CI != "none" {
		steps = append(steps, fmt.Sprintf(
			"Record the engine checksums in .vendkit/%s.yml under engine.sha256 "+
				"from the %s release's SHA256SUMS.txt (one hex per platform "+
				"asset). Until filled, `vendkit self-verify` is advisory; the "+
				"fetch step still verifies each download against SHA256SUMS.txt "+
				"(DR-0016).", decl.SliceName, p.Version))
		steps = append(steps, fmt.Sprintf(
			"Provision the PR-capable sync credential as secret %s for the "+
				"PR handler (GitHub: PAT/App token, NOT GITHUB_TOKEN; "+
				"Azure DevOps: PR-create-capable identity).", p.PRTokenSecret))
		steps = append(steps,
			"Protect the default branch and make the gate a required check "+
				"(GitHub: ruleset/branch protection; Azure Repos: Build "+
				"Validation policy).")
	} else {
		steps = append(steps,
			"Protect the default branch. NOTE: with ci 'none' there is no "+
				"PR-time gate — you forfeit automated drift enforcement and "+
				"must run `vendkit gate --strict` yourself.")
	}
	if p.SCM == "azure-repos" {
		steps = append(steps,
			"Add a required-reviewers branch policy covering .vendkit/** "+
				"and the vendored namespaces (Azure Repos does not honour "+
				"CODEOWNERS); record attestation required_reviewers_policy.")
	}
	steps = append(steps, fmt.Sprintf(
		"Record attestations in .vendkit/%s.yml: branch_protection_enabled, "+
			"sync_credential_provisioned, pull_request_enforcement "+
			"(azure-pipelines), required_check_enforced.", decl.SliceName))
	lines := []string{
		"Manual steps (reported, never performed — onboarding spec §4; each " +
			"maps to a conformance rule):",
	}
	for i, s := range steps {
		lines = append(lines, fmt.Sprintf("  %d. %s", i+1, s))
	}
	lines = append(lines,
		"Then run `vendkit conformance --strict` — fully onboarded == it passes.")
	return strings.Join(lines, "\n") + "\n"
}

func sliceConfigYAML(decl *ExportDecl, p OnboardParams, pinFiles []string) string {
	profileLine := "# profile: <bind to a publisher profile>"
	if p.Profile != "" {
		profileLine = "profile: " + p.Profile
	}
	var pinBlock string
	if len(pinFiles) > 0 {
		var files []string
		for _, f := range pinFiles {
			files = append(files, "    - "+f)
		}
		pinBlock = "pin:\n" +
			"  # First entry is the authoritative read source; the sync PR advances the\n" +
			"  # matching reference line in every listed file, in lockstep.\n" +
			"  pattern: \"ref: refs/tags/v\"\n" +
			"  files:\n" + strings.Join(files, "\n") + "\n"
	} else {
		pinBlock = "# ci is 'none': no pin lines — the manifest's source.release is the pin.\n"
	}
	handlerArg := HandlerModules[p.SCM]
	var handlersBlock string
	if p.CI == "none" {
		handlersBlock = "# handlers: deliberately unwired (fully manual). Wire any executable that\n" +
			"# honours the handler protocol, e.g.:\n" +
			"#   pr: {exec: [vendkit, handler, " + handlerArg + "]}\n"
	} else {
		handlersBlock = "handlers:\n" +
			"  # Delivery handlers (handler protocol, DR-0014): any executable honouring\n" +
			"  # the protocol can replace these reference commands. `vendkit handler <scm>`\n" +
			"  # is the built-in reference implementation (same binary, DR-0016).\n" +
			"  pr:\n" +
			"    exec: [vendkit, handler, " + handlerArg + "]\n" +
			"  handoff:\n" +
			"    exec: [vendkit, handler, " + handlerArg + "]\n" +
			"    dedup_key: vendkit-watch-" + decl.SliceName + "\n" +
			"  fact-verify:\n" +
			"    exec: [vendkit, handler, " + handlerArg + "]\n"
	}
	var engineBlock string
	if p.CI == "none" {
		engineBlock = "# ci is 'none': the human tier runs its installed engine against fetched\n" +
			"# trees (DR-0016 §4) — there is no engine pin to fetch and verify.\n"
	} else {
		engineBlock = "engine:\n" +
			"  # DR-0016: the engine is a pinned, checksummed release binary — the\n" +
			"  # scaffolded pipeline fetches it, verifies it against SHA256SUMS.txt,\n" +
			"  # then `vendkit self-verify` re-asserts it against the sha256 below.\n" +
			"  # Advanced with the content pin by the sync PR; refill sha256 from the\n" +
			"  # target release's SHA256SUMS.txt when the version moves (a blank value\n" +
			"  # is advisory-only — self-verify skips it).\n" +
			"  version: " + p.Version + "\n" +
			"  sha256:\n" +
			"    linux/amd64: \"\"\n" +
			"    linux/arm64: \"\"\n" +
			"    darwin/amd64: \"\"\n" +
			"    darwin/arm64: \"\"\n" +
			"    windows/amd64: \"\"\n" +
			"    windows/arm64: \"\"\n"
	}
	return "schema_version: 1\n" +
		"slice: " + decl.SliceName + "\n" +
		"publisher:\n" +
		"  scm: " + decl.PublisherSCM + "\n" +
		"  repo: " + decl.PublisherRepo + "\n" +
		"scm: " + p.SCM + "\n" +
		"ci: " + p.CI + "\n" +
		profileLine + "\n" +
		pinBlock +
		engineBlock +
		"watch:\n" +
		"  channel: stable\n" +
		handlersBlock +
		"seeds:\n" +
		"  # Seeded files are scaffolded once, then yours (DR-0013). 'informational'\n" +
		"  # adds a note to sync PRs when an upstream template later changes.\n" +
		"  notes: informational\n" +
		"attestations:\n" +
		"  branch_protection_enabled: false\n" +
		"  sync_credential_provisioned: false\n" +
		"waivers: []\n"
}
