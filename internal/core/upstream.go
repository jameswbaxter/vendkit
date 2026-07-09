// Upstream reads over the git protocol — vendor-service-free (DR-0015).

package core

import (
	"os"
	"os/exec"
	"sort"
	"strings"
)

type Tag struct {
	Name   string
	Commit string
}

// CloneURL: something git can clone. Verbatim for URLs and paths;
// otherwise the scm-keyed shorthand expansion (data, not a service).
func CloneURL(scm, repo string) (string, error) {
	if strings.Contains(repo, "://") || strings.HasPrefix(repo, "/") ||
		strings.HasPrefix(repo, "./") || strings.HasPrefix(repo, "../") ||
		strings.HasPrefix(repo, "git@") {
		return repo, nil
	}
	parts := strings.Split(repo, "/")
	switch scm {
	case "github":
		if len(parts) != 2 {
			return "", Usagef("github repo shorthand must be owner/repo: %q", repo)
		}
		return "https://github.com/" + repo + ".git", nil
	case "azure-repos":
		if len(parts) != 3 {
			return "", Usagef("azure-repos shorthand must be org/project/repo: %q", repo)
		}
		return "https://dev.azure.com/" + parts[0] + "/" + parts[1] + "/_git/" + parts[2], nil
	}
	return "", Usagef("unknown scm %q", scm)
}

// ListReleaseTags: all tags with peeled commit SHAs via `git ls-remote`.
// No --refs: annotated tags list the tag OBJECT sha on the bare ref and the
// peeled commit on the `^{}` line — provenance needs the commit.
func ListReleaseTags(url string) ([]Tag, error) {
	cmd := exec.Command("git", "ls-remote", "--tags", url)
	out, err := cmd.Output()
	if err != nil {
		detail := err.Error()
		if ee, ok := err.(*exec.ExitError); ok {
			detail = strings.TrimSpace(string(ee.Stderr))
		}
		return nil, Errf("listing tags of %s failed: %s", url, detail)
	}
	shas := map[string]string{}
	for _, line := range strings.Split(string(out), "\n") {
		sha, ref, found := strings.Cut(line, "\t")
		if !found || !strings.HasPrefix(ref, "refs/tags/") {
			continue
		}
		name := ref[len("refs/tags/"):]
		if strings.HasSuffix(name, "^{}") {
			shas[name[:len(name)-3]] = sha // peeled commit wins
		} else if _, seen := shas[name]; !seen {
			shas[name] = sha
		}
	}
	var tags []Tag
	for name, sha := range shas {
		tags = append(tags, Tag{Name: name, Commit: sha})
	}
	sort.Slice(tags, func(i, j int) bool { return tags[i].Name < tags[j].Name })
	return tags, nil
}

// FetchPublisher: a working checkout of the publisher at `ref` in `dest`
// (human-tier diff/update). Depth-1 clone of just that tag.
func FetchPublisher(url, ref, dest string) error {
	cmd := exec.Command("git", "clone", "-q", "--depth", "1", "--branch", ref, url, dest)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return Errf("cloning %s at %s failed: %s", url, ref, strings.TrimSpace(string(out)))
	}
	return nil
}

// ReadFileAt: one file's bytes at a ref, without a full clone.
func ReadFileAt(url, ref, path string) ([]byte, error) {
	if fi, err := os.Stat(url); err == nil && fi.IsDir() {
		cmd := exec.Command("git", "-C", url, "show", ref+":"+path)
		out, err := cmd.Output()
		if err != nil {
			return nil, Errf("cannot read %s@%s from %s", path, ref, url)
		}
		return out, nil
	}
	tmp, err := os.MkdirTemp("", "vendkit-fetch-")
	if err != nil {
		return nil, Errf("mkdtemp: %v", err)
	}
	defer os.RemoveAll(tmp)
	for _, args := range [][]string{
		{"init", "-q"},
		{"fetch", "-q", "--depth", "1", url, "refs/tags/" + ref},
	} {
		cmd := exec.Command("git", append([]string{"-C", tmp}, args...)...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return nil, Errf("fetching %s from %s failed: %s",
				ref, url, strings.TrimSpace(string(out)))
		}
	}
	cmd := exec.Command("git", "-C", tmp, "show", "FETCH_HEAD:"+path)
	out, err := cmd.Output()
	if err != nil {
		return nil, Errf("cannot read %s@%s from %s", path, ref, url)
	}
	return out, nil
}
