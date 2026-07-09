// The include/seed glob dialect: Python pathlib.Path.glob semantics as of
// the reference implementation, pinned by tests/vectors/pathlib-globs.json.
// Key differences from fnmatch: '*'/'?' do NOT cross '/', '**' spans zero
// or more DIRECTORIES (a trailing '**' therefore matches no files),
// dotfiles match, case-sensitive.

package core

import (
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
)

// TreeGlob returns sorted repo-relative posix paths of REGULAR FILES under
// root matching the pattern. Symlinks that match are returned with
// isSymlink=true entries so callers can reject them (declaration spec §2).
type TreeHit struct {
	Rel       string
	IsSymlink bool
}

func TreeGlob(root, pattern string) ([]TreeHit, error) {
	patSegs := strings.Split(pattern, "/")
	var hits []TreeHit
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == root {
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		rel = filepath.ToSlash(rel)
		if d.IsDir() {
			return nil // only files become entries
		}
		if matchSegments(strings.Split(rel, "/"), patSegs) {
			isLink := d.Type()&fs.ModeSymlink != 0
			hits = append(hits, TreeHit{Rel: rel, IsSymlink: isLink})
		}
		return nil
	})
	if err != nil {
		return nil, Errf("walking %s: %v", root, err)
	}
	sort.Slice(hits, func(i, j int) bool { return hits[i].Rel < hits[j].Rel })
	return hits, nil
}

// matchSegments matches a file's path segments against pattern segments.
// '**' consumes zero or more segments but may NOT consume the final (file)
// segment — pathlib's '**' matches directories only.
func matchSegments(segs, pat []string) bool {
	if len(pat) == 0 {
		return len(segs) == 0
	}
	if pat[0] == "**" {
		// Consume zero segments…
		if matchSegments(segs, pat[1:]) {
			return true
		}
		// …or one directory segment (never the final file segment).
		if len(segs) > 1 {
			return matchSegments(segs[1:], pat)
		}
		return false
	}
	if len(segs) == 0 {
		return false
	}
	if !segmentMatch(segs[0], pat[0]) {
		return false
	}
	return matchSegments(segs[1:], pat[1:])
}

// segmentMatch is fnmatch within one segment (no '/' present by
// construction, so '*'/'?' cannot cross it).
func segmentMatch(seg, pat string) bool {
	return fnmatchTranslate(pat).MatchString(seg)
}
