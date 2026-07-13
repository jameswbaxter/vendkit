package ci

import (
	"io"
	"os"
	"strings"
	"testing"
)

// captureStdout runs fn with os.Stdout redirected to a pipe and returns what
// was written. The surfaces print the neutral key=value line to stdout on
// every platform (log-greppability, ci.go package doc), plus the ADO
// ##vso directives, so stdout is where most dialect assertions live.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w
	defer func() { os.Stdout = orig }()
	fn()
	w.Close()
	out, _ := io.ReadAll(r)
	return string(out)
}

// --- Detect: precedence and the two platform triggers ------------------------

func TestDetectPrecedence(t *testing.T) {
	// Isolate every signal Detect reads.
	for _, k := range []string{"VENDKIT_PLATFORM", "GITHUB_ACTIONS", "TF_BUILD"} {
		t.Setenv(k, "")
	}

	cases := []struct {
		name string
		env  map[string]string
		want string
	}{
		{"default is neutral", nil, "neutral"},
		{"github via GITHUB_ACTIONS", map[string]string{"GITHUB_ACTIONS": "true"}, "github-actions"},
		{"azure via TF_BUILD", map[string]string{"TF_BUILD": "True"}, "azure-pipelines"},
		{"override wins over github", map[string]string{"VENDKIT_PLATFORM": "neutral", "GITHUB_ACTIONS": "true"}, "neutral"},
		{"override wins over azure", map[string]string{"VENDKIT_PLATFORM": "github-actions", "TF_BUILD": "True"}, "github-actions"},
		{"github wins over azure when both set", map[string]string{"GITHUB_ACTIONS": "true", "TF_BUILD": "True"}, "github-actions"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for _, k := range []string{"VENDKIT_PLATFORM", "GITHUB_ACTIONS", "TF_BUILD"} {
				t.Setenv(k, "")
			}
			for k, v := range tc.env {
				t.Setenv(k, v)
			}
			if got := Detect(); got != tc.want {
				t.Fatalf("Detect() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestGetSurfaceUnknownIsError(t *testing.T) {
	if _, err := GetSurface("gitlab-ci"); err == nil {
		t.Fatal("expected error for unknown surface, got nil")
	}
	for _, name := range []string{"github-actions", "azure-pipelines", "neutral"} {
		if _, err := GetSurface(name); err != nil {
			t.Fatalf("GetSurface(%q) errored: %v", name, err)
		}
	}
}

// --- GitHub Actions dialect ---------------------------------------------------

func TestGitHubActionsOutputWiring(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "gh_output")
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	t.Setenv("GITHUB_OUTPUT", f.Name())

	out := captureStdout(t, func() {
		GitHubActions{}.EmitOutput("changed", "true")
	})

	// stdout still carries the greppable neutral line.
	if strings.TrimSpace(out) != "changed=true" {
		t.Fatalf("stdout = %q, want %q", strings.TrimSpace(out), "changed=true")
	}
	// and the value is appended to the $GITHUB_OUTPUT file for downstream steps.
	body, _ := os.ReadFile(f.Name())
	if !strings.Contains(string(body), "changed=true") {
		t.Fatalf("GITHUB_OUTPUT file = %q, want it to contain %q", body, "changed=true")
	}
}

func TestGitHubActionsOutputWithoutFileStillPrints(t *testing.T) {
	t.Setenv("GITHUB_OUTPUT", "") // no file → must not panic, still greppable
	out := captureStdout(t, func() {
		GitHubActions{}.EmitOutput("pinned", "v1.2.0")
	})
	if strings.TrimSpace(out) != "pinned=v1.2.0" {
		t.Fatalf("stdout = %q", out)
	}
}

func TestGitHubActionsErrorAnnotation(t *testing.T) {
	out := captureStdout(t, func() {
		GitHubActions{}.EmitError("manifest is stale")
	})
	if strings.TrimSpace(out) != "::error::manifest is stale" {
		t.Fatalf("EmitError = %q, want %q", strings.TrimSpace(out), "::error::manifest is stale")
	}
}

func TestGitHubActionsSummaryAppendsToStepSummary(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "gh_summary")
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	t.Setenv("GITHUB_STEP_SUMMARY", f.Name())
	GitHubActions{}.EmitSummary("## Gate: 0 findings")
	body, _ := os.ReadFile(f.Name())
	if !strings.Contains(string(body), "## Gate: 0 findings") {
		t.Fatalf("step summary file = %q", body)
	}
}

// --- Azure Pipelines dialect --------------------------------------------------

func TestAzurePipelinesOutputMapping(t *testing.T) {
	out := captureStdout(t, func() {
		AzurePipelines{}.EmitOutput("changed", "true")
	})
	// Both the greppable line and the ##vso setvariable (isOutput=true so a
	// later job/step can consume it) must appear.
	if !strings.Contains(out, "changed=true") {
		t.Fatalf("missing greppable line in %q", out)
	}
	want := "##vso[task.setvariable variable=changed;isOutput=true]true"
	if !strings.Contains(out, want) {
		t.Fatalf("missing ##vso mapping\n got: %q\nwant substring: %q", out, want)
	}
}

func TestAzurePipelinesErrorLogIssue(t *testing.T) {
	out := captureStdout(t, func() {
		AzurePipelines{}.EmitError("manifest is stale")
	})
	want := "##vso[task.logissue type=error]manifest is stale"
	if strings.TrimSpace(out) != want {
		t.Fatalf("EmitError = %q, want %q", strings.TrimSpace(out), want)
	}
}

func TestAzurePipelinesSummaryUploads(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "ado_summary")
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	t.Setenv("VENDKIT_ADO_SUMMARY_FILE", f.Name())
	out := captureStdout(t, func() {
		AzurePipelines{}.EmitSummary("## Gate: 0 findings")
	})
	body, _ := os.ReadFile(f.Name())
	if !strings.Contains(string(body), "## Gate: 0 findings") {
		t.Fatalf("summary file = %q", body)
	}
	if !strings.Contains(out, "##vso[task.uploadsummary]"+f.Name()) {
		t.Fatalf("missing uploadsummary directive in %q", out)
	}
}
