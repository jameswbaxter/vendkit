// Publisher-side verification of glob-localise output (issue #10).
//
// Two instruments. CheckLocalisation: declaration-validity rules — pure
// consistency of the declaration plus the matched tree, folded into
// `generate --check` as advisory findings. VerifyLocalisation: the
// expectation oracle — diff the ACTUAL localised output (the real adapter
// chain, or an already-materialised consumer) against a publisher-AUTHORED
// expectation file. The expected values are input, never engine-derived at
// check time: an engine-computed oracle would agree with the engine's bugs
// by construction, which is how the 5e5d701 no-op survived the unit suite.

package core

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
)

// LocalisationFinding is one finding from either instrument. Rule is the
// stable token: profile-unknown | glob-uncatalogued | catalogue-glob-orphan |
// localisation-empty (validity), mismatch | rule-absent | expectation-stale
// (oracle). Path is the publisher repo-relative rule file where file-scoped.
type LocalisationFinding struct {
	Rule    string `json:"rule"`
	Path    string `json:"path,omitempty"`
	Profile string `json:"profile,omitempty"`
	Glob    string `json:"glob,omitempty"`
	Detail  string `json:"detail,omitempty"`
}

func (f LocalisationFinding) String() string {
	s := f.Rule + ":"
	if f.Profile != "" {
		s += " [" + f.Profile + "]"
	}
	if f.Path != "" {
		s += " " + f.Path
	}
	if f.Glob != "" {
		s += " " + f.Glob
	}
	if f.Detail != "" {
		s += " — " + f.Detail
	}
	return s
}

