// Scenario kit (testing.md §2), ported from tests/test_scenarios.py:
// throwaway publisher/consumer git repos, the vendkit CLI driven end-to-end
// under the neutral CI surface, no network.
//
// The Python harness ran the reference CLI (or, with VENDKIT_CLI set, the Go
// binary) as a subprocess. This port always exercises the freshly built Go
// binary. The one Python-specific coupling — the reference journal handler
// module wired into the scaffolded slice config — is replaced by an
// equivalent Go journal-handler binary (internal/e2e/journalhandler), which
// the world fixture wires into the slice config exactly as the Python fixture
// swapped in vendkit.handlers.journal.
package e2e

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

var (
	vendkitBin string // freshly built ./cmd/vendkit
	journalBin string // freshly built ./internal/e2e/journalhandler
)

func TestMain(m *testing.M) {
	tmp, err := os.MkdirTemp("", "vendkit-e2e-")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	vendkitBin = filepath.Join(tmp, "vendkit")
	journalBin = filepath.Join(tmp, "journal")
	build := func(out, pkg string) {
		cmd := exec.Command("go", "build", "-o", out, pkg)
		cmd.Env = os.Environ()
		if b, err := cmd.CombinedOutput(); err != nil {
			fmt.Fprintf(os.Stderr, "build %s failed: %v\n%s", pkg, err, b)
			os.RemoveAll(tmp)
			os.Exit(1)
		}
	}
	build(vendkitBin, "github.com/jameswbaxter/vendkit/cmd/vendkit")
	build(journalBin, "github.com/jameswbaxter/vendkit/internal/e2e/journalhandler")
	code := m.Run()
	os.RemoveAll(tmp)
	os.Exit(code)
}

// -- process + env helpers -------------------------------------------------------

// mergedEnv mirrors the Python harness env: neutral CI surface plus a fully
// isolated, deterministic git identity (so vendkit's own git commits/tags do
// not depend on the ambient ~/.gitconfig).
func mergedEnv(extra map[string]string) []string {
	overrides := map[string]string{
		"VENDKIT_PLATFORM":    "neutral",
		"GIT_CONFIG_GLOBAL":   "/dev/null",
		"GIT_CONFIG_SYSTEM":   "/dev/null",
		"GIT_AUTHOR_NAME":     "t",
		"GIT_AUTHOR_EMAIL":    "t@invalid",
		"GIT_COMMITTER_NAME":  "t",
		"GIT_COMMITTER_EMAIL": "t@invalid",
	}
	for k, v := range extra {
		overrides[k] = v
	}
	var out []string
	for _, e := range os.Environ() {
		i := strings.IndexByte(e, '=')
		if i < 0 {
			out = append(out, e)
			continue
		}
		if _, ok := overrides[e[:i]]; ok {
			continue
		}
		out = append(out, e)
	}
	for k, v := range overrides {
		out = append(out, k+"="+v)
	}
	return out
}

func runCmd(t *testing.T, name, dir string, env map[string]string, args ...string) (string, string, int) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Env = mergedEnv(env)
	var so, se bytes.Buffer
	cmd.Stdout, cmd.Stderr = &so, &se
	err := cmd.Run()
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			return so.String(), se.String(), ee.ExitCode()
		}
		t.Fatalf("failed to run %s %v: %v", name, args, err)
	}
	return so.String(), se.String(), 0
}

// vk drives the vendkit binary. When check is true a nonzero exit is fatal.
func vk(t *testing.T, dir string, env map[string]string, check bool, args ...string) (string, string, int) {
	t.Helper()
	so, se, code := runCmd(t, vendkitBin, dir, env, args...)
	if check && code != 0 {
		t.Fatalf("vendkit %s -> %d\nstdout:\n%s\nstderr:\n%s",
			strings.Join(args, " "), code, so, se)
	}
	return so, se, code
}

// git runs git with the harness's fixed committer identity.
func git(t *testing.T, dir string, args ...string) {
	t.Helper()
	full := append([]string{"-c", "user.name=t", "-c", "user.email=t@invalid",
		"-c", "commit.gpgsign=false"}, args...)
	so, se, code := runCmd(t, "git", dir, nil, full...)
	if code != 0 {
		t.Fatalf("git %s in %s -> %d\n%s\n%s", strings.Join(args, " "), dir, code, so, se)
	}
}

func gitOut(t *testing.T, dir string, args ...string) string {
	t.Helper()
	so, se, code := runCmd(t, "git", dir, nil, args...)
	if code != 0 {
		t.Fatalf("git %s in %s -> %d\n%s", strings.Join(args, " "), dir, code, se)
	}
	return so
}

// -- fs helpers ------------------------------------------------------------------

func mkdirAll(t *testing.T, p string) {
	t.Helper()
	if err := os.MkdirAll(p, 0o755); err != nil {
		t.Fatal(err)
	}
}

