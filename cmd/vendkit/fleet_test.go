// Fleet aggregation (conformance spec §5): folds conformance --json documents
// into one fleet report. These exercise the parsing contract, the aggregate
// ordering/census, and the human + --json surfaces over synthetic documents.

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jameswbaxter/vendkit/internal/core"
)

func rule(status string) *core.RuleResult {
	return &core.RuleResult{RuleID: "r-" + status, Title: "t", Severity: "advisory", Status: status}
}

// doc builds a conformance document with the given rule statuses; gap_count is
// derived so the fixtures stay internally consistent.
func doc(slice, profile, pin string, statuses ...string) *core.ConformanceDoc {
	d := &core.ConformanceDoc{Slice: slice, Profile: profile}
	if pin != "" {
		d.Pin = &core.PinDoc{Version: pin}
	}
	for _, s := range statuses {
		r := rule(s)
		d.Rules = append(d.Rules, r)
		if r.IsGap() {
			d.GapCount++
		}
	}
	return d
}

func TestAggregateFleetCensusAndOrdering(t *testing.T) {
	docs := []*core.ConformanceDoc{
		doc("clean", "code-repo", "v1.0.0", "pass", "pass"),
		doc("failing", "code-repo", "v0.9.0", "pass", "fail", "fail"),
		doc("erroring", "docs", "v0.8.0", "error", "pass"),
		doc("skipping", "code-repo", "v1.0.0", "skipped", "pass"),
	}
	r := core.AggregateFleet(docs)

	if r.TotalConsumers != 4 {
		t.Errorf("total_consumers = %d, want 4", r.TotalConsumers)
	}
	if r.TotalGaps != 3 { // failing(2) + erroring(1)
		t.Errorf("total_gaps = %d, want 3", r.TotalGaps)
	}
	wantCensus := map[string]int{"error": 1, "fail": 1, "skipped": 1, "pass": 1}
	for k, v := range wantCensus {
		if r.ByWorstStatus[k] != v {
			t.Errorf("by_worst_status[%s] = %d, want %d", k, r.ByWorstStatus[k], v)
		}
	}
	// Worst offenders first: error > fail > skipped > pass.
	gotOrder := []string{}
	for _, row := range r.Consumers {
		gotOrder = append(gotOrder, row.WorstStatus)
	}
	wantOrder := []string{"error", "fail", "skipped", "pass"}
	if strings.Join(gotOrder, ",") != strings.Join(wantOrder, ",") {
		t.Errorf("worst-first ordering = %v, want %v", gotOrder, wantOrder)
	}
	if r.Consumers[0].Slice != "erroring" {
		t.Errorf("top offender = %q, want erroring", r.Consumers[0].Slice)
	}
}

func TestAggregateFleetPinLagIsNull(t *testing.T) {
	r := core.AggregateFleet([]*core.ConformanceDoc{doc("s", "p", "v1.0.0", "pass")})
	out, _ := json.Marshal(r)
	if !strings.Contains(string(out), `"pin_lag":null`) {
		t.Errorf("expected pin_lag:null in fleet JSON (offline), got:\n%s", out)
	}
}

func TestLoadFleetDocsFromDirAndArray(t *testing.T) {
	dir := t.TempDir()
	// One file with a single object.
	writeDoc(t, filepath.Join(dir, "a.json"), doc("alpha", "code-repo", "v1.0.0", "pass"))
	// One file with a JSON array of two documents.
	arr, _ := json.Marshal([]*core.ConformanceDoc{
		doc("beta", "code-repo", "v1.0.0", "fail"),
		doc("gamma", "docs", "v1.0.0", "pass"),
	})
	if err := os.WriteFile(filepath.Join(dir, "b.json"), arr, 0o644); err != nil {
		t.Fatal(err)
	}
	docs, err := loadFleetDocs([]string{dir}, nil)
	if err != nil {
		t.Fatalf("loadFleetDocs: %v", err)
	}
	if len(docs) != 3 {
		t.Fatalf("loaded %d docs, want 3", len(docs))
	}
}

func TestLoadFleetDocsFromStdinStream(t *testing.T) {
	// Two concatenated / newline-delimited objects on stdin.
	a, _ := json.Marshal(doc("alpha", "code-repo", "v1.0.0", "pass"))
	b, _ := json.Marshal(doc("beta", "code-repo", "v1.0.0", "fail"))
	stdin := strings.NewReader(string(a) + "\n" + string(b) + "\n")
	docs, err := loadFleetDocs(nil, stdin)
	if err != nil {
		t.Fatalf("loadFleetDocs: %v", err)
	}
	if len(docs) != 2 {
		t.Fatalf("loaded %d docs, want 2", len(docs))
	}
}

func TestLoadFleetDocsRejectsNonConformanceFile(t *testing.T) {
	dir := t.TempDir()
	bad := filepath.Join(dir, "junk.json")
	if err := os.WriteFile(bad, []byte(`{"hello":"world"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := loadFleetDocs([]string{bad}, nil)
	if err == nil || !strings.Contains(err.Error(), "junk.json") {
		t.Fatalf("expected a loud error naming junk.json, got %v", err)
	}
}

func TestLoadFleetDocsRejectsMalformedFile(t *testing.T) {
	dir := t.TempDir()
	bad := filepath.Join(dir, "broken.json")
	if err := os.WriteFile(bad, []byte(`{not json`), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := loadFleetDocs([]string{bad}, nil)
	if err == nil || !strings.Contains(err.Error(), "broken.json") {
		t.Fatalf("expected a loud error naming broken.json, got %v", err)
	}
}

func TestPrintFleetHumanSurface(t *testing.T) {
	r := core.AggregateFleet([]*core.ConformanceDoc{
		doc("failing", "code-repo", "v0.9.0", "fail"),
		doc("clean", "code-repo", "v1.0.0", "pass"),
	})
	out := captureStdout(t, func() { printFleetHuman(r) })
	if !strings.Contains(out, "fleet: 2 consumer(s), 1 total gap(s)") {
		t.Errorf("missing fleet summary line:\n%s", out)
	}
	if !strings.Contains(out, "fail=1") || !strings.Contains(out, "pass=1") {
		t.Errorf("missing worst-status census:\n%s", out)
	}
	// Worst offender listed first, pin and lag rendered.
	body := out[strings.Index(out, "WORST"):]
	if strings.Index(body, "failing") > strings.Index(body, "clean") {
		t.Errorf("worst offender not listed first:\n%s", out)
	}
	if !strings.Contains(out, "v0.9.0") {
		t.Errorf("pin not rendered:\n%s", out)
	}
}

// -- helpers --------------------------------------------------------------------

func writeDoc(t *testing.T, path string, d *core.ConformanceDoc) {
	t.Helper()
	b, err := json.Marshal(d)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	orig := os.Stdout
	rd, wr, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = wr
	fn()
	wr.Close()
	os.Stdout = orig
	var sb strings.Builder
	buf := make([]byte, 4096)
	for {
		n, err := rd.Read(buf)
		sb.Write(buf[:n])
		if err != nil {
			break
		}
	}
	return sb.String()
}
