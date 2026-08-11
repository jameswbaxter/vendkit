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
//
// Two front-matter shapes are supported for the field, and only the first
// occurrence within the leading front-matter block is touched (count=1):
//
//	field: "g1, g2, g3"      # inline: quoted/unquoted comma string, or [g1, g2]
//	field:                   # block list: one glob per `- item` line
//	  - g1
//	  - g2
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
	keep := func(g string) bool {
		own, owned := owners[g]
		return !owned || own[profile]
	}
	out, _ := scanField(data, fmField, keep, nil)
	return out
}

// FieldGlobs returns the glob items the glob-localise adapter reads from the
// document's fmField, and whether the field occurs in the front matter at
// all. It is the SAME walk localise() prunes through, so validity checks
// built on it cannot disagree with the adapter (issue #10).
func FieldGlobs(data []byte, fmField string) ([]string, bool) {
	var items []string
	_, found := scanField(data, fmField,
		func(string) bool { return true },
		func(g string) { items = append(items, g) })
	return items, found
}

// scanField locates the first fmField occurrence in the leading front-matter
// block and rewrites its value, keeping each glob item iff keep(item); every
// item parsed is also reported through saw (pre-prune; nil to skip). found
// reports whether the field was seen.
func scanField(data []byte, fmField string, keep func(string) bool, saw func(string)) (out []byte, found bool) {
	if !utf8.Valid(data) {
		return data, false
	}
	lines := strings.Split(string(data), "\n")
	fmEnd := frontMatterEnd(lines)
	if fmEnd < 0 {
		return data, false // no front matter -> nothing to localise
	}

	keyRe := regexp.MustCompile(`^(\s*)` + regexp.QuoteMeta(fmField) + `:(.*)$`)
	itemRe := regexp.MustCompile(`^(\s+)-\s+(.*?)\s*$`)

	kept := make([]string, 0, len(lines))
	done := false
	for i := 0; i < len(lines); i++ {
		if done || i > fmEnd {
			kept = append(kept, lines[i])
			continue
		}
		m := keyRe.FindStringSubmatch(lines[i])
		if m == nil {
			kept = append(kept, lines[i])
			continue
		}
		found = true
		indent, rest := m[1], strings.TrimSpace(m[2])
		if rest != "" && !strings.HasPrefix(rest, "#") {
			// Inline form.
			kept = append(kept, indent+fmField+": "+pruneInline(rest, keep, saw))
			done = true
			continue
		}
		// Block-list form: emit the key line, then filter the more-indented
		// `- item` lines that follow.
		kept = append(kept, lines[i])
		j := i + 1
		for j <= fmEnd {
			im := itemRe.FindStringSubmatch(lines[j])
			if im == nil || len(im[1]) <= len(indent) {
				break
			}
			item := strings.Trim(strings.TrimSpace(im[2]), `"'`)
			if item != "" && saw != nil {
				saw(item)
			}
			if item == "" || keep(item) {
				kept = append(kept, lines[j])
			}
			j++
		}
		i = j - 1
		done = true
	}
	return []byte(strings.Join(kept, "\n")), found
}

// frontMatterEnd returns the index of the closing `---` of the leading YAML
// front-matter block, or -1 if the document has none.
func frontMatterEnd(lines []string) int {
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return -1
	}
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			return i
		}
	}
	return -1
}

// pruneInline prunes a single-line glob value — a quoted/unquoted comma string
// or a `[g1, g2]` flow list — keeping items for which keep() is true and
// preserving the original quoting/flow wrapper. Each parsed item is reported
// through saw (nil to skip) before the keep decision.
func pruneInline(rest string, keep func(string) bool, saw func(string)) string {
	quote := ""
	if len(rest) >= 2 && (rest[0] == '"' || rest[0] == '\'') && rest[len(rest)-1] == rest[0] {
		quote = string(rest[0])
		rest = rest[1 : len(rest)-1]
	}
	flow := false
	if strings.HasPrefix(rest, "[") && strings.HasSuffix(rest, "]") {
		flow = true
		rest = strings.TrimSpace(rest[1 : len(rest)-1])
	}
	var kept []string
	for _, g := range strings.Split(rest, ",") {
		g = strings.TrimSpace(g)
		if g == "" {
			continue
		}
		item := strings.Trim(g, `"'`)
		if saw != nil {
			saw(item)
		}
		if keep(item) {
			kept = append(kept, g)
		}
	}
	joined := strings.Join(kept, ", ")
	if flow {
		return "[" + joined + "]"
	}
	return quote + joined + quote
}
