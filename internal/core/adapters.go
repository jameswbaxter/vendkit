// Content adapters (DR-0009): identity copy by default; two named
// transforms. Deterministic pure functions of (bytes, params, profile name).

package core

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

// ApplyAdapters transforms file content for a consumer. Path renaming is
// separate (ConsumerPath); this handles bytes only.
func ApplyAdapters(decl *ExportDecl, path string, data []byte, profile string) ([]byte, error) {
	hits, err := decl.AdaptersFor(path)
	if err != nil {
		return nil, err
	}
	for _, adapter := range hits {
		if adapter.Kind == "glob-localise" {
			data = localise(data, adapter.FMField, adapter.Catalogue, profile)
		}
	}
	return data, nil
}

// localise prunes a front-matter glob union to the consumer's profile:
// keep a glob iff owned by the profile or owned by no profile (universal)
// — export-declaration spec §3. Unbound consumers keep the union verbatim.
func localise(data []byte, fmField string, catalogue map[string][]string, profile string) []byte {
	if profile == "" {
		return data
	}
	owners := map[string]map[string]bool{}
	for pname, globs := range catalogue {
		for _, g := range globs {
			if owners[g] == nil {
				owners[g] = map[string]bool{}
			}
			owners[g][pname] = true
		}
	}
	if !utf8.Valid(data) {
		return data
	}
	text := string(data)

	// The field is a single front-matter line: `field: "g1, g2, g3"`.
	pattern := regexp.MustCompile(
		`(?m)^(` + regexp.QuoteMeta(fmField) + `:\s*)(["']?)(.*?)(["']?)\s*$`)

	replaced := false
	out := pattern.ReplaceAllStringFunc(text, func(m string) string {
		if replaced {
			return m // count=1 semantics
		}
		sub := pattern.FindStringSubmatch(m)
		// Backreference \2 == \4 (matching quote pair) — Go regexp has no
		// backrefs; enforce by hand.
		if sub[2] != sub[4] {
			return m
		}
		replaced = true
		var kept []string
		for _, g := range strings.Split(sub[3], ",") {
			g = strings.TrimSpace(g)
			if g == "" {
				continue
			}
			own, owned := owners[g]
			if !owned || own[profile] {
				kept = append(kept, g)
			}
		}
		return sub[1] + sub[2] + strings.Join(kept, ", ") + sub[4]
	})
	return []byte(out)
}
