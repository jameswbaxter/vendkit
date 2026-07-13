// Command docsgen renders the Markdown docs tree (docs/) into a self-contained,
// versioned static site under a dist/ directory, one directory per release
// version plus a `latest/` alias and a `versions.json` manifest read by the
// in-page version selector.
//
// This is a BUILD-TIME tool only. It is a separate `package main` and is never
// imported by cmd/vendkit, so the shipped `vendkit` binary does not depend on
// goldmark or anything here at runtime.
//
// Usage:
//
//	go run ./internal/docsgen --docs docs --out dist --version v0.2.0
//
// The tool is idempotent and accumulating: run it once per release version into
// the same --out directory. Each run renders --version into <out>/<version>/,
// merges the version into <out>/versions.json (newest first, `latest` marked by
// SemVer order), and — when the rendered version is the newest known — mirrors
// it into <out>/latest/. Previously published versions are left untouched.
//
// Output is deterministic (stable ordering, no embedded timestamps) so the
// generated tree is diff-friendly and testable.
package main

import (
	"flag"
	"fmt"
	"os"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "docsgen:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	fs := flag.NewFlagSet("docsgen", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	docsDir := fs.String("docs", "docs", "source Markdown docs directory")
	outDir := fs.String("out", "dist", "output site directory (accumulates versions across runs)")
	version := fs.String("version", "", "release label to render, e.g. v0.2.0 (required)")
	fs.Usage = func() {
		fmt.Fprint(os.Stderr, `docsgen — render the docs/ tree into a versioned static site.

Usage:
  go run ./internal/docsgen --version <label> [--docs <dir>] [--out <dir>]

Flags:
  --docs <dir>      Source Markdown directory (default "docs").
  --out <dir>       Output site directory (default "dist"). Accumulates:
                    run once per version; existing versions are preserved.
  --version <label> Release label for this render, e.g. v0.2.0 (required).

Behaviour:
  * Renders <docs> into <out>/<version>/ (index.html + one page per doc).
  * Merges <version> into <out>/versions.json (newest first; SemVer order
    picks the `+"`latest`"+`).
  * Mirrors the newest version into <out>/latest/.
  * Every page carries a left nav and a version-selector dropdown that reads
    <out>/versions.json at runtime, so older pages surface newer versions.
`)
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *version == "" {
		fs.Usage()
		return fmt.Errorf("--version is required")
	}
	return Generate(Options{
		DocsDir: *docsDir,
		OutDir:  *outDir,
		Version: *version,
	})
}