func sortedNames[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// -- part A: declaration validity (generate --check) ------------------------------

// CheckLocalisation evaluates the declaration-validity rules for every
// glob-localise adapter against the matched tree — exported and seeded files
// both flow through adapters (export-declaration spec §2). Advisory in
// `generate --check`. Everything runs through the adapter's own field parser
// (FieldGlobs) and, for localisation-empty, the actual transform, so the
// check cannot disagree with materialisation. Deterministic order per
// adapter: unknown catalogue keys, then per-file empty localisations, then
// uncatalogued globs, then orphaned catalogue globs.
func CheckLocalisation(decl *ExportDecl, root string) ([]LocalisationFinding, error) {
	hasLocalise := false
	for _, a := range decl.Adapters {
		if a.Kind == "glob-localise" {
			hasLocalise = true
		}
	}
	if !hasLocalise {
		return nil, nil
	}
	exported, err := decl.ExportedFiles(root)
	if err != nil {
		return nil, err
	}
	seeded, err := decl.SeededFiles(root)
	if err != nil {
		return nil, err
	}
	files := append(append([]string{}, exported...), seeded...)
	sort.Strings(files)

	var out []LocalisationFinding
	for _, a := range decl.Adapters {
		if a.Kind != "glob-localise" {
			continue
		}
		for _, pname := range sortedNames(a.Catalogue) {
			if _, ok := decl.Profiles[pname]; !ok {
				out = append(out, LocalisationFinding{
					Rule: "profile-unknown", Profile: pname,
					Detail: fmt.Sprintf("catalogue key of adapter %q is not a "+
						"declared profile", a.Match),
				})
			}
		}
		catalogued := map[string]bool{}
		for _, globs := range a.Catalogue {
			for _, g := range globs {
				catalogued[g] = true
			}
		}
		// The empty-check materialises for declared profiles ∪ catalogue keys
		// (a catalogue-only key is already flagged above but still binds).
		profiles := map[string]bool{}
		for p := range decl.Profiles {
			profiles[p] = true
		}
		for p := range a.Catalogue {
			profiles[p] = true
		}
		plist := sortedNames(profiles)

		seen := map[string]bool{}
		uncat := map[string]string{} // glob -> first file carrying it
		for _, rel := range files {
			if !PathMatch(rel, a.Match) {
				continue
			}
			data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
			if err != nil {
				return nil, Errf("read %s: %v", rel, err)
			}
			globs, found := FieldGlobs(data, a.FMField)
			if !found {
				continue // the adapter no-ops without the field; not a rule
			}
			for _, g := range globs {
				if !catalogued[g] {
					if _, dup := uncat[g]; !dup {
						uncat[g] = rel
					}
				}
				seen[g] = true
			}
			if len(globs) == 0 {
				continue // already-empty union; localisation changes nothing
			}
			for _, p := range plist {
				if !decl.ProfileInScope(p, rel) {
					continue
				}
				pruned := localise(data, a.FMField, a.Catalogue, p)
				if kept, _ := FieldGlobs(pruned, a.FMField); len(kept) == 0 {
					out = append(out, LocalisationFinding{
						Rule: "localisation-empty", Path: rel, Profile: p,
						Detail: fmt.Sprintf("field %q would arrive empty for "+
							"this profile — the rule matches nothing and can "+
							"never load", a.FMField),
					})
				}
			}
		}
		for _, g := range sortedNames(uncat) {
			out = append(out, LocalisationFinding{
				Rule: "glob-uncatalogued", Path: uncat[g], Glob: g,
				Detail: fmt.Sprintf("glob in field %q is claimed by no "+
					"catalogue entry, so it vendors to every profile "+
					"(universal) — catalogue it if it is profile-owned", a.FMField),
			})
		}
		var orphans []string
		for g := range catalogued {
			if !seen[g] {
				orphans = append(orphans, g)
			}
		}
		sort.Strings(orphans)
		for _, g := range orphans {
			out = append(out, LocalisationFinding{
				Rule: "catalogue-glob-orphan", Glob: g,
				Detail: fmt.Sprintf("catalogue glob of adapter %q appears in "+
					"no matched rule — dead entry", a.Match),
			})
		}
	}
	return out, nil
}

// -- part B: the expectation oracle (verify-localisation) --------------------------

// LocalisationExpectations is the publisher-authored oracle file:
//
//	schema_version: 1
//	expectations:
//	  <profile>:
//	    <publisher-relative path>: [glob, ...]
//
// Paths are publisher repo-relative (the declaration's path space); values
// are the intended localised field content, in order.
type LocalisationExpectations struct {
	ByProfile map[string]map[string][]string
}

func LoadLocalisationExpectations(path string) (*LocalisationExpectations, error) {
	data, err := LoadYAML(path)
	if err != nil {
		return nil, err
	}
	var errs []string
	if !schemaVersionIs(data, 1) {
		errs = append(errs, "schema_version must be 1")
	}
	for key := range data {
		if key != "schema_version" && key != "expectations" {
			errs = append(errs, fmt.Sprintf("unknown top-level key: %q", key))
		}
	}
	byProfile := map[string]map[string][]string{}
	for pname, raw := range getMap(data, "expectations") {
		rules, ok := raw.(map[string]any)
		if !ok {
			errs = append(errs, fmt.Sprintf(
				"expectations.%s must map paths to glob lists", pname))
			continue
		}
		prules := map[string][]string{}
		for rel, v := range rules {
			globs, ok := strList(v)
			if !ok {
				errs = append(errs, fmt.Sprintf(
					"expectations.%s.%s must be a list of glob strings", pname, rel))
				continue
			}
			if globs == nil {
				globs = []string{}
			}
			prules[rel] = globs
		}
		byProfile[pname] = prules
	}
	if len(errs) > 0 {
		sort.Strings(errs) // map-driven; sorted for a deterministic message
		return nil, Usagef("%s: %s", path, strings.Join(errs, "; "))
	}
	return &LocalisationExpectations{ByProfile: byProfile}, nil
}

// localiseRules maps each exported file to its glob-localise adapter. The
// oracle covers the drift-gated surface only: seeds are consumer-owned after
// materialisation and lawfully diverge, so they are not verifiable.
func localiseRules(decl *ExportDecl, publisherRoot string) (map[string]Adapter, error) {
	exported, err := decl.ExportedFiles(publisherRoot)
	if err != nil {
		return nil, err
	}
	rules := map[string]Adapter{}
	for _, rel := range exported {
		hits, err := decl.AdaptersFor(rel)
		if err != nil {
			return nil, err
		}
		for _, a := range hits {
			if a.Kind == "glob-localise" {
				rules[rel] = a // at most one per path (AdaptersFor enforces)
			}
		}
	}
	return rules, nil
}

// actualLocalisation: rel -> field globs after the REAL adapter chain, for
// files in the profile's scope whose field is present. This is the engine
// end of the differential — never a reimplementation of the pruning.
func actualLocalisation(decl *ExportDecl, publisherRoot string,
	rules map[string]Adapter, profile string) (map[string][]string, error) {
	actual := map[string][]string{}
	for _, rel := range sortedNames(rules) {
		if !decl.ProfileInScope(profile, rel) {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(publisherRoot, filepath.FromSlash(rel)))
		if err != nil {
			return nil, Errf("read %s: %v", rel, err)
		}
		doc, err := ApplyAdapters(decl, rel, raw, profile)
		if err != nil {
			return nil, err
		}
		if globs, found := FieldGlobs(doc, rules[rel].FMField); found {
			if globs == nil {
				globs = []string{}
			}
			actual[rel] = globs
		}
	}
	return actual, nil
}

// consumerLocalisation reads the already-materialised consumer copies
// instead of rendering. A missing vendored copy is skipped — that is the
// gate's finding, and it surfaces here as rule-absent when expected.
func consumerLocalisation(decl *ExportDecl, consumerRoot string,
	rules map[string]Adapter, profile string) (map[string][]string, error) {
	actual := map[string][]string{}
	for _, rel := range sortedNames(rules) {
		if !decl.ProfileInScope(profile, rel) {
			continue
		}
		cpath, err := decl.ConsumerPath(rel)
		if err != nil {
			return nil, err
		}
		doc, err := os.ReadFile(filepath.Join(consumerRoot, filepath.FromSlash(cpath)))
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, Errf("read %s: %v", cpath, err)
		}
		if globs, found := FieldGlobs(doc, rules[rel].FMField); found {
			if globs == nil {
				globs = []string{}
			}
			actual[rel] = globs
		}
	}
	return actual, nil
}

type LocalisationReport struct {
	Findings []LocalisationFinding
	Checked  int // (profile, rule) pairs compared
}

