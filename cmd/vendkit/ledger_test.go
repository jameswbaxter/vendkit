// Behavioural-differences ledger audit (M4). The ledger is the Markdown table
// in docs/specs/platform-integration.md §6. The project's parity rule
// (docs/testing.md) and INV-8 (docs/architecture.md) demand a *bidirectional*
// correspondence between that table and the Layer-1 code:
//
//   1. the numbering is contiguous (no gaps / dupes / reorders);
//   2. every mitigation's named cross-reference resolves (a real DR file, a real
//      spec-section heading);
//   3. ledger → code: every entry names a durable code marker that proves the
//      difference still exists (ledgerAnchors);
//   4. code → ledger: every per-platform fork SITE in the scanned Layer-1 files
//      is covered by an explicit allowlist that maps it to a ledger entry or
//      justifies it as a pure "dialect, not semantic" divergence (forkAllowlist).
//
// This test is the enforcement the roadmap owed: it fails loudly when a ledger
// entry loses its code anchor, when a cross-reference rots, or when a new
// platform branch lands with no ledger entry and no allowlist justification.
// It reads the canonical docs/ and internal/ sources at test time (located via
// runtime.Caller), so it tracks the tree it is compiled from.

package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// repoRoot returns the repository root, three directories above this test file
// (cmd/vendkit/ledger_test.go). Using runtime.Caller keeps the paths correct
// regardless of the test's working directory.
func repoRoot(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Dir(filepath.Dir(filepath.Dir(thisFile)))
}

type ledgerEntry struct {
	Num                    int
	Difference, Mitigation string
}