func write(t *testing.T, p, content string) {
	t.Helper()
	mkdirAll(t, filepath.Dir(p))
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeBytes(t *testing.T, p string, b []byte) {
	t.Helper()
	if err := os.WriteFile(p, b, 0o644); err != nil {
		t.Fatal(err)
	}
}

func read(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func readBytes(t *testing.T, p string) []byte {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func appendText(t *testing.T, p, extra string) { write(t, p, read(t, p)+extra) }

func exists(p string) bool { _, err := os.Stat(p); return err == nil }

func chmodOff(t *testing.T, p string) {
	t.Helper()
	fi, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(p, fi.Mode()&^0o111); err != nil {
		t.Fatal(err)
	}
}

func chmodExec(t *testing.T, p string) {
	t.Helper()
	fi, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(p, fi.Mode()|0o111); err != nil {
		t.Fatal(err)
	}
}

// -- json helpers ----------------------------------------------------------------

func loadJSON(t *testing.T, p string) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(readBytes(t, p), &m); err != nil {
		t.Fatalf("parse %s: %v", p, err)
	}
	return m
}

func writeJSON(t *testing.T, p string, v any) {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	writeBytes(t, p, b)
}

func toStr(v any) string { s, _ := v.(string); return s }

func entriesOf(m map[string]any) []any {
	e, _ := m["entries"].([]any)
	return e
}

func asMap(v any) map[string]any { m, _ := v.(map[string]any); return m }

// parseDocFrom parses the JSON object beginning at the first '{' in s.
func parseDocFrom(t *testing.T, s string) map[string]any {
	t.Helper()
	i := strings.Index(s, "{")
	if i < 0 {
		t.Fatalf("no JSON object in output:\n%s", s)
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(s[i:]), &m); err != nil {
		t.Fatalf("parse doc: %v\n%s", err, s[i:])
	}
	return m
}

// parseArrayFrom parses the JSON array beginning at the first '[' in s.
func parseArrayFrom(t *testing.T, s string) []any {
	t.Helper()
	i := strings.Index(s, "[")
	if i < 0 {
		t.Fatalf("no JSON array in output:\n%s", s)
	}
	var a []any
	if err := json.Unmarshal([]byte(s[i:]), &a); err != nil {
		t.Fatalf("parse array: %v\n%s", err, s[i:])
	}
	return a
}

func journalRecords(t *testing.T, p string) []map[string]any {
	t.Helper()
	var out []map[string]any
	for _, line := range strings.Split(strings.TrimSpace(read(t, p)), "\n") {
		if line == "" {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			t.Fatalf("parse journal line: %v\n%s", err, line)
		}
		out = append(out, m)
	}
	return out
}

func recordsOfKind(records []map[string]any, kind string) []map[string]any {
	var out []map[string]any
	for _, r := range records {
		if toStr(r["kind"]) == kind {
			out = append(out, r)
		}
	}
	return out
}

func mustContain(t *testing.T, haystack, needle string) {
	t.Helper()
	if !strings.Contains(haystack, needle) {
		t.Fatalf("expected to find %q in:\n%s", needle, haystack)
	}
}

// -- world fixture ---------------------------------------------------------------

// world builds a publisher (with origin) at v0.1.0 and an onboarded consumer
// (with origin), mirroring the Python `world` fixture.
func world(t *testing.T) (pub, con string) {
	t.Helper()
	tmp := t.TempDir()
	pub = filepath.Join(tmp, "publisher")

	write(t, filepath.Join(pub, "docs", "standard.md"), "# Standard\n\nRule one.\n")
	write(t, filepath.Join(pub, "docs", "guide.md"), "# Guide\n")
	write(t, filepath.Join(pub, "templates", "CONTRIBUTING.md"),
		"# Contributing\n\nStarter text — customise me.\n")
	tool := filepath.Join(pub, "tools", "check")
	write(t, tool, "#!/bin/sh\nexit 0\n")
	chmodExec(t, tool)
	write(t, filepath.Join(pub, "vendkit-export.yml"), `schema_version: 1
slice: {name: docs, title: "Design docs"}
publisher: {scm: github, repo: example-org/pub}
include: ["docs/**/*.md", "tools/*"]
seed: ["templates/*.md"]
exclude: ["**/TEMPLATE.md"]
profiles:
  code-repo: {}
`)
	git(t, pub, "init", "-q", "-b", "main")
	vk(t, pub, nil, true, "generate")
	git(t, pub, "add", "-A")
	git(t, pub, "commit", "-q", "-m", "init")
	pubOrigin := filepath.Join(tmp, "publisher-origin.git")
	git(t, tmp, "init", "-q", "--bare", pubOrigin)
	git(t, pub, "remote", "add", "origin", pubOrigin)
	git(t, pub, "push", "-q", "origin", "main")
	vk(t, pub, nil, true, "release", "--version", "v0.1.0")

	con = filepath.Join(tmp, "consumer")
	mkdirAll(t, con)
	write(t, filepath.Join(con, "CODEOWNERS"), "/.vendkit/ @example-org/owners\n")
	git(t, con, "init", "-q", "-b", "main")
	vk(t, pub, nil, true, "init", "--ci", "github-actions", "--scm", "github",
		"--version", "v0.1.0", "--profile", "code-repo",
		"--publisher-root", pub, "--consumer-root", con)
	// Local-world coordinates: the publisher is a path git can clone, and
	// deliveries go to the (Go) journal handler instead of the GitHub API.
	cfgPath := filepath.Join(con, ".vendkit", "docs.yml")
	cfg := read(t, cfgPath)
	cfg = strings.ReplaceAll(cfg, "repo: example-org/pub", "repo: "+pub)
	cfg = strings.ReplaceAll(cfg, "[python3, -m, vendkit.handlers.github]",
		`["`+journalBin+`"]`)
	write(t, cfgPath, cfg)
	git(t, con, "add", "-A")
	git(t, con, "commit", "-q", "-m", "onboard docs slice v0.1.0")
	conOrigin := filepath.Join(tmp, "consumer-origin.git")
	git(t, tmp, "init", "-q", "--bare", conOrigin)
	git(t, con, "remote", "add", "origin", conOrigin)
	git(t, con, "push", "-q", "origin", "main")
	return pub, con
}

func release(t *testing.T, pub, version string) {
	t.Helper()
	vk(t, pub, nil, true, "generate")
	git(t, pub, "add", "-A")
	git(t, pub, "commit", "-q", "-m", "prep "+version)
	git(t, pub, "push", "-q", "origin", "main")
	vk(t, pub, nil, true, "release", "--version", version)
}

// -- gate lane -------------------------------------------------------------------

func TestCleanTreePassesStrictGate(t *testing.T) {
	_, con := world(t)
	so, _, _ := vk(t, con, nil, true, "gate", "--strict")
	mustContain(t, so, "findings=0")
}

func TestHandEditFailsStrictAdvisoryReports(t *testing.T) {
	_, con := world(t)
	appendText(t, filepath.Join(con, "docs", "standard.md"), "sneaky edit\n")
	so, _, code := vk(t, con, nil, false, "gate", "--strict")
	if code != 1 {
		t.Fatalf("strict gate exit = %d, want 1", code)
	}
	mustContain(t, so, "changed: docs/standard.md")
	advisory, _, _ := vk(t, con, nil, true, "gate") // advisory: reports, exits 0
	mustContain(t, advisory, "findings=1")
}

func TestDeleteAndChmodAreDrift(t *testing.T) {
	_, con := world(t)
	if err := os.Remove(filepath.Join(con, "docs", "guide.md")); err != nil {
		t.Fatal(err)
	}
	chmodOff(t, filepath.Join(con, "tools", "check"))
	so, _, code := vk(t, con, nil, false, "gate", "--strict")
	if code != 1 {
		t.Fatalf("strict gate exit = %d, want 1", code)
	}
	mustContain(t, so, "removed: docs/guide.md")
	mustContain(t, so, "changed: tools/check")
	mustContain(t, so, "exec bit")
}

func TestCRLFRecheckoutDoesNotTripGate(t *testing.T) {
	_, con := world(t)
	f := filepath.Join(con, "docs", "standard.md")
	writeBytes(t, f, bytes.ReplaceAll(readBytes(t, f), []byte("\n"), []byte("\r\n")))
	vk(t, con, nil, true, "gate", "--strict")
}

func TestCollisionBetweenSlicesDetected(t *testing.T) {
	_, con := world(t)
	manifest := loadJSON(t, filepath.Join(con, ".vendkit", "docs-manifest.json"))
	rogue := map[string]any{}
	for k, v := range manifest {
		rogue[k] = v
	}
	rogue["slice"] = "rogue"
	rogue["entries"] = []any{entriesOf(manifest)[0]}
	writeJSON(t, filepath.Join(con, ".vendkit", "rogue-manifest.json"), rogue)
	so, _, code := vk(t, con, nil, false, "gate", "--strict")
	if code != 1 {
		t.Fatalf("strict gate exit = %d, want 1", code)
	}
	mustContain(t, so, "collision")
}

// -- sync lane --------------------------------------------------------------------

func TestSyncSameVersionIsNoop(t *testing.T) {
	pub, con := world(t)
	so, _, _ := vk(t, con, nil, true, "sync-pipeline", "--slice", "docs",
		"--publisher-root", pub, "--consumer-root", con)
	mustContain(t, so, "update-available=false")
}

func TestSyncUpgradeComposesWithGate(t *testing.T) {
	// The composition invariant INV-1 plus PR mechanics end-to-end.
	pub, con := world(t)
	write(t, filepath.Join(pub, "docs", "standard.md"),
		"# Standard\n\nRule one.\nRule two.\n")
	write(t, filepath.Join(pub, "docs", "new.md"), "# New\n")
	release(t, pub, "v0.2.0")

	journal := filepath.Join(t.TempDir(), "journal.jsonl")
	so, _, _ := vk(t, con, map[string]string{"VENDKIT_NEUTRAL_JOURNAL": journal},
		true, "sync-pipeline", "--slice", "docs",
		"--publisher-root", pub, "--consumer-root", con)
	mustContain(t, so, "update-available=true")
	mustContain(t, so, "changed=true")

	// One PR intent, deterministic branch, via the PR handler.
	prs := recordsOfKind(journalRecords(t, journal), "pr")
	if len(prs) != 1 {
		t.Fatalf("journal PR intents = %d, want 1", len(prs))
	}
	if got := toStr(prs[0]["head_branch"]); got != "vendkit/docs/sync-v0.1.0-to-v0.2.0" {
		t.Errorf("head_branch = %q", got)
	}
	if prs[0]["vendkit_handler_protocol"] != float64(1) {
		t.Errorf("vendkit_handler_protocol = %v, want 1", prs[0]["vendkit_handler_protocol"])
	}

	// Content refreshed, addition reconciled, pins advanced in lockstep.
	mustContain(t, read(t, filepath.Join(con, "docs", "standard.md")), "Rule two.")
	if !exists(filepath.Join(con, "docs", "new.md")) {
		t.Error("docs/new.md not reconciled into scope")
	}
	for _, wf := range []string{"docs-sync", "vendkit-gate", "vendkit-watch"} {
		text := read(t, filepath.Join(con, ".github", "workflows", wf+".yml"))
		mustContain(t, text, "refs/tags/v0.2.0")
	}

	// Provenance recorded (manifest spec §1).
	manifest := loadJSON(t, filepath.Join(con, ".vendkit", "docs-manifest.json"))
	source := asMap(manifest["source"])
	if toStr(source["release"]) != "v0.2.0" {
		t.Errorf("source.release = %q, want v0.2.0", source["release"])
	}
	if len(toStr(source["commit"])) != 40 {
		t.Errorf("source.commit length = %d, want 40", len(toStr(source["commit"])))
	}

	// INV-1: the sync output passes the strict gate.
	vk(t, con, nil, true, "gate", "--strict")

	// Idempotency: re-running the pipeline finds nothing to do.
	again, _, _ := vk(t, con, nil, true, "sync-pipeline", "--slice", "docs",
		"--publisher-root", pub, "--consumer-root", con)
	mustContain(t, again, "changed=false")
}

func TestRetractedTargetRefused(t *testing.T) {
	pub, con := world(t)
	write(t, filepath.Join(pub, "docs", "guide.md"), "# Guide v2\n")
	release(t, pub, "v0.2.0")
	appendText(t, filepath.Join(pub, "vendkit-export.yml"), "retracted: [v0.2.0]\n")
	release(t, pub, "v0.2.1")
	// Publisher checkout sits at v0.2.0 (the retracted one): refuse, exit 3.
	git(t, pub, "checkout", "-q", "v0.2.0")
	so, _, code := vk(t, con, nil, false, "sync-pipeline", "--slice", "docs",
		"--publisher-root", pub, "--consumer-root", con)
	if code != 3 {
		t.Fatalf("exit = %d, want 3\n%s", code, so)
	}
	mustContain(t, so, "refused=retracted")
}

// -- releases ----------------------------------------------------------------------

func TestReleaseRefusesStaleManifest(t *testing.T) {
	pub, _ := world(t)
	write(t, filepath.Join(pub, "docs", "guide.md"), "# Guide changed\n")
	so, _, code := vk(t, pub, nil, false, "release", "--version", "v0.2.0")
	if code != 3 {
		t.Fatalf("exit = %d, want 3\n%s", code, so)
	}
	mustContain(t, so, "refused=stale-manifest")
}

func TestReleaseBumpAndMigrationGates(t *testing.T) {
	pub, _ := world(t)
	if err := os.Remove(filepath.Join(pub, "docs", "guide.md")); err != nil { // surface removal
		t.Fatal(err)
	}
	vk(t, pub, nil, true, "generate")
	git(t, pub, "add", "-A")
	git(t, pub, "commit", "-q", "-m", "drop guide")
	// Removal with only a patch bump: refused.
	so, _, _ := vk(t, pub, nil, false, "release", "--version", "v0.1.1")
	mustContain(t, so, "refused=bump-too-small")
	// Major bump but no migration payload: refused.
	so, _, _ = vk(t, pub, nil, false, "release", "--version", "v1.0.0")
	mustContain(t, so, "refused=migration-missing")
	// With a payload: allowed.
	write(t, filepath.Join(pub, "migrations", "drop-guide.yml"), `schema_version: 1
id: drop-guide
applies_from: v1.0.0
kind: structural
profiles: ["*"]
summary: "guide.md retired; fold content into standard.md"
rationale: "One doc is enough."
detection: [{glob: "docs/guide.md"}]
instructions: "Merge guide content into standard.md, delete guide.md."
verification:
  must_be_absent: ["docs/guide.md"]
  must_be_present: ["docs/standard.md"]
`)
	git(t, pub, "add", "-A")
	git(t, pub, "commit", "-q", "-m", "migration payload")
	vk(t, pub, nil, true, "release", "--version", "v1.0.0")
}

func TestReleaseTagExistsRefused(t *testing.T) {
	pub, _ := world(t)
	so, _, code := vk(t, pub, nil, false, "release", "--version", "v0.1.0")
	if code != 3 {
		t.Fatalf("exit = %d, want 3\n%s", code, so)
	}
	mustContain(t, so, "refused=")
}

// -- migrations -----------------------------------------------------------------------

func TestMigrationWindowAndVerify(t *testing.T) {
	pub, con := world(t)
	write(t, filepath.Join(pub, "migrations", "drop-guide.yml"), `schema_version: 1
id: drop-guide
applies_from: v0.2.0
kind: structural
profiles: ["*"]
summary: "guide retired"
rationale: "consolidation"
verification:
  must_be_absent: ["docs/guide.md"]
`)
	so, _, _ := vk(t, pub, nil, true, "migrations", "--pinned", "v0.1.0",
		"--target", "v0.2.0", "--publisher-root", pub)
	doc := parseDocFrom(t, so)
	applicable, _ := doc["applicable"].([]any)
	if len(applicable) != 1 || toStr(asMap(applicable[0])["id"]) != "drop-guide" {
		t.Fatalf("applicable = %v, want [drop-guide]", applicable)
	}
	// Outside the window: nothing applies.
	so, _, _ = vk(t, pub, nil, true, "migrations", "--pinned", "v0.2.0",
		"--target", "v0.3.0", "--publisher-root", pub)
	mustContain(t, so, "count=0")

	obligations, err := json.Marshal(doc["obligations"])
	if err != nil {
		t.Fatal(err)
	}
	_, _, code := vk(t, con, nil, false, "migrations-verify", "--obligations",
		string(obligations), "--consumer-root", con)
	if code != 1 { // guide.md still present
		t.Fatalf("migrations-verify exit = %d, want 1", code)
	}
	if err := os.Remove(filepath.Join(con, "docs", "guide.md")); err != nil {
		t.Fatal(err)
	}
	git(t, con, "add", "-A")
	git(t, con, "commit", "-q", "-m", "apply migration")
	vk(t, con, nil, true, "migrations-verify", "--obligations",
		string(obligations), "--consumer-root", con)
	// Zero obligations: green no-op (safe as an always-on required check).
	vk(t, con, nil, true, "migrations-verify", "--obligations", "{}",
		"--consumer-root", con)
}

// -- watch --------------------------------------------------------------------------

func TestWatchDetectsUpdateAndDryRunIsOffline(t *testing.T) {
	pub, con := world(t)
	write(t, filepath.Join(pub, "docs", "guide.md"), "# Guide v2\n")
	release(t, pub, "v0.2.0")
	journal := filepath.Join(t.TempDir(), "journal.jsonl")
	so, _, _ := vk(t, con, map[string]string{"VENDKIT_NEUTRAL_JOURNAL": journal},
		true, "watch", "--no-handoff", "--json")
	mustContain(t, so, "findings=1")
	findings := parseArrayFrom(t, so)
	f0 := asMap(findings[0])
	if toStr(f0["kind"]) != "update-available" {
		t.Errorf("finding kind = %q", f0["kind"])
	}
	if toStr(f0["latest"]) != "v0.2.0" {
		t.Errorf("finding latest = %q", f0["latest"])
	}
	if toStr(f0["bump"]) != "minor" {
		t.Errorf("finding bump = %q", f0["bump"])
	}
	// Handoff hands exactly one deduped intent to the handler.
	vk(t, con, map[string]string{"VENDKIT_NEUTRAL_JOURNAL": journal}, true, "watch")
	handoffs := recordsOfKind(journalRecords(t, journal), "handoff")
	if len(handoffs) != 1 || toStr(handoffs[0]["dedup_key"]) != "vendkit-watch-docs" {
		t.Fatalf("handoffs = %v, want one with dedup_key vendkit-watch-docs", handoffs)
	}
	// Dry-run: no findings, exit 0 (PR self-test, no network).
	dry, _, _ := vk(t, con, nil, true, "watch", "--dry-run")
	mustContain(t, dry, "findings=0")
}

func TestWatchDetectsTagMoved(t *testing.T) {
	pub, con := world(t)
	// Simulate tag substitution: delete and re-point v0.1.0.
	write(t, filepath.Join(pub, "docs", "guide.md"), "# tampered\n")
	vk(t, pub, nil, true, "generate")
	git(t, pub, "add", "-A")
	git(t, pub, "commit", "-q", "-m", "tamper")
	git(t, pub, "tag", "-f", "-a", "v0.1.0", "-m", "moved")
	git(t, pub, "push", "-q", "--force", "origin", "refs/tags/v0.1.0")
	so, _, _ := vk(t, con, nil, true, "watch", "--no-handoff", "--json")
	findings := parseArrayFrom(t, so)
	found := false
	for _, f := range findings {
		if toStr(asMap(f)["kind"]) == "tag-moved" {
			found = true
		}
	}
	if !found {
		t.Fatalf("no tag-moved finding in %s", so)
	}
}

// -- conformance ----------------------------------------------------------------------

func TestConformanceReportsAndAttestations(t *testing.T) {
	_, con := world(t)
	so, _, _ := vk(t, con, nil, true, "conformance", "--slice", "docs")
	lines := strings.Split(so, "\n")
	hasPass := func(rule string) bool {
		for _, l := range lines {
			if strings.HasPrefix(l, "pass") && strings.Contains(l, rule) {
				return true
			}
		}
		return false
	}
	hasFail := func(rule string) bool {
		for _, l := range lines {
			if strings.HasPrefix(l, "fail") && strings.Contains(l, rule) {
				return true
			}
		}
		return false
	}
	if !hasPass("manifest-committed") {
		t.Error("expected pass manifest-committed")
	}
	if !hasPass("control-plane-owned") {
		t.Error("expected pass control-plane-owned")
	}
	if !hasFail("branch-protected") {
		t.Error("expected fail branch-protected")
	}
	_, _, code := vk(t, con, nil, false, "conformance", "--slice", "docs", "--strict")
	if code != 1 {
		t.Fatalf("strict conformance exit = %d, want 1", code)
	}
	// Attest + record the non-tree-decidable facts: strict goes green.
	cfgPath := filepath.Join(con, ".vendkit", "docs.yml")
	cfg := read(t, cfgPath)
	cfg = strings.ReplaceAll(cfg, "branch_protection_enabled: false",
		"branch_protection_enabled: true")
	cfg = strings.ReplaceAll(cfg, "sync_credential_provisioned: false",
		"sync_credential_provisioned: true")
	cfg = strings.ReplaceAll(cfg, "attestations:",
		"attestations:\n  required_check_enforced: true")
	write(t, cfgPath, cfg)
	vk(t, con, nil, true, "conformance", "--slice", "docs", "--strict")
}

// -- seeded files (DR-0013) -----------------------------------------------------------

func TestSeedScaffoldedOnceAndFreeToDiverge(t *testing.T) {
	_, con := world(t)
	seeded := filepath.Join(con, "templates", "CONTRIBUTING.md")
	mustContain(t, read(t, seeded), "Starter text") // onboard seeded it
	manifest := loadJSON(t, filepath.Join(con, ".vendkit", "docs-manifest.json"))
	var entry map[string]any
	for _, e := range entriesOf(manifest) {
		if toStr(asMap(e)["path"]) == "templates/CONTRIBUTING.md" {
			entry = asMap(e)
		}
	}
	if entry == nil || entry["seed"] != true {
		t.Fatalf("templates/CONTRIBUTING.md entry seed flag = %v", entry)
	}
	write(t, seeded, "# Contributing\n\nOur own rules.\n") // diverge
	vk(t, con, nil, true, "gate", "--strict")              // gate never checks seeds
}

func TestSeedAdoptsPreexistingFileWithoutClobbering(t *testing.T) {
	pub, _ := world(t)
	con2 := filepath.Join(t.TempDir(), "consumer2")
	own := "# Contributing\n\nPredates the slice; must survive onboarding.\n"
	write(t, filepath.Join(con2, "templates", "CONTRIBUTING.md"), own)
	vk(t, pub, nil, true, "onboard", "--ci", "github-actions", "--scm", "github",
		"--version", "v0.1.0", "--profile", "code-repo",
		"--publisher-root", pub, "--consumer-root", con2)
	if read(t, filepath.Join(con2, "templates", "CONTRIBUTING.md")) != own {
		t.Error("pre-existing seed file was clobbered")
	}
	manifest := loadJSON(t, filepath.Join(con2, ".vendkit", "docs-manifest.json"))
	found := false
	for _, e := range entriesOf(manifest) {
		em := asMap(e)
		if toStr(em["path"]) == "templates/CONTRIBUTING.md" && em["seed"] == true {
			found = true
		}
	}
	if !found {
		t.Error("adopted file not recorded as a seed entry")
	}
}

func TestTemplateUpdateNeverTouchesCopyAndNotesInPR(t *testing.T) {
	pub, con := world(t)
	ours := "# Contributing\n\nHeavily customised.\n"
	write(t, filepath.Join(con, "templates", "CONTRIBUTING.md"), ours)
	git(t, con, "add", "-A")
	git(t, con, "commit", "-q", "-m", "customise seed")
	// Upstream: template improves AND a vendored file changes (a template-only
	// change deliberately never forces a PR on its own).
	write(t, filepath.Join(pub, "templates", "CONTRIBUTING.md"), "# Contributing v2\n")
	write(t, filepath.Join(pub, "docs", "standard.md"),
		"# Standard\n\nRule one.\nRule two.\n")
	release(t, pub, "v0.2.0")
	journal := filepath.Join(t.TempDir(), "journal.jsonl")
	vk(t, con, map[string]string{"VENDKIT_NEUTRAL_JOURNAL": journal}, true,
		"sync-pipeline", "--slice", "docs", "--publisher-root", pub,
		"--consumer-root", con)
	if read(t, filepath.Join(con, "templates", "CONTRIBUTING.md")) != ours {
		t.Error("seeded copy was modified by a template update")
	}
	prs := recordsOfKind(journalRecords(t, journal), "pr")
	if len(prs) == 0 {
		t.Fatal("no PR intent recorded")
	}
	body := toStr(prs[0]["body_md"])
	mustContain(t, body, "upstream template changed")
	mustContain(t, body, "templates/CONTRIBUTING.md")
	vk(t, con, nil, true, "gate", "--strict") // INV-1 still holds
}

func TestSeedDeletionRespectedWithEscapeHatch(t *testing.T) {
	pub, con := world(t)
	if err := os.Remove(filepath.Join(con, "templates", "CONTRIBUTING.md")); err != nil {
		t.Fatal(err)
	}
	git(t, con, "add", "-A")
	git(t, con, "commit", "-q", "-m", "we do not want this template")
	vk(t, con, nil, true, "gate", "--strict") // deletion is not drift
	write(t, filepath.Join(pub, "docs", "guide.md"), "# Guide v2\n")
	release(t, pub, "v0.2.0")
	vk(t, con, nil, true, "sync-pipeline", "--slice", "docs",
		"--publisher-root", pub, "--consumer-root", con)
	if exists(filepath.Join(con, "templates", "CONTRIBUTING.md")) {
		t.Error("deleted seed was re-seeded")
	}
	// Escape hatch: drop the entry, reconcile re-offers the seed.
	mpath := filepath.Join(con, ".vendkit", "docs-manifest.json")
	manifest := loadJSON(t, mpath)
	var kept []any
	for _, e := range entriesOf(manifest) {
		if toStr(asMap(e)["path"]) != "templates/CONTRIBUTING.md" {
			kept = append(kept, e)
		}
	}
	manifest["entries"] = kept
	writeJSON(t, mpath, manifest)
	so, _, _ := vk(t, con, nil, true, "sync", "--apply", "--reconcile-scope",
		"--target", "v0.2.0", "--publisher-root", pub, "--consumer-root", con)
	mustContain(t, so, "seeded: templates/CONTRIBUTING.md")
	if !exists(filepath.Join(con, "templates", "CONTRIBUTING.md")) {
		t.Error("escape hatch did not re-seed the file")
	}
}

func TestSeedPathStillClaimsINV7Collision(t *testing.T) {
	_, con := world(t)
	manifest := loadJSON(t, filepath.Join(con, ".vendkit", "docs-manifest.json"))
	var seedEntry any
	for _, e := range entriesOf(manifest) {
		if asMap(e)["seed"] == true {
			seedEntry = e
			break
		}
	}
	if seedEntry == nil {
		t.Fatal("no seed entry found")
	}
	rogue := map[string]any{}
	for k, v := range manifest {
		rogue[k] = v
	}
	rogue["slice"] = "rogue"
	rogue["entries"] = []any{seedEntry}
	writeJSON(t, filepath.Join(con, ".vendkit", "rogue-manifest.json"), rogue)
	so, _, code := vk(t, con, nil, false, "gate", "--strict")
	if code != 1 {
		t.Fatalf("exit = %d, want 1", code)
	}
	mustContain(t, so, "collision")
}

func TestSeedRetirementIsPatchGrade(t *testing.T) {
	pub, con := world(t)
	if err := os.Remove(filepath.Join(pub, "templates", "CONTRIBUTING.md")); err != nil {
		t.Fatal(err)
	}
	vk(t, pub, nil, true, "generate")
	git(t, pub, "add", "-A")
	git(t, pub, "commit", "-q", "-m", "retire template")
	git(t, pub, "push", "-q", "origin", "main")
	// A seed removal demands neither MAJOR nor a migration payload (DR-0013).
	vk(t, pub, nil, true, "release", "--version", "v0.1.1")
	so, _, _ := vk(t, con, nil, true, "sync-pipeline", "--slice", "docs",
		"--publisher-root", pub, "--consumer-root", con)
	mustContain(t, so, "changed=true")
	if !exists(filepath.Join(con, "templates", "CONTRIBUTING.md")) {
		t.Error("consumer copy was deleted") // copy untouched
	}
	manifest := loadJSON(t, filepath.Join(con, ".vendkit", "docs-manifest.json"))
	for _, e := range entriesOf(manifest) {
		if toStr(asMap(e)["path"]) == "templates/CONTRIBUTING.md" {
			t.Error("retired template still tracked")
		}
	}
	vk(t, con, nil, true, "gate", "--strict")
}

// -- handler protocol / axes (DR-0014, DR-0015) --------------------------------------

func TestCINoneIsFullyManual(t *testing.T) {
	// ci: none — no pipelines, provenance is the pin, PR delivery unwired:
	// sync stops at 'branch pushed, intent emitted' and the human takes over.
	pub, _ := world(t)
	tmp := t.TempDir()
	con := filepath.Join(tmp, "manual-consumer")
	mkdirAll(t, con)
	git(t, con, "init", "-q", "-b", "main")
	vk(t, pub, nil, true, "init", "--ci", "none", "--scm", "github",
		"--version", "v0.1.0", "--profile", "code-repo",
		"--publisher-root", pub, "--consumer-root", con)
	if exists(filepath.Join(con, ".github")) {
		t.Error("ci: none scaffolded pipelines")
	}
	cfgPath := filepath.Join(con, ".vendkit", "docs.yml")
	cfg := read(t, cfgPath)
	mustContain(t, cfg, "ci: none")
	write(t, cfgPath, strings.ReplaceAll(cfg, "repo: example-org/pub", "repo: "+pub))
	git(t, con, "add", "-A")
	git(t, con, "commit", "-q", "-m", "manual onboard")
	origin := filepath.Join(tmp, "manual-origin.git")
	git(t, tmp, "init", "-q", "--bare", origin)
	git(t, con, "remote", "add", "origin", origin)
	git(t, con, "push", "-q", "origin", "main")

	vk(t, con, nil, true, "gate", "--strict") // the lanes still run by hand
	so, _, _ := vk(t, con, nil, true, "conformance", "--slice", "docs")
	mustContain(t, so, "skipped")
	mustContain(t, so, "gate-wired")
	so, _, _ = vk(t, con, nil, true, "watch", "--no-handoff") // pin read from provenance
	mustContain(t, so, "findings=0")

	write(t, filepath.Join(pub, "docs", "guide.md"), "# Guide v2\n")
	release(t, pub, "v0.2.0")
	git(t, pub, "checkout", "-q", "v0.2.0")
	so, _, _ = vk(t, con, nil, true, "sync-pipeline", "--slice", "docs",
		"--publisher-root", pub, "--consumer-root", con)
	git(t, pub, "checkout", "-q", "main")
	mustContain(t, so, "pr-delivered=false") // unwired: intent emitted
	var intentLine string
	for _, l := range strings.Split(so, "\n") {
		if strings.HasPrefix(l, "pr-intent=") {
			intentLine = strings.TrimPrefix(l, "pr-intent=")
		}
	}
	if intentLine == "" {
		t.Fatalf("no pr-intent line in:\n%s", so)
	}
	var intent map[string]any
	if err := json.Unmarshal([]byte(intentLine), &intent); err != nil {
		t.Fatalf("parse pr-intent: %v", err)
	}
	if toStr(intent["head_branch"]) != "vendkit/docs/sync-v0.1.0-to-v0.2.0" {
		t.Errorf("intent head_branch = %q", intent["head_branch"])
	}
	branches := gitOut(t, tmp, "ls-remote", "--heads", origin)
	mustContain(t, branches, "vendkit/docs/sync-v0.1.0-to-v0.2.0") // pushed for review
}

func TestStrayVendkitYAMLIsLoud(t *testing.T) {
	// The .vendkit/ namespace is strict: a YAML file that is not a slice
	// config is a usage error, never a silent skip (DR-0012).
	_, con := world(t)
	write(t, filepath.Join(con, ".vendkit", "notes.yml"), "just: notes\n")
	_, _, code := vk(t, con, nil, false, "watch", "--dry-run")
	if code != 2 {
		t.Fatalf("exit = %d, want 2", code)
	}
}

func TestInitInfersSCMFromRemote(t *testing.T) {
	pub, _ := world(t)
	tmp := t.TempDir()
	con := filepath.Join(tmp, "inferred-consumer")
	mkdirAll(t, con)
	git(t, con, "init", "-q", "-b", "main")
	git(t, con, "remote", "add", "origin",
		"https://github.com/example-org/thing.git")
	vk(t, pub, nil, true, "init", "--ci", "github-actions", "--version", "v0.1.0",
		"--profile", "code-repo", "--publisher-root", pub,
		"--consumer-root", con) // no --scm: inferred
	mustContain(t, read(t, filepath.Join(con, ".vendkit", "docs.yml")), "scm: github")
	// No remote and no --scm: loud usage error, never a guess.
	con2 := filepath.Join(tmp, "no-remote-consumer")
	mkdirAll(t, con2)
	git(t, con2, "init", "-q", "-b", "main")
	_, se, code := vk(t, pub, nil, false, "init", "--ci", "github-actions",
		"--version", "v0.1.0", "--publisher-root", pub, "--consumer-root", con2)
	if code != 2 {
		t.Fatalf("exit = %d, want 2", code)
	}
	mustContain(t, se, "--scm")
}

func TestCodeownersIsOptInAndGithubOnly(t *testing.T) {
	pub, con := world(t)
	// The world consumer wrote its own CODEOWNERS; init added no stanza.
	mustContain(t, read(t, filepath.Join(con, "CODEOWNERS")), "@example-org/owners")
	tmp := t.TempDir()
	co := filepath.Join(tmp, "codeowners-consumer")
	mkdirAll(t, co)
	git(t, co, "init", "-q", "-b", "main")
	vk(t, pub, nil, true, "init", "--ci", "github-actions", "--scm", "github",
		"--version", "v0.1.0", "--profile", "code-repo",
		"--codeowners", "@example-org/platform",
		"--publisher-root", pub, "--consumer-root", co)
	mustContain(t, read(t, filepath.Join(co, "CODEOWNERS")),
		"/.vendkit/ @example-org/platform")
	// Azure Repos does not honour CODEOWNERS: refuse rather than scaffold a
	// dead file (points at the required-reviewers policy instead).
	ado := filepath.Join(tmp, "ado-consumer")
	mkdirAll(t, ado)
	git(t, ado, "init", "-q", "-b", "main")
	_, se, code := vk(t, pub, nil, false, "init", "--ci", "azure-pipelines",
		"--scm", "azure-repos", "--version", "v0.1.0", "--codeowners", "@x",
		"--publisher-root", pub, "--consumer-root", ado)
	if code != 2 {
		t.Fatalf("exit = %d, want 2", code)
	}
	mustContain(t, se, "required-reviewers")
}

// -- human tier -----------------------------------------------------------------

func TestStatusReportsPinLatestAndDrift(t *testing.T) {
	pub, con := world(t)
	so, _, _ := vk(t, con, nil, true, "status")
	mustContain(t, so, "docs")
	mustContain(t, so, "v0.1.0")
	mustContain(t, so, "up to date")
	mustContain(t, so, "clean")
	write(t, filepath.Join(pub, "docs", "guide.md"), "# Guide v2\n")
	release(t, pub, "v0.2.0")
	appendText(t, filepath.Join(con, "docs", "standard.md"), "sneaky edit\n")
	so, _, _ = vk(t, con, nil, true, "status")
	mustContain(t, so, "UPDATE AVAILABLE (minor)")
	mustContain(t, so, "DRIFT: 1 finding(s)")
}

func TestDiffPreviewsWithoutTouchingTheTree(t *testing.T) {
	pub, con := world(t)
	write(t, filepath.Join(pub, "docs", "standard.md"),
		"# Standard\n\nRule one.\nRule two.\n")
	release(t, pub, "v0.2.0")
	before := read(t, filepath.Join(con, "docs", "standard.md"))
	so, _, _ := vk(t, con, nil, true, "diff")
	mustContain(t, so, "v0.1.0 → v0.2.0")
	mustContain(t, so, "+Rule two.")
	if read(t, filepath.Join(con, "docs", "standard.md")) != before {
		t.Error("diff mutated the working tree")
	}
}

func TestUpdateLocalAppliesAndComposesWithGate(t *testing.T) {
	pub, con := world(t)
	write(t, filepath.Join(pub, "docs", "standard.md"),
		"# Standard\n\nRule one.\nRule two.\n")
	release(t, pub, "v0.2.0")
	so, _, _ := vk(t, con, nil, true, "update")
	mustContain(t, so, "applied to the working tree")
	mustContain(t, read(t, filepath.Join(con, "docs", "standard.md")), "Rule two.")
	manifest := loadJSON(t, filepath.Join(con, ".vendkit", "docs-manifest.json"))
	if toStr(asMap(manifest["source"])["release"]) != "v0.2.0" {
		t.Error("manifest pin not advanced")
	}
	mustContain(t, read(t, filepath.Join(con, ".github", "workflows", "docs-sync.yml")),
		"refs/tags/v0.2.0") // pins advanced
	vk(t, con, nil, true, "gate", "--strict") // INV-1 via the human path
	again, _, _ := vk(t, con, nil, true, "update")
	mustContain(t, again, "already at v0.2.0")
}

func TestUpdatePRIsACompositionOverTheSyncLane(t *testing.T) {
	pub, con := world(t)
	write(t, filepath.Join(pub, "docs", "guide.md"), "# Guide v2\n")
	release(t, pub, "v0.2.0")
	journal := filepath.Join(t.TempDir(), "journal.jsonl")
	so, _, _ := vk(t, con, map[string]string{"VENDKIT_NEUTRAL_JOURNAL": journal},
		true, "update", "--pr")
	mustContain(t, so, "pr-delivered=true")
	prs := recordsOfKind(journalRecords(t, journal), "pr")
	if len(prs) == 0 {
		t.Fatal("no PR intent recorded")
	}
	if toStr(prs[0]["head_branch"]) != "vendkit/docs/sync-v0.1.0-to-v0.2.0" {
		t.Errorf("head_branch = %q", prs[0]["head_branch"])
	}
}

func TestExplainRegistry(t *testing.T) {
	_, con := world(t)
	so, _, _ := vk(t, con, nil, true, "explain", "tag-moved")
	mustContain(t, so, "tampering")
	so, _, _ = vk(t, con, nil, true, "explain", "list")
	mustContain(t, so, "retracted")
	_, _, code := vk(t, con, nil, false, "explain", "nonsense")
	if code != 2 {
		t.Fatalf("exit = %d, want 2", code)
	}
}

func TestInitNoninteractiveRequiresCIAndVersion(t *testing.T) {
	pub, _ := world(t)
	con := filepath.Join(t.TempDir(), "prompt-consumer")
	mkdirAll(t, con)
	git(t, con, "init", "-q", "-b", "main")
	_, se, code := vk(t, pub, nil, false, "init", "--scm", "github",
		"--version", "v0.1.0", "--publisher-root", pub, "--consumer-root", con)
	if code != 2 {
		t.Fatalf("exit = %d, want 2", code)
	}
	mustContain(t, se, "--ci is required")
}

// TestGatePathIsStandaloneOnly is the Go analogue of the Python INV-9 test
// (the gate must run with PyYAML unimportable). The Go binary is a single
// static executable; the gate reads only the manifest and the working tree
// and shells out to nothing. We exercise that by running `gate --strict` with
// PATH emptied — no external tool (not even git) is reachable — and asserting
// it still passes.
func TestGatePathIsStandaloneOnly(t *testing.T) {
	_, con := world(t)
	_, se, code := vk(t, con, map[string]string{"PATH": ""}, false, "gate", "--strict")
	if code != 0 {
		t.Fatalf("gate --strict exit = %d, want 0\n%s", code, se)
	}
}
