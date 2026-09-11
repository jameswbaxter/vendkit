<!-- headwater:generated shelf_index. `headwater generate` writes this file, and `headwater generate --check` holds it. Edit the corpus, not this file. -->

# Design records

21 documents on this shelf, in the reading order this corpus derives.

- [Vendored identity copies, not package distribution](DR-0001-vendored-identity-copies.md) — Slices travel as verbatim bytes committed in the consumer's own tree and tracked by a checksum manifest, rather than installed from a package registry.
- [Slice identity lives in the declaration, not the tools](DR-0002-identity-in-declaration.md) — Every piece of slice identity lives in the export declaration, so a second publisher is a second declaration driving byte-identical tools.
- [Two-lane distribution: sync PRs + PR-time gate](DR-0003-two-lane-model.md) — Freshness and integrity run as two independent lanes, a credentialed sync PR and an offline PR-time gate, bound so that sync output always passes the gate.
- [Normalised content hashing](DR-0004-normalised-hashing.md) — Hashes are taken over a canonicalised UTF-8 stream with normalised line endings, so a checksum survives platform line-ending churn.
- [The tag is the release: immutable, SHA-anchored, retractable](DR-0005-tag-is-the-release.md) — An annotated SemVer tag is the whole release, never moved or reused, and a bad one is retracted by declaration rather than deleted.
- [Pull reconciliation with optional push hints](DR-0006-pull-with-push-hints.md) — Consumers reconcile by scheduled pull, and a publisher's push hint changes only when that sync runs, never what it does.
- [ADO and GitHub Actions as peer backends behind a port interface](DR-0007-platform-ports.md) — Platforms appear only behind a port interface with both bindings shipping together, and differences are recorded in a ledger rather than abstracted away.
- [Declarative migrations with deterministic verification](DR-0008-declarative-migrations.md) — Releases ship declarative migration payloads carrying machine-checkable verification obligations and no executable code.
- [Content adapters: identity copy by default, named transforms by declaration](DR-0009-content-adapters.md) — A verbatim copy is the default, and any deviation exists only as a named, deterministic adapter declared in the export declaration.
- [Scope changes are reviewed PR events, never automatic](DR-0010-review-gated-scope.md) — The tracked slice is the unit of consent: scope grows only inside a reviewed PR, and nothing in the framework deletes a consumer file.
- [One CLI; dependency-free consumer gate path](DR-0011-single-cli-stdlib-gate.md) — One CLI entrypoint, whose PR-blocking consumer path is standard-library only, so the gate needs no installs and no network.
- [One consumer config file per slice under `.vendkit/`](DR-0012-consumer-config-consolidation.md) — Each slice has exactly two files under the consumer config directory, found by fixed globs, so adding a slice cannot forget a wiring.
- [Seeded files: a scaffold-once lifecycle class](DR-0013-seeded-files.md) — Seeded files are scaffolded once and then free to diverge, with the manifest entry itself serving as the lifecycle record.
- [Detection and delivery split by an exec handler protocol](DR-0014-handler-protocol.md) — Detection is split from delivery, and delivery sits behind an exec handler protocol that receives a JSON intent on stdin.
- [Split the platform vocabulary into SCM and CI axes; formats not services](DR-0015-scm-ci-axes.md) — The platform vocabulary splits into independent SCM and CI axes that name formats rather than services.
- [The engine becomes a pinned, checksummed artefact (INV-6 revision)](DR-0016-engine-as-pinned-artefact.md) — The engine ships as a per-platform released binary that consumers pin by version and checksum instead of vendoring as source.
- [Single engine implementation in Go, with the scenario kit as the parity ratchet](DR-0017-single-implementation-go.md) — Go is the single engine implementation, and the scenario kit drives the CLI as a subprocess to ratchet parity before the Python engine retires.
- [Versioned docs site from a pure-Go static generator](DR-0018-versioned-docs-generator.md) — The versioned docs site is rendered by a small pure-Go generator that the shipped CLI never imports.
- [`.vendkit/` has two halves: `consumer/` and `publisher/`](DR-0019-vendkit-dir-two-halves.md) — The consumer config directory splits into consumer and publisher halves that no glob reaches across, and the publisher paths become defaults.
- [Slim the machinery slice: the engine ships as artefact only](DR-0020-slim-machinery-slice.md) — The machinery slice exports only what consumers consume in-tree, leaving the pinned release artefact as the engine's sole distribution channel.
- [The documentation is a governed corpus, typed and indexed by a pinned Headwater](DR-0021-headwater-governed-design-records.md) — The design records, the component specifications and the standards that bind them become one typed corpus checked in CI by a pinned Headwater binary, with the specifications split into functional and technical layers and every shelf index generated.
