// Export declaration: the single source of slice identity (DR-0002).
// Schema: export-declaration spec v1.

package core

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

var slugRx = regexp.MustCompile(`^[a-z][a-z0-9-]{0,15}$`)

const DefaultDecl = "vendkit-export.yml"

var AdapterKinds = []string{"prefix-namespace", "glob-localise"}

type Adapter struct {
	Kind      string
	Match     string
	Prefix    string
	FMField   string
	Catalogue map[string][]string
}

type Profile struct {
	Name          string
	ExportInclude []string
	ExportExclude []string
}

type ExportDecl struct {
	SliceName     string
	SliceTitle    string
	PublisherSCM  string
	PublisherRepo string
	Include       []string
	Exclude       []string
	Seed          []string
	Adapters      []Adapter
	Profiles      map[string]Profile
	Retracted     []string
	ManifestName  string
	Path          string
}

func LoadExportDecl(path string) (*ExportDecl, error) {
	data, err := LoadYAML(path)
	if err != nil {
		return nil, err
	}
	var errs []string

	if !schemaVersionIs(data, 1) {
		errs = append(errs, "schema_version must be 1")
	}
	sl := getMap(data, "slice")
	name := getStr(sl, "name")
	if !slugRx.MatchString(name) {
		errs = append(errs, fmt.Sprintf("slice.name %q must match %s", name, slugRx.String()))
	}
	title := getStr(sl, "title")
	if title == "" {
		title = name
	}

	pub := getMap(data, "publisher")
	scm := getStr(pub, "scm")
	if scm != "github" && scm != "azure-repos" {
		errs = append(errs, "publisher.scm must be 'github' or 'azure-repos'")
	}
	repo := getStr(pub, "repo")
	if repo == "" {
		errs = append(errs, "publisher.repo is required (git URL or shorthand)")
	}

	include, ok1 := strList(data["include"])
	seed, ok2 := strList(data["seed"])
	if !ok1 || !ok2 {
		errs = append(errs, "include/seed must be lists of glob strings")
	}
	if len(include) == 0 && len(seed) == 0 {
		errs = append(errs, "at least one of include/seed must be non-empty")
	}
	exclude, _ := strList(data["exclude"])

	var adapters []Adapter
	for i, raw := range getList(data, "adapters") {
		m, _ := raw.(map[string]any)
		kind := getStr(m, "kind")
		known := false
		for _, k := range AdapterKinds {
			if kind == k {
				known = true
			}
		}
		if !known {
			// Hard error, never a silent skip (DR-0009).
			errs = append(errs, fmt.Sprintf("adapters[%d]: unknown kind %q", i, kind))
			continue
		}
		adp := Adapter{Kind: kind, Match: getStr(m, "match")}
		if adp.Match == "" {
			errs = append(errs, fmt.Sprintf("adapters[%d]: match is required", i))
		}
		if kind == "prefix-namespace" {
			adp.Prefix = getStr(m, "prefix")
			if adp.Prefix == "" {
				errs = append(errs, fmt.Sprintf("adapters[%d]: prefix is required", i))
			}
		} else {
			adp.FMField = getStr(m, "field")
			adp.Catalogue = map[string][]string{}
			for pname, globs := range getMap(m, "catalogue") {
				gl, _ := strList(globs)
				adp.Catalogue[pname] = gl
			}
			if adp.FMField == "" {
				errs = append(errs, fmt.Sprintf("adapters[%d]: field is required", i))
			}
		}
		adapters = append(adapters, adp)
	}

	profiles := map[string]Profile{}
	for pname, praw := range getMap(data, "profiles") {
		pm, _ := praw.(map[string]any)
		es := getMap(pm, "export_slice")
		inc, _ := strList(es["include"])
		if len(inc) == 0 {
			inc = []string{"*"}
		}
		exc, _ := strList(es["exclude"])
		profiles[pname] = Profile{Name: pname, ExportInclude: inc, ExportExclude: exc}
	}

	retracted, _ := strList(data["retracted"])
	manifestName := getStr(data, "manifest_name")
	if manifestName == "" {
		manifestName = name + "-manifest.json"
	}

	known := map[string]bool{"schema_version": true, "slice": true,
		"publisher": true, "include": true, "exclude": true, "seed": true,
		"adapters": true, "profiles": true, "retracted": true,
		"manifest_name": true}
	var unknown []string
	for key := range data {
		if !known[key] {
			unknown = append(unknown, key)
		}
	}
	sort.Strings(unknown)
	for _, key := range unknown {
		errs = append(errs, fmt.Sprintf("unknown top-level key: %q", key))
	}

	if len(errs) > 0 {
		return nil, Usagef("%s: %s", path, strings.Join(errs, "; "))
	}
	return &ExportDecl{
		SliceName: name, SliceTitle: title,
		PublisherSCM: scm, PublisherRepo: repo,
		Include: include, Exclude: exclude, Seed: seed,
		Adapters: adapters, Profiles: profiles, Retracted: retracted,
		ManifestName: manifestName, Path: path,
	}, nil
}

