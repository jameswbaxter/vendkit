// CLI surface freeze (1.0). The top-level command surface documented in
// docs/specs/cli.md is a public API frozen at v1.0.0 (COMPATIBILITY.md):
// adding, removing, or renaming a command is a MAJOR event with a migration
// entry. This test — modelled on ledger_test.go — locks three representations
// together so none can drift silently:
//
//   1. frozenSurface — the authoritative v1.0.0 snapshot of top-level commands,
//      each carrying the doc token that must appear in cli.md;
//   2. code → snapshot: every `case` value in the command dispatch switch
//      (cmd/vendkit/main.go) is a frozen command, and every frozen command has
//      a dispatch case;
//   3. snapshot → docs: every frozen command is documented in cli.md.
//
// A new command landing in main.go with no frozenSurface entry fails here, as
// does a frozen command that loses its dispatch case or its cli.md row.
// Changing the surface on purpose means editing frozenSurface — the deliberate
// 1.0 gate. It reads the canonical cmd/ and docs/ sources at test time (located
// via runtime.Caller, through repoRoot in ledger_test.go), so it tracks the
// tree it is compiled from.

package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// frozenSurface is the v1.0.0 CLI command surface. The key is the command token
// as it appears in the dispatch switch; docToken is the substring that must be
// present in docs/specs/cli.md to prove the command is still documented. Keep
// this in lockstep with cmd/vendkit/main.go's `switch cmd` and the cli.md
// machine/human tables — that lockstep is exactly what the freeze protects.
var frozenSurface = map[string]struct{ docToken, note string }{
	"generate":          {"vendkit generate", "machine tier — manifest build/verify"},
	"gate":              {"vendkit gate", "machine tier — consumer integrity + INV-7"},
	"sync":              {"vendkit sync ", "machine tier — low-level materialise"},
	"sync-pipeline":     {"vendkit sync-pipeline", "machine tier — full sync lane"},
	"release":           {"vendkit release", "machine tier — cut a release"},
	"watch":             {"vendkit watch", "machine tier — detect upstream releases"},
	"migrations":        {"vendkit migrations ", "machine tier — resolve migration window"},
	"migrations-verify": {"vendkit migrations-verify", "machine tier — obligation check"},
	"conformance":       {"vendkit conformance", "machine tier — adoption check"},
	"fleet":             {"vendkit fleet", "machine tier — read-only aggregation"},
	"self-verify":       {"vendkit self-verify", "machine tier — engine-pin re-assert"},
	"handler":           {"vendkit handler", "machine tier — reference delivery handler"},
	"push-hint":         {"vendkit push-hint", "machine tier — publisher dispatch step"},
	"init":              {"vendkit init", "scaffold a consumer (machine table)"},
	"onboard":           {"alias: `onboard`", "init alias"},
	"status":            {"vendkit status", "human tier — per-slice rollup"},
	"diff":              {"vendkit diff", "human tier — unified diff of an upgrade"},
	"update":            {"vendkit update", "human tier — the whole upgrade"},
	"explain":           {"vendkit explain", "human tier — what a finding/refusal means"},
}

// dispatchCases parses cmd/vendkit/main.go and returns every string case value
// of the top-level command switch (`switch cmd { … }`). It locates the switch by
// its tag identifier `cmd`, so it is unaffected by the other, tagless switch in
// run() (error classification). A `case "init", "onboard":` clause naturally
// yields both tokens.
func dispatchCases(t *testing.T, root string) []string {
	t.Helper()
	path := filepath.Join(root, "cmd", "vendkit", "main.go")
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, src, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	var cases []string
	ast.Inspect(f, func(n ast.Node) bool {
		sw, ok := n.(*ast.SwitchStmt)
		if !ok {
			return true
		}
		id, ok := sw.Tag.(*ast.Ident)
		if !ok || id.Name != "cmd" {
			return true
		}
		for _, stmt := range sw.Body.List {
			cc, ok := stmt.(*ast.CaseClause)
			if !ok || cc.List == nil { // nil List == the default clause
				continue
			}
			for _, e := range cc.List {
				if lit, ok := e.(*ast.BasicLit); ok && lit.Kind == token.STRING {
					if v, err := strconv.Unquote(lit.Value); err == nil {
						cases = append(cases, v)
					}
				}
			}
		}
		return false // found the command switch; no need to descend further
	})
	sort.Strings(cases)
	return cases
}

// TestSurfaceDispatchMatchesSnapshot enforces the code↔snapshot direction: the
// set of dispatched commands in main.go equals frozenSurface exactly. A new
// command with no snapshot entry, or a snapshot entry with no dispatch case,
// fails here.
func TestSurfaceDispatchMatchesSnapshot(t *testing.T) {
	root := repoRoot(t)
	cases := dispatchCases(t, root)
	if len(cases) == 0 {
		t.Fatal("parsed zero dispatch cases — the `switch cmd` shape changed")
	}
	seen := map[string]bool{}
	for _, c := range cases {
		seen[c] = true
		if _, ok := frozenSurface[c]; !ok {
			t.Errorf("command %q is dispatched in cmd/vendkit/main.go but absent from "+
				"frozenSurface: the CLI surface is frozen at v1.0.0 — adding a command is "+
				"a MAJOR event (COMPATIBILITY.md). Add it to frozenSurface and cli.md "+
				"deliberately, or revert the new case.", c)
		}
	}
	for cmd := range frozenSurface {
		if !seen[cmd] {
			t.Errorf("frozen command %q has no dispatch case in cmd/vendkit/main.go — "+
				"removing a command is a MAJOR event; if intended, drop it from "+
				"frozenSurface and cli.md and record a migration entry.", cmd)
		}
	}
}

// TestSurfaceDocumentedInCLISpec enforces the snapshot→docs direction: every
// frozen command is present in docs/specs/cli.md via its documented token.
func TestSurfaceDocumentedInCLISpec(t *testing.T) {
	root := repoRoot(t)
	data, err := os.ReadFile(filepath.Join(root, "docs", "specs", "cli.md"))
	if err != nil {
		t.Fatalf("read docs/specs/cli.md: %v", err)
	}
	spec := string(data)
	for cmd, e := range frozenSurface {
		if !strings.Contains(spec, e.docToken) {
			t.Errorf("frozen command %q is not documented in cli.md (looked for %q; %s) — "+
				"the frozen surface and its spec have drifted apart.", cmd, e.docToken, e.note)
		}
	}
}
