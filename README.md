# VendKit

VendKit is a framework for **vendoring curated slices of files across repositories,
with provenance, integrity gates, and governed upgrades**. A *publisher* repository
declares which of its files form a distributable **slice**; *consumer* repositories
vendor a pinned copy of that slice and wire in machinery that:

- **watches** for new publisher releases and raises an upgrade prompt,
- **syncs** the vendored copy to a newer release as a reviewed pull request
  (adds, updates, and removals — never applied silently),
- **gates** every consumer PR so a hand-edit of a vendored file cannot merge,
- **verifies** structural migrations deterministically when a release reshapes
  content the mechanical sync cannot touch,
- **checks conformance** of a consumer against the publisher's adoption rules.

It targets **Azure DevOps Pipelines and GitHub Actions as peer, first-class
backends** — and a fully manual mode (`ci: none`) with no CI at all. The
core engine is vendor-service-free (git + filesystem only): upstream reads
use the git protocol, and every delivery (sync PR, work item, platform-fact
verification) goes through an exec **handler protocol** with GitHub/ADO
reference handlers shipped in-tree — any executable honouring the protocol
extends support without an engine change. The consumer-side integrity path
is dependency-free (standard library only).

## Why not X?

- **Package managers / registries** distribute built artefacts with external
  provenance. VendKit distributes *source files into the consumer's own tree*,
  where they are reviewed, greppable, and owned like first-party code — while a
  checksum manifest keeps them faithful to upstream.
- **git submodule / subtree** pin whole repositories, not curated slices, and
  offer no drift gate, no per-file provenance, no migration lifecycle, and poor
  review ergonomics.
- **Copybara-style transformation pipelines** solve directional code sync for a
  monorepo boundary; VendKit is release-oriented (immutable tags, SemVer windows,
  migration payloads) and puts the consumer in control of every change via PR.
- **Renovate / Dependabot** update dependency manifests; they do not vendor file
  trees, enforce integrity between updates, or carry structural-migration
  payloads.

## Core concepts (one paragraph)

A publisher's **export declaration** selects files by glob and gives the slice an
identity (name, labels, namespaces, profiles). Cutting a **release** creates an
immutable SemVer tag; *the tag is the release*. The engine writes a **manifest**
recording every exported file's normalised checksum; consumers hold a scoped copy
of that manifest recording exactly what they vendored, from which release and
commit. Two lanes keep the copy faithful: the **sync lane** (scheduled or
push-triggered; opens one reviewed PR per upgrade) and the **gate lane** (runs on
every consumer PR; fails if a vendored file was hand-edited or deleted). A
**composition invariant** binds them: the sync lane's output always passes the
gate lane. Publishers can also ship **seeded files** — scaffold-once templates
materialised only where no file exists, then consumer-owned and free to
diverge, never clobbered (DR-0013). Releases that reshape *consumer-owned* content ship declarative
**migration** payloads, resolved per consumer and verified deterministically.
**Conformance** rules — shipped with each release — evaluate whether a consumer
is correctly wired. A **scaffolder** onboards a new consumer in one command.

See [GLOSSARY.md](GLOSSARY.md) for precise terms.

## Document map

| Document | Contents |
|---|---|
| [docs/architecture.md](docs/architecture.md) | Layering, components, data flow, the invariants |
| [docs/specs/export-declaration.md](docs/specs/export-declaration.md) | Publisher-side declaration schema |
| [docs/specs/manifest-and-gate.md](docs/specs/manifest-and-gate.md) | Manifest schema, normalisation, gate-lane semantics |
| [docs/specs/sync.md](docs/specs/sync.md) | Materialisation semantics, scope reconciliation, content adapters |
| [docs/specs/releases-and-versioning.md](docs/specs/releases-and-versioning.md) | Tags, immutability, provenance, retraction, channels |
| [docs/specs/release-watch.md](docs/specs/release-watch.md) | Upstream watch config and collector contract |
| [docs/specs/migrations.md](docs/specs/migrations.md) | Migration payload schema and lifecycle |
| [docs/specs/conformance.md](docs/specs/conformance.md) | Conformance spec schema and detector kinds |
| [docs/specs/handler-protocol.md](docs/specs/handler-protocol.md) | The delivery boundary: intents, handlers, idempotency |
| [docs/specs/platform-integration.md](docs/specs/platform-integration.md) | CI surfaces, credentials, differences ledger |
| [docs/specs/onboarding.md](docs/specs/onboarding.md) | Scaffolder behaviour; consumer configuration file |
| [docs/specs/security-model.md](docs/specs/security-model.md) | Threat model, credentials, tag protection |
| [docs/specs/cli.md](docs/specs/cli.md) | The `vendkit` CLI surface |
| [docs/testing.md](docs/testing.md) | Conformance kit and invariant test strategy |
| [docs/design/](docs/design/README.md) | Design records (DR-0001…) — the load-bearing decisions |
| [ROADMAP.md](ROADMAP.md) | Implementation milestones |

## Status

**Implemented and self-hosting.** A single Go engine (`cmd/vendkit/` +
`internal/`) builds to one static binary with embedded scaffolds (DR-0017).
CI output surfaces, GitHub/ADO/journal reference handlers, both scaffold sets,
the core conformance rules, and the human-tier CLI
(`status`/`diff`/`update`/`explain`) are in place; this repository generates
and freshness-checks its own manifest from its own `vendkit-export.yml`. The
golden vectors under `tests/vectors/` and the end-to-end scenario kit
(`internal/e2e/`) are the correctness ratchet. See [ROADMAP.md](ROADMAP.md) for
what remains before a public 1.0 (release-attached binaries per DR-0016, Layer 2
wrapper packaging, live platform-matrix CI, fleet audit, API-backed fact-verify
handlers).

Try it end to end without any CI platform (everything runs against local git
repos, deliveries land in the journal handler):

```sh
go build -o vendkit ./cmd/vendkit   # the engine, one static binary
go test ./...                       # unit + end-to-end scenario kit
./vendkit generate --check          # self-host manifest freshness
```

Schemas are versioned from `1` and there are **no** compatibility obligations
to any prior system: this is a clean-room design.

## License

[Apache-2.0](LICENSE).