// parseLedger reads §6 ("Behavioural differences ledger") of the
// platform-integration spec and parses the numbered rows out of the Markdown
// table. It stops at the next `## ` heading (or EOF), and skips the header /
// separator rows (they carry no leading integer).
func parseLedger(t *testing.T, root string) []ledgerEntry {
	t.Helper()
	path := filepath.Join(root, "docs", "specs", "platform-integration.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	lines := strings.Split(string(data), "\n")
	start := -1
	for i, ln := range lines {
		if strings.HasPrefix(ln, "## ") && strings.Contains(ln, "Behavioural differences ledger") {
			start = i
			break
		}
	}
	if start < 0 {
		t.Fatal("could not find the '## 6. Behavioural differences ledger' section")
	}
	rowRx := regexp.MustCompile(`^\|\s*(\d+)\s*\|(.+?)\|(.+?)\|\s*$`)
	var entries []ledgerEntry
	for i := start + 1; i < len(lines); i++ {
		if strings.HasPrefix(lines[i], "## ") {
			break // next section
		}
		m := rowRx.FindStringSubmatch(lines[i])
		if m == nil {
			continue
		}
		num, _ := strconv.Atoi(m[1])
		entries = append(entries, ledgerEntry{
			Num:        num,
			Difference: strings.TrimSpace(m[2]),
			Mitigation: strings.TrimSpace(m[3]),
		})
	}
	return entries
}

// TestLedgerNumberingContiguous asserts the ledger is numbered 1..N with no
// gap, duplicate, or reorder — the numbers are load-bearing (entries are cited
// as "difference #N" across the docs and in this test's anchor map).
func TestLedgerNumberingContiguous(t *testing.T) {
	entries := parseLedger(t, repoRoot(t))
	if len(entries) == 0 {
		t.Fatal("parsed zero ledger entries — the table shape changed")
	}
	for i, e := range entries {
		if e.Num != i+1 {
			t.Fatalf("ledger numbering broken at row %d: got #%d, want #%d "+
				"(gap, duplicate, or reorder)", i+1, e.Num, i+1)
		}
	}
}

// headingExists reports whether the file at path has a Markdown heading whose
// text begins with "<sec>." — e.g. sec="3" matches "## 3. Attestations".
func headingExists(t *testing.T, path, sec string) bool {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	rx := regexp.MustCompile(`(?m)^#{1,4}\s+` + regexp.QuoteMeta(sec) + `\.`)
	return rx.Match(data)
}

// TestLedgerCrossReferencesResolve checks that each mitigation's named
// cross-references actually resolve. Two reference shapes are recognised:
//
//   - DR references ("DR-0015") → a docs/design/DR-NNNN-*.md file must exist;
//   - section references ("conformance spec §3", "security model §2",
//     "testing.md §3", or a bare local "(§4)") → the target spec must carry a
//     matching "## N." heading. A bare §N with no doc qualifier resolves against
//     this same spec (platform-integration.md).
//
// The resolver is pragmatic but real: it dereferences to files and headings, so
// a citation to a section that does not exist fails the test.
func TestLedgerCrossReferencesResolve(t *testing.T) {
	root := repoRoot(t)
	entries := parseLedger(t, root)

	drRx := regexp.MustCompile(`DR-(\d{4})`)
	// Optional doc qualifier immediately preceding "§N"; a bare "(§N)" resolves
	// against platform-integration.md itself.
	secRx := regexp.MustCompile(`(?:(conformance spec|security model|testing\.md)\s+)?\(?§\s*(\d+)`)

	docFor := map[string]string{
		"conformance spec": "docs/specs/conformance.md",
		"security model":   "docs/specs/security-model.md",
		"testing.md":       "docs/testing.md",
		"":                 "docs/specs/platform-integration.md",
	}

	for _, e := range entries {
		text := e.Mitigation
		for _, m := range drRx.FindAllStringSubmatch(text, -1) {
			glob := filepath.Join(root, "docs", "design", "DR-"+m[1]+"-*.md")
			if hits, _ := filepath.Glob(glob); len(hits) == 0 {
				t.Errorf("entry #%d cites DR-%s but no design record matches %s",
					e.Num, m[1], glob)
			}
		}
		for _, m := range secRx.FindAllStringSubmatch(text, -1) {
			qualifier, sec := m[1], m[2]
			file, ok := docFor[qualifier]
			if !ok {
				t.Errorf("entry #%d: unhandled reference qualifier %q", e.Num, qualifier)
				continue
			}
			if !headingExists(t, filepath.Join(root, file), sec) {
				t.Errorf("entry #%d cites %s §%s but that section heading is absent",
					e.Num, file, sec)
			}
		}
	}
}

// ledgerAnchors maps each ledger entry number to a durable code marker that
// proves the difference still exists in Layer-1 code. Keep it in lockstep with
// docs/specs/platform-integration.md §6: adding a ledger entry means adding its
// anchor here (and removing one means removing its anchor). Each marker is a
// stable string literal or identifier already present in the source — chosen to
// be grep-durable, not tied to a line number.
var ledgerAnchors = map[int]struct {
	file, marker, note string
}{
	1: {"internal/core/conformance.go", "Azure Repos PR gating is a branch policy",
		"hasEvent() returns -1 (not tree-decidable) for Azure Repos PR gating → attest path"},
	2: {"cmd/vendkit/handler.go", "GITHUB_TOKEN-opened PRs do ",
		"githubPR() refuses the GITHUB_TOKEN fallback (also covered by TestGithubPR_RefusesGithubTokenFallback)"},
	3: {"cmd/vendkit/handler.go", "ado-pull-trigger",
		"adoPushHint() is a no-op skip; the GHA side dispatches repository_dispatch (githubPushHint)"},
	4: {"internal/core/watch.go", "tag-moved",
		"watch provenance SHA check: a pinned tag must still resolve to the recorded commit (both platforms)"},
	5: {"internal/core/conformance.go", "required_check_enforced",
		"pipelineWired() degrades required-check enforcement to an attestation, invisible in-tree on both"},
	6: {"internal/core/onboard.go", "--codeowners is GitHub-only",
		"Onboard() rejects --codeowners on non-github SCM (Azure Repos does not honour CODEOWNERS)"},
	7: {"internal/ci/ci.go", "##vso[task.setvariable",
		"AzurePipelines.EmitOutput() emits raw values on the ##vso line — no multi-line/JSON transport"},
}

// TestLedgerCodeAnchorsPresent enforces the ledger→code direction: every entry
// has an anchor, every anchor has an entry, and every anchor's marker is present
// in its file. A ledger entry whose difference has been removed from the code
// (or an anchor that drifts) fails here.
func TestLedgerCodeAnchorsPresent(t *testing.T) {
	root := repoRoot(t)
	entries := parseLedger(t, root)
	if len(entries) != len(ledgerAnchors) {
		t.Fatalf("ledger has %d entries but the anchor map has %d — keep them in lockstep",
			len(entries), len(ledgerAnchors))
	}
	for _, e := range entries {
		a, ok := ledgerAnchors[e.Num]
		if !ok {
			t.Errorf("ledger entry #%d has no code anchor in ledgerAnchors", e.Num)
			continue
		}
		data, err := os.ReadFile(filepath.Join(root, a.file))
		if err != nil {
			t.Errorf("entry #%d: read %s: %v", e.Num, a.file, err)
			continue
		}
		if !strings.Contains(string(data), a.marker) {
			t.Errorf("entry #%d anchor missing: %q not found in %s (%s)",
				e.Num, a.marker, a.file, a.note)
		}
	}
}

// -- code → ledger completeness guard -----------------------------------------
//
// Design: for a fixed small set of Layer-1 files we parse the Go AST and, for
// each top-level declaration (func / method / type / var), check whether its
// source span contains any platform token. A declaration that does is a
// per-platform fork SITE, keyed as "<file>::<decl>" (methods carry their
// receiver type, e.g. "AzurePipelines.EmitOutput"). Every discovered site must
// appear in forkAllowlist with a justification — either a ledger entry number,
// or "dialect, not semantic" for pure output-format / file-layout / dispatch
// divergence that carries no behavioural difference (platform-integration §2).
//
// This is deliberately AST-scoped (not a raw line grep) so it survives
// whitespace and line-number churn, yet is precise enough to flag a genuinely
// new platform branch. The scm switch values are quoted (`"github"` / `"ado"`)
// so they match only string literals, not the many identifiers (githubPR,
// adoHandler, …) that embed those words.

var platformTokens = []string{
	"github-actions", "azure-pipelines", "azure-repos",
	"GitHubActions", "AzurePipelines",
	`"github"`, `"ado"`,
}

var scanFiles = []string{
	"internal/ci/ci.go",
	"cmd/vendkit/handler.go",
	"internal/core/conformance.go",
	"internal/core/onboard.go",
}

// forkAllowlist covers every per-platform fork site in scanFiles. To satisfy the
// guard when you add a platform branch: either record a ledger entry (and its
// anchor above) and cite "ledger #N" here, or — if the divergence is pure output
// dialect / file layout / dispatch with no behavioural difference — allowlist it
// as "dialect, not semantic".
var forkAllowlist = map[string]string{
	// internal/ci/ci.go — the CI output surface (§2). Pure dialect by design,
	// EXCEPT AzurePipelines.EmitOutput, whose inability to carry multi-line/JSON
	// on a ##vso line is a genuine behavioural difference — ledger #7 (Part B).
	"internal/ci/ci.go::Detect":                     "dialect, not semantic (CI surface selection, §1)",
	"internal/ci/ci.go::GetSurface":                 "dialect, not semantic (surface factory)",
	"internal/ci/ci.go::GitHubActions":              "dialect, not semantic (GHA output-surface type)",
	"internal/ci/ci.go::GitHubActions.EmitOutput":   "dialect, not semantic (GITHUB_OUTPUT append)",
	"internal/ci/ci.go::GitHubActions.EmitSummary":  "dialect, not semantic (GITHUB_STEP_SUMMARY)",
	"internal/ci/ci.go::GitHubActions.EmitError":    "dialect, not semantic (::error:: annotation)",
	"internal/ci/ci.go::AzurePipelines":             "dialect, not semantic (ADO output-surface type)",
	"internal/ci/ci.go::AzurePipelines.EmitOutput":  "ledger #7 (##vso lines cannot carry multi-line/JSON)",
	"internal/ci/ci.go::AzurePipelines.EmitSummary": "dialect, not semantic (##vso uploadsummary)",
	"internal/ci/ci.go::AzurePipelines.EmitError":   "dialect, not semantic (##vso logissue)",

	// cmd/vendkit/handler.go
	"cmd/vendkit/handler.go::cmdHandler": "dialect, not semantic (scm dispatch github|ado; per-kind differences are ledger #2 and #3)",

	// internal/core/conformance.go
	"internal/core/conformance.go::pipelineFiles": "dialect, not semantic (per-CI pipeline file layout)",
	"internal/core/conformance.go::hasEvent":      "ledger #1 (Azure Repos pr: triggers are not tree-decidable)",
	"internal/core/conformance.go::detect":        "ledger #6 (azure-repos has no CODEOWNERS → required-reviewers attest)",
	"internal/core/conformance.go::pathsLockstep": "dialect, not semantic (per-CI path-filter YAML shape)",

	// internal/core/onboard.go
	"internal/core/onboard.go::scaffoldOutputs": "dialect, not semantic (per-CI scaffold file layout)",
	"internal/core/onboard.go::HandlerModules":  "dialect, not semantic (scm → reference-handler arg)",
	"internal/core/onboard.go::Onboard":         "ledger #6 (--codeowners rejected on non-github SCM)",
	"internal/core/onboard.go::manualSteps":     "ledger #6 (azure-repos required-reviewers checklist) + dialect (per-CI attestation note)",
}

func containsPlatformToken(s string) bool {
	for _, tok := range platformTokens {
		if strings.Contains(s, tok) {
			return true
		}
	}
	return false
}

func recvTypeName(e ast.Expr) string {
	switch t := e.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		return recvTypeName(t.X)
	}
	return "recv"
}

