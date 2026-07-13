# DR-0018: Versioned docs site from a pure-Go static generator

Status: accepted

## Context

The docs of record are the Markdown under `docs/` (architecture, specs, design
records). A hand-authored GitHub Pages landing page (`site/index.html`) already
ships, but its "Docs" links pointed at `github.com/.../blob/main/docs/...` —
always `main`, never the release a consumer has pinned. M4 calls for a
*versioned* docs site: a reader on `v0.3.0` should see the `v0.3.0` docs, with a
selector to move between releases.

The obvious tools for this (mkdocs, Docusaurus, Antora) are Python or Node.
DR-0017 deliberately made VendKit a single Go implementation and the roadmap
records that "no Python runtime residue remains"; reintroducing an interpreter
or an npm tree purely to render docs would undo that and add a `setup-python`/
`setup-node` step and a lockfile to maintain. The consumer-facing promise —
"one static binary, needs nothing" — should extend to how the project builds
its own site.

## Decision

1. **A small static-site generator, written in Go, renders the versioned
   docs.** It lives at `internal/docsgen` as a standalone `package main` and is
   run at build time with `go run ./internal/docsgen --docs docs --out dist
   --version vX.Y.Z`. It is never imported by `cmd/vendkit`, so the shipped
   `vendkit` binary's dependency set is unchanged.
2. **Markdown→HTML uses `github.com/yuin/goldmark` (with the GFM extension)** —
   pure Go, no cgo, compiled only into the docsgen tool. It is a build-time
   dependency of the module, invisible to consumers who install the CLI. A
   hand-rolled renderer was rejected: goldmark handles the docs' tables, fenced
   code, autolinks and heading IDs correctly with far less risk than
   re-implementing CommonMark + GFM.
3. **Output is versioned and accumulating.** Each run renders one version into
   `dist/<version>/` (one page per doc plus an `index.html`), merges the version
   into a `dist/versions.json` manifest (newest first, SemVer order marking
   `latest`), and mirrors the newest version into `dist/latest/`. Every page
   carries a left nav and a version-selector dropdown that reads
   `versions.json` at runtime, so pages published for an older release surface
   releases added later. Internal `*.md` doc-to-doc links are rewritten to the
   rendered `*.html` paths within the same version.
4. **Output is deterministic** — stable ordering, no embedded timestamps — so
   the generated tree is diff-friendly and testable, and so re-running a version
   is idempotent.
5. **Publishing accumulates on a `gh-pages` branch.** The Pages workflow seeds
   from the currently published branch, overlays the change (landing page and/or
   a newly rendered version), and republishes the whole tree. This replaces the
   single-artifact `upload-pages-artifact`/`deploy-pages` model, which cannot
   accumulate versions. `peaceiris/actions-gh-pages` is a self-contained action
   run on the runner — it adds no Node/Python toolchain to the repository.

## Alternatives considered

- **mkdocs / mkdocs-material** — the natural fit for Markdown docs, but pulls in
  Python + pip + a `requirements.txt`, reintroducing the interpreter DR-0017
  retired. Rejected.
- **Docusaurus / Antora** — Node/npm toolchains with a `package.json` and
  lockfile to maintain and audit; heavier than the job needs. Rejected.
- **Hand-rolled Markdown renderer, zero new dependencies** — avoids a dependency
  but must correctly cover headings, paragraphs, fenced/inline code, links,
  emphasis, ordered+unordered lists and GFM tables across the real docs; a
  well-maintained pure-Go library is lower risk for the same output. Rejected in
  favour of goldmark.
- **Keep linking to `blob/main/docs/...`** — no build, but the docs never match
  a pinned release, which is the entire point of M4. Rejected.
- **Single-artifact GitHub Pages deploy** — simplest CI, but each release
  overwrites the last; versions cannot coexist. Rejected for the gh-pages
  accumulation pattern.

## Consequences

- `go build ./cmd/vendkit` is unaffected: docsgen is a separate main package and
  goldmark never enters the binary's dependency graph (verifiable with
  `go list -deps ./cmd/vendkit`).
- `go.mod` gains one build-time require, `github.com/yuin/goldmark`; `go mod
  tidy` keeps it and `go.sum` honest.
- The generator has its own tests (`internal/docsgen`), and rendering is
  exercised against the real `docs/` tree, so the site build is covered by
  `go test ./...` rather than only proven in CI.
- The GitHub Pages source must be switched once to the `gh-pages` branch (a repo
  setting, documented in `.github/workflows/pages.yml`).
- Adding docs is free: any new `*.md` under `docs/` is picked up automatically;
  no per-page registration.
