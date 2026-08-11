package main

import "testing"

// capturedEngineBaseURL (issue #15): only a provably version-independent
// root is committed — the per-release form <root>/<version> with its tail
// stripped — and only for publishers whose engine URL cannot be derived.
func TestCapturedEngineBaseURL(t *testing.T) {
	cases := []struct {
		name, scm, env, want string
	}{
		{"azure per-release URL is rooted",
			"azure-repos", "https://mirror.example/vendkit/v1.2.3", "https://mirror.example/vendkit"},
		{"trailing slash tolerated",
			"azure-repos", "https://mirror.example/vendkit/v1.2.3/", "https://mirror.example/vendkit"},
		{"github publisher never captures (derivation covers it)",
			"github", "https://mirror.example/vendkit/v1.2.3", ""},
		{"non-per-release URL is not provably version-independent",
			"azure-repos", "https://mirror.example/flat", ""},
		{"unset env captures nothing", "azure-repos", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("VENDKIT_BASE_URL", tc.env)
			if got := capturedEngineBaseURL(tc.scm, "v1.2.3"); got != tc.want {
				t.Errorf("capturedEngineBaseURL(%q) with %q = %q, want %q",
					tc.scm, tc.env, got, tc.want)
			}
		})
	}
}