// VerifyLocalisation diffs actual localised output against the expectation
// file. With consumerRoot empty it materialises in memory from the publisher
// tree, for each profile in declared ∪ expected (or just profileFilter);
// with consumerRoot set it reads the materialised consumer, whose profile is
// profileFilter or the consumer's bound profile. Glob lists compare in
// order, as they appear in the field.
func VerifyLocalisation(decl *ExportDecl, publisherRoot, consumerRoot string,
	exp *LocalisationExpectations, profileFilter string) (*LocalisationReport, error) {
	rules, err := localiseRules(decl, publisherRoot)
	if err != nil {
		return nil, err
	}
	var profiles []string
	switch {
	case consumerRoot != "":
		p := profileFilter
		if p == "" {
			cfg, err := FindSliceConfig(consumerRoot, decl.SliceName)
			if err != nil {
				return nil, err
			}
			if cfg != nil {
				p = cfg.Profile
			}
		}
		if p == "" {
			return nil, Usagef("consumer is not bound to a profile — an " +
				"unbound consumer keeps the union verbatim, so there is " +
				"nothing to verify (--profile overrides)")
		}
		profiles = []string{p}
	case profileFilter != "":
		profiles = []string{profileFilter}
	default:
		set := map[string]bool{}
		for p := range decl.Profiles {
			set[p] = true
		}
		for p := range exp.ByProfile {
			set[p] = true
		}
		if len(set) == 0 {
			return nil, Usagef("no profiles declared and no expectations " +
				"to verify against")
		}
		profiles = sortedNames(set)
	}

	report := &LocalisationReport{}
	for _, p := range profiles {
		var actual map[string][]string
		if consumerRoot != "" {
			actual, err = consumerLocalisation(decl, consumerRoot, rules, p)
		} else {
			actual, err = actualLocalisation(decl, publisherRoot, rules, p)
		}
		if err != nil {
			return nil, err
		}
		expected := exp.ByProfile[p]
		paths := map[string]bool{}
		for rel := range actual {
			paths[rel] = true
		}
		for rel := range expected {
			paths[rel] = true
		}
		for _, rel := range sortedNames(paths) {
			act, aOK := actual[rel]
			want, eOK := expected[rel]
			switch {
			case aOK && eOK:
				report.Checked++
				if !slices.Equal(act, want) {
					report.Findings = append(report.Findings, LocalisationFinding{
						Rule: "mismatch", Path: rel, Profile: p,
						Detail: fmt.Sprintf("expected [%s], got [%s]",
							strings.Join(want, ", "), strings.Join(act, ", ")),
					})
				}
			case eOK:
				report.Findings = append(report.Findings, LocalisationFinding{
					Rule: "rule-absent", Path: rel, Profile: p,
					Detail: "expected rule has no localised field here (not " +
						"in the exported surface or profile scope, or the " +
						"field is missing)",
				})
			default:
				report.Findings = append(report.Findings, LocalisationFinding{
					Rule: "expectation-stale", Path: rel, Profile: p,
					Detail: "localised rule has no expectation entry — " +
						"refresh the expected file with --write after review",
				})
			}
		}
	}
	return report, nil
}

// WriteExpectedLocalisation refreshes the expectation file from the CURRENT
// engine output for every declared profile — the sanctioned way to update
// the oracle after a reviewed change. It is a deliberate step, never run at
// check time: deriving the expected file when checking would compare the
// engine against itself. Returns the number of (profile, rule) entries.
func WriteExpectedLocalisation(decl *ExportDecl, publisherRoot, path string) (int, error) {
	if len(decl.Profiles) == 0 {
		return 0, Usagef("no profiles declared — there is no localisation to expect")
	}
	rules, err := localiseRules(decl, publisherRoot)
	if err != nil {
		return 0, err
	}
	var b strings.Builder
	b.WriteString("# Localisation expectations, verified by `vendkit verify-localisation`.\n")
	b.WriteString("# Publisher-authored oracle: review every change to this file — it is\n")
	b.WriteString("# what the localised output is checked AGAINST. Refresh deliberately\n")
	b.WriteString("# with --write after a reviewed declaration or rule change.\n")
	b.WriteString("schema_version: 1\n")
	b.WriteString("expectations:\n")
	entries := 0
	for _, p := range sortedNames(decl.Profiles) {
		actual, err := actualLocalisation(decl, publisherRoot, rules, p)
		if err != nil {
			return 0, err
		}
		if len(actual) == 0 {
			fmt.Fprintf(&b, "  %q: {}\n", p)
			continue
		}
		fmt.Fprintf(&b, "  %q:\n", p)
		for _, rel := range sortedNames(actual) {
			globs := actual[rel]
			entries++
			if len(globs) == 0 {
				fmt.Fprintf(&b, "    %q: []\n", rel)
				continue
			}
			fmt.Fprintf(&b, "    %q:\n", rel)
			for _, g := range globs {
				fmt.Fprintf(&b, "      - %q\n", g)
			}
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return 0, Errf("mkdir for %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		return 0, Errf("write %s: %v", path, err)
	}
	return entries, nil
}
