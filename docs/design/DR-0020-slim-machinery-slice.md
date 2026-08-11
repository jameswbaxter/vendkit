# DR-0020 — Slim the machinery slice: the engine ships as artefact only

- **Status:** accepted
- **Date:** 2026-08-11
- **Supersedes:** amends [DR-0001](DR-0001-vendored-identity-copies.md)'s
  engine clause; completes the direction set by
  [DR-0016](DR-0016-engine-as-pinned-artefact.md)

## Context

DR-0001 committed the framework to identity copies at every hop of the tier
chain, "including the framework's own engine, which downstream publishers
vendor rather than install." That sentence was written when the engine was
interpreted Python: the vendored source *was* the executable, so vendoring it
put the engine under the same manifest, checksum and review regime as every
other slice file at zero extra cost.

Two later decisions removed that identity. DR-0017 replaced the Python engine
with a compiled Go binary, and DR-0016 moved execution to a released,
per-platform, checksummed artefact that consumers pin by version and sha256 —
explicitly rejecting both `go run` from the vendored checkout (a toolchain on
every runner) and vendoring the binary itself (bloat and churn). The Go
sources entered the export declaration during the DR-0017 transition,
alongside the Python reference they replaced, and simply stayed after the
cutover; no DR ever re-justified them.

The result is a dual distribution channel. The pinned binary is what every
scaffolded pipeline actually fetches, verifies and runs. The exported source
(`cmd/**`, `internal/**`, `assets.go`, `go.mod`, `go.sum`) is inert in every
consumer tree: nobody downstream builds it, and engine correctness is enforced
upstream by the golden vectors and the scenario kit, not by downstream review.
Meanwhile the inert copy has real costs:

- Every engine-internal refactor churns the manifest and lands a sync PR of
  Go diffs in every downstream tree — review noise with no decision content.
- Any dependency bump touches `go.mod`/`go.sum`, so mechanical PRs (Dependabot
  and the like) fail the manifest freshness gate unless someone regenerates
  the manifest by hand on each such branch.
- Consumer trees carry dozens of Go files whose only realistic fate is to be
  ignored.

## Decision

The machinery slice exports what consumers actually consume in-tree, and
nothing else:

- `scaffold/*/*.tmpl` — the consumer pipeline scaffolds
- `conformance/core-rules.yml` — the default wiring rules
- `LICENSE`

The engine's Go sources, `assets.go`, `go.mod` and `go.sum` leave the export.
The engine's sole distribution channel is the DR-0016 release artefact:
per-platform binaries plus a checksum file, pinned by `engine: {version,
sha256}` in each consumer's slice config and re-asserted by `self-verify`.

DR-0001's identity-copy rule continues to hold for all slice *content*, at
every hop. This DR re-classes the engine from content to executor: its
integrity story is the consumer-held checksum pin and tag-anchored release
provenance (DR-0005), not the vendored-tree manifest.

## Alternatives considered

- **Status quo — keep exporting the source.** Preserves DR-0001's letter, but
  the reviewability it buys is theoretical: downstream publishers neither
  build nor meaningfully review engine internals, and parity is already
  ratcheted upstream (DR-0017). The dual channel makes every internal
  refactor a downstream event and every dependency bump a gate failure.
  Rejected.
- **Drop only `go.mod`/`go.sum`.** Stops the dependency-bump churn but leaves
  an unbuildable source export — worse than either a complete source slice or
  none. Rejected.
- **Vendor the engine binary instead of the source.** Already considered and
  rejected in DR-0016 (tens of MB per platform, churn on every release).
- **Keep source as the execution path (`go run`).** Already rejected in
  DR-0016 (compiler on every runner, larger supply-chain surface than a
  checksummed static binary).

## Consequences

- The manifest shrinks from ~60 entries to a handful; engine-internal changes
  no longer generate downstream sync PRs, and `go.mod`/`go.sum` changes no
  longer touch the manifest, so dependency-only PRs pass the freshness gate
  without hand-regeneration.
- Tier chains still hold the engine's integrity at every hop — as the
  checksum pin in each consumer's slice config rather than as source bytes in
  the tree. Downstream publishers re-export scaffolds and rules; the engine
  reaches every hop as the same tag-anchored artefact.
- Air-gapped consumers keep the DR-0016 story (mirror the artefacts, override
  the fetch URL; the pin still decides trust) but lose the in-tree source
  fallback. Building from a tagged framework-repo checkout remains available
  to anyone who needs it.
- DR-0001 is amended, not edited: its engine sentence is historical as of
  this DR (DRs are immutable once accepted).
- Obligations: the export declaration and regenerated manifest change with
  this DR; architecture §4's self-hosting paragraph is updated to match.
  Removing vendored engine sources from existing consumer trees is a normal
  release delta — consumers pick it up as an ordinary sync PR whose diff is
  deletions.
- Bootstrap is unchanged: the release freshness pre-gate needs only the
  working tree.