// -- export surface -------------------------------------------------------------

// matched: sorted repo-relative posix paths — matched(patterns) − exclude.
// Regular files only; symlinks are rejected (export-declaration spec §2).
func (d *ExportDecl) matched(root string, patterns []string) ([]string, error) {
	found := map[string]bool{}
	for _, pattern := range patterns {
		hits, err := TreeGlob(root, pattern)
		if err != nil {
			return nil, err
		}
		for _, h := range hits {
			if h.IsSymlink {
				return nil, Usagef("symlink in export surface: %s", h.Rel)
			}
			found[h.Rel] = true
		}
	}
	var out []string
	for rel := range found {
		if !MatchAny(rel, d.Exclude) {
			out = append(out, rel)
		}
	}
	sort.Strings(out)
	return out, nil
}

// ExportedFiles: the vendored (drift-gated) surface.
func (d *ExportDecl) ExportedFiles(root string) ([]string, error) {
	result, err := d.matched(root, d.Include)
	if err != nil {
		return nil, err
	}
	if len(result) == 0 && len(d.Seed) == 0 {
		return nil, Usagef("%s: export surface is empty", d.Path)
	}
	seeded, err := d.matched(root, d.Seed)
	if err != nil {
		return nil, err
	}
	inResult := map[string]bool{}
	for _, r := range result {
		inResult[r] = true
	}
	var overlap []string
	for _, s := range seeded {
		if inResult[s] {
			overlap = append(overlap, s)
		}
	}
	if len(overlap) > 0 {
		// A path cannot be both drift-gated and free-to-diverge (DR-0013).
		sort.Strings(overlap)
		if len(overlap) > 3 {
			overlap = overlap[:3]
		}
		return nil, Usagef("%s: path(s) matched by both include and seed: %s",
			d.Path, strings.Join(overlap, ", "))
	}
	return result, nil
}

// SeededFiles: the scaffold-once surface (DR-0013).
func (d *ExportDecl) SeededFiles(root string) ([]string, error) {
	return d.matched(root, d.Seed)
}

// -- adapters ---------------------------------------------------------------------

func (d *ExportDecl) AdaptersFor(path string) ([]Adapter, error) {
	var hits []Adapter
	for _, a := range d.Adapters {
		if PathMatch(path, a.Match) {
			hits = append(hits, a)
		}
	}
	for _, kind := range AdapterKinds {
		n := 0
		for _, a := range hits {
			if a.Kind == kind {
				n++
			}
		}
		if n > 1 {
			return nil, Usagef("%s: more than one %s adapter matches", path, kind)
		}
	}
	return hits, nil
}

func (d *ExportDecl) ConsumerPath(path string) (string, error) {
	hits, err := d.AdaptersFor(path)
	if err != nil {
		return "", err
	}
	for _, a := range hits {
		if a.Kind == "prefix-namespace" {
			head, tail := "", path
			if idx := strings.LastIndex(path, "/"); idx >= 0 {
				head, tail = path[:idx], path[idx+1:]
			}
			if !strings.HasPrefix(tail, a.Prefix) {
				tail = a.Prefix + tail
			}
			if head != "" {
				return head + "/" + tail, nil
			}
			return tail, nil
		}
	}
	return path, nil
}

// -- profiles ----------------------------------------------------------------------

// ProfileInScope: whether reconcile-scope may offer `path` to this profile.
func (d *ExportDecl) ProfileInScope(profile, path string) bool {
	if profile == "" {
		return true // unbound consumer takes the whole surface
	}
	prof, ok := d.Profiles[profile]
	if !ok {
		return true
	}
	return MatchAny(path, prof.ExportInclude) && !MatchAny(path, prof.ExportExclude)
}
