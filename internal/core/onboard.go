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

// HandlerModules: consumer-scm → reference handler module — just a default
// string in a config file (handler-protocol spec §6).
var HandlerModules = map[string]string{
	"github":      "vendkit.handlers.github",
	"azure-repos": "vendkit.handlers.ado",
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

type OnboardParams struct {
	CI            string
	SCM           string
	Version       string
	Profile       string
	Mode          string
	BaseBranch    string
	PRTokenSecret string
	Codeowners    string
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
	handlerModule := HandlerModules[p.SCM]
	var handlersBlock string
	if p.CI == "none" {
		handlersBlock = "# handlers: deliberately unwired (fully manual). Wire any executable that\n" +
			"# honours the handler protocol, e.g.:\n" +
			"#   pr: {exec: [python3, -m, " + handlerModule + "]}\n"
	} else {
		handlersBlock = "handlers:\n" +
			"  # Delivery handlers (handler protocol, DR-0014): any executable honouring\n" +
			"  # the protocol can replace these reference commands.\n" +
			"  pr:\n" +
			"    exec: [python3, -m, " + handlerModule + "]\n" +
			"  handoff:\n" +
			"    exec: [python3, -m, " + handlerModule + "]\n" +
			"    dedup_key: vendkit-watch-" + decl.SliceName + "\n" +
			"  fact-verify:\n" +
			"    exec: [python3, -m, " + handlerModule + "]\n"
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
