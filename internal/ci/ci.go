// Package ci is the CI output surface — the only in-process platform
// adaptation in the engine (platform-integration spec §2). Every surface
// also prints the plain key=value line, for log-greppability.
package ci

import (
	"fmt"
	"os"
)

func Detect() string {
	if override := os.Getenv("VENDKIT_PLATFORM"); override != "" {
		return override
	}
	if os.Getenv("GITHUB_ACTIONS") == "true" {
		return "github-actions"
	}
	if os.Getenv("TF_BUILD") == "True" {
		return "azure-pipelines"
	}
	return "neutral"
}

type Surface interface {
	EmitOutput(key, value string)
	EmitSummary(markdown string)
	EmitError(message string)
}

type Neutral struct{}

func (Neutral) EmitOutput(key, value string) { fmt.Printf("%s=%s\n", key, value) }
func (Neutral) EmitSummary(markdown string)  { fmt.Fprintln(os.Stderr, markdown) }
func (Neutral) EmitError(message string)     { fmt.Fprintf(os.Stderr, "ERROR: %s\n", message) }

type GitHubActions struct{}

func (GitHubActions) EmitOutput(key, value string) {
	fmt.Printf("%s=%s\n", key, value)
	if out := os.Getenv("GITHUB_OUTPUT"); out != "" {
		if fh, err := os.OpenFile(out, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644); err == nil {
			fmt.Fprintf(fh, "%s=%s\n", key, value)
			fh.Close()
		}
	}
}

func (GitHubActions) EmitSummary(markdown string) {
	if path := os.Getenv("GITHUB_STEP_SUMMARY"); path != "" {
		if fh, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644); err == nil {
			fmt.Fprintln(fh, markdown)
			fh.Close()
			return
		}
	}
	fmt.Fprintln(os.Stderr, markdown)
}

func (GitHubActions) EmitError(message string) { fmt.Printf("::error::%s\n", message) }

type AzurePipelines struct{}

func (AzurePipelines) EmitOutput(key, value string) {
	fmt.Printf("%s=%s\n", key, value)
	fmt.Printf("##vso[task.setvariable variable=%s;isOutput=true]%s\n", key, value)
}

func (AzurePipelines) EmitSummary(markdown string) {
	if path := os.Getenv("VENDKIT_ADO_SUMMARY_FILE"); path != "" {
		if fh, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644); err == nil {
			fmt.Fprintln(fh, markdown)
			fh.Close()
			fmt.Printf("##vso[task.uploadsummary]%s\n", path)
			return
		}
	}
	fmt.Fprintln(os.Stderr, markdown)
}

func (AzurePipelines) EmitError(message string) {
	fmt.Printf("##vso[task.logissue type=error]%s\n", message)
}

func GetSurface(name string) (Surface, error) {
	if name == "" {
		name = Detect()
	}
	switch name {
	case "github-actions":
		return GitHubActions{}, nil
	case "azure-pipelines":
		return AzurePipelines{}, nil
	case "neutral":
		return Neutral{}, nil
	}
	return nil, fmt.Errorf("unknown CI surface %q (expected one of "+
		"[github-actions azure-pipelines neutral])", name)
}
