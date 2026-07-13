package core

import (
	"os"
	"path/filepath"
	"testing"
)

func writeSubs(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "subscribers.yml")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadSubscribers_ValidWithDefaults(t *testing.T) {
	path := writeSubs(t, `schema_version: 1
subscribers:
  - repo: acme/leaf
  - repo: acme/other
    event_type: custom-event
    token_secret: LEAF_DISPATCH_TOKEN
    platform: github-actions
  - repo: acme/ado-consumer
    platform: azure-pipelines
`)
	subs, err := LoadSubscribers(path)
	if err != nil {
		t.Fatalf("LoadSubscribers: %v", err)
	}
	if len(subs) != 3 {
		t.Fatalf("got %d subscribers, want 3", len(subs))
	}
	// Defaults filled.
	if subs[0].EventType != DefaultPushHintEventType {
		t.Errorf("default event_type = %q, want %q", subs[0].EventType, DefaultPushHintEventType)
	}
	if subs[0].Platform != "github-actions" || !subs[0].IsGHA() {
		t.Errorf("default platform = %q, want github-actions", subs[0].Platform)
	}
	// Explicit values kept.
	if subs[1].EventType != "custom-event" || subs[1].TokenSecret != "LEAF_DISPATCH_TOKEN" {
		t.Errorf("entry 1 = %+v", subs[1])
	}
	// Non-GHA kept but flagged for skipping.
	if subs[2].IsGHA() {
		t.Errorf("azure-pipelines subscriber should not be GHA")
	}
}

func TestLoadSubscribers_Rejects(t *testing.T) {
	cases := map[string]string{
		"no schema": `subscribers:
  - repo: acme/leaf
`,
		"no subscribers list": `schema_version: 1
`,
		"gha missing repo": `schema_version: 1
subscribers:
  - platform: github-actions
`,
		"bad repo shape": `schema_version: 1
subscribers:
  - repo: acme
`,
		"unknown platform": `schema_version: 1
subscribers:
  - repo: acme/leaf
    platform: gitlab-ci
`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			path := writeSubs(t, body)
			if _, err := LoadSubscribers(path); err == nil {
				t.Fatalf("expected error for %q, got nil", name)
			}
		})
	}
}

func TestLoadSubscribers_MissingFileIsUsageError(t *testing.T) {
	_, err := LoadSubscribers(filepath.Join(t.TempDir(), "absent.yml"))
	if err == nil {
		t.Fatal("expected error for a missing file")
	}
	if _, ok := err.(*UsageError); !ok {
		t.Errorf("missing file error type = %T, want *UsageError", err)
	}
}
