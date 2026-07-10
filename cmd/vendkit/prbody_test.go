// Ported from tests/test_scenarios.py::test_seed_notes_silent_suppresses_pr_section.
// The Python analogue tests vendkit.cli._pr_body with a SyncReport; the Go
// analogue is prBody over core.SyncReport.

package main

import (
	"strings"
	"testing"

	"github.com/jameswbaxter/vendkit/internal/core"
)

func TestSeedNotesSilentSuppressesPRSection(t *testing.T) {
	report := &core.SyncReport{
		Updated:         []string{"docs/x.md"},
		TemplateUpdated: []string{"templates/CONTRIBUTING.md"},
	}
	loud := prBody("docs", "v1", "v2", report, nil, nil, "informational")
	quiet := prBody("docs", "v1", "v2", report, nil, nil, "silent")
	if !strings.Contains(loud, "upstream template changed") {
		t.Errorf("informational seed notes must surface the template section:\n%s", loud)
	}
	if strings.Contains(quiet, "upstream template changed") {
		t.Errorf("silent seed notes must suppress the template section:\n%s", quiet)
	}
}
