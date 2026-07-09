// Golden fidelity vectors (DR-0017): the same data files the Python
// reference asserts. Parity here is the precondition for everything else.

package core

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func vectors(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "tests", "vectors", name))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestNormalisationVectors(t *testing.T) {
	var cases []struct {
		Name     string `json:"name"`
		InputB64 string `json:"input_b64"`
		SHA256   string `json:"sha256"`
		Raw      bool   `json:"raw"`
	}
	if err := json.Unmarshal(vectors(t, "normalisation.json"), &cases); err != nil {
		t.Fatal(err)
	}
	for _, c := range cases {
		data, _ := base64.StdEncoding.DecodeString(c.InputB64)
		digest, raw := NormaliseHash(data)
		if digest != c.SHA256 || raw != c.Raw {
			t.Errorf("%s: got (%s, %v), want (%s, %v)",
				c.Name, digest, raw, c.SHA256, c.Raw)
		}
	}
}

func TestFnmatchGlobVectors(t *testing.T) {
	var cases []struct {
		Path    string `json:"path"`
		Pattern string `json:"pattern"`
		Match   bool   `json:"match"`
	}
	if err := json.Unmarshal(vectors(t, "fnmatch-globs.json"), &cases); err != nil {
		t.Fatal(err)
	}
	for _, c := range cases {
		if got := PathMatch(c.Path, c.Pattern); got != c.Match {
			t.Errorf("PathMatch(%q, %q) = %v, want %v",
				c.Path, c.Pattern, got, c.Match)
		}
	}
}

func TestPathlibGlobVectors(t *testing.T) {
	var doc struct {
		Tree  []string `json:"tree"`
		Cases []struct {
			Pattern string   `json:"pattern"`
			Matches []string `json:"matches"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(vectors(t, "pathlib-globs.json"), &doc); err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	for _, rel := range doc.Tree {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("x\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, c := range doc.Cases {
		hits, err := TreeGlob(root, c.Pattern)
		if err != nil {
			t.Fatal(err)
		}
		got := make([]string, len(hits))
		for i, h := range hits {
			got[i] = h.Rel
		}
		want := c.Matches
		if len(got) != len(want) {
			t.Errorf("%q: got %v, want %v", c.Pattern, got, want)
			continue
		}
		for i := range got {
			if got[i] != want[i] {
				t.Errorf("%q: got %v, want %v", c.Pattern, got, want)
				break
			}
		}
	}
}

func TestCanonicalManifestVector(t *testing.T) {
	var manifest any
	if err := json.Unmarshal(vectors(t, "canonical-manifest.input.json"), &manifest); err != nil {
		t.Fatal(err)
	}
	got := CanonicalJSON(manifest)
	want := vectors(t, "canonical-manifest.expected.json")
	if string(got) != string(want) {
		t.Errorf("canonical JSON mismatch:\n--- got ---\n%s\n--- want ---\n%s",
			got, want)
	}
}