func declName(decl ast.Decl) string {
	switch d := decl.(type) {
	case *ast.FuncDecl:
		if d.Recv != nil && len(d.Recv.List) > 0 {
			return recvTypeName(d.Recv.List[0].Type) + "." + d.Name.Name
		}
		return d.Name.Name
	case *ast.GenDecl:
		for _, spec := range d.Specs {
			switch s := spec.(type) {
			case *ast.TypeSpec:
				return s.Name.Name
			case *ast.ValueSpec:
				if len(s.Names) > 0 {
					return s.Names[0].Name
				}
			}
		}
	}
	return ""
}

// forkSites parses rel and returns the "<rel>::<decl>" key of every top-level
// declaration whose source span contains a platform token.
func forkSites(t *testing.T, root, rel string) []string {
	t.Helper()
	path := filepath.Join(root, rel)
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, src, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", rel, err)
	}
	var sites []string
	for _, decl := range f.Decls {
		name := declName(decl)
		if name == "" {
			continue
		}
		span := src[fset.Position(decl.Pos()).Offset:fset.Position(decl.End()).Offset]
		if containsPlatformToken(string(span)) {
			sites = append(sites, rel+"::"+name)
		}
	}
	sort.Strings(sites)
	return sites
}

// TestLedgerCodeForkCoverage enforces the code→ledger direction: every
// per-platform fork site in the scanned Layer-1 files is covered by
// forkAllowlist, and no allowlist entry is stale. A new platform branch with no
// ledger entry and no allowlist justification fails here.
func TestLedgerCodeForkCoverage(t *testing.T) {
	root := repoRoot(t)
	found := map[string]bool{}
	for _, rel := range scanFiles {
		for _, site := range forkSites(t, root, rel) {
			found[site] = true
			if _, ok := forkAllowlist[site]; !ok {
				t.Errorf("un-recorded platform fork %q: add a ledger entry "+
					"(+ anchor in ledgerAnchors) or allowlist it as a dialect "+
					"divergence in forkAllowlist", site)
			}
		}
	}
	for site := range forkAllowlist {
		if !found[site] {
			t.Errorf("stale allowlist entry %q: no platform token is found at "+
				"that declaration anymore — remove it", site)
		}
	}
}
