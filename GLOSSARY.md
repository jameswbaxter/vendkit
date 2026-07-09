# Glossary

Terms are normative across all specs. Where a spec disagrees with this file, fix
the spec.

- **Publisher** — a repository that declares and releases a slice. A repository
  can be both a publisher and a consumer (tiered chains).
- **Consumer** — a repository that vendors one or more slices.
- **Slice** — the coherent set of files a publisher exports, plus its identity
  (name, labels, namespaces, profiles). A consumer may vendor many slices, each
  from a different publisher, at different versions and cadences.
- **Slice name / prefix** — short identifier (e.g. `docs`, `std`) that namespaces
  every consumer-side artefact of the slice: manifest, config, pipelines.
- **Export declaration** — the publisher-side YAML file that defines the slice:
  include/exclude globs, identity, content adapters, profiles. The single source
  of slice identity; the tools carry none.
- **Release** — an immutable, annotated SemVer tag (`vMAJOR.MINOR.PATCH`) on the
  publisher repository. The tag *is* the release; there is no separate artefact.
- **Pin** — the consumer's record of which release it has vendored. The
  manifest's `source.release` provenance is authoritative; the
  platform-native reference line in the consumer's sync pipeline (located
  via `pin.pattern` + `pin.files` in the slice config) is the bootstrap
  fallback and the intent watch reads. Under `ci: none` the provenance is
  the only pin.
- **Manifest** — JSON record of the exported (publisher-side) or vendored
  (consumer-side) file set: per-file normalised checksum, consumer path, exec
  bit; plus source provenance (publisher, release, commit) on the consumer side.
- **Normalised checksum** — SHA-256 over a canonicalised byte stream (see
  manifest spec) so checkout settings can never masquerade as drift, while any
  real edit is detected.
- **Sync lane** — the freshness mechanism: scheduled (and optionally
  push-triggered) job that detects `pinned < target`, materialises the new
  release into the working tree, and opens **one reviewed PR**. Never
  auto-merges.
- **Gate lane** — the integrity mechanism: a check on every consumer PR that
  re-hashes vendored files against the manifest and fails on any hand-edit or
  deletion. Dependency-free; reads its file set from the manifest itself.
- **Composition invariant** — the guarantee that a sync-lane `apply` always
  produces a tree that passes the strict gate lane.
- **Materialise** — the engine operation that writes a release's slice into a
  consumer tree and rewrites the scoped manifest. A pure function of
  (release tree, export declaration, consumer profile, current manifest).
- **Scope reconciliation** — opt-in mode where materialise *expands* the tracked
  slice to newly exported files within the consumer's profile. Additions surface
  in the sync PR for review; scope never changes silently.
- **Profile** — a named consumer archetype declared by the publisher. Carries
  optional `export_slice` (which subset of the surface this archetype vendors)
  and optional adapter parameters. A consumer binds to one profile in its slice
  config.
- **Content adapter** — a named, declared transform applied to a class of files
  during materialise (e.g. filename-prefix namespacing, glob localisation).
  Default is the **identity copy**: verbatim bytes, identical path, at every hop.
- **Seeded file** — a scaffold-once template (declared under `seed:` in the
  export declaration): materialised only when the target path does not exist,
  then consumer-owned and free to diverge. The gate never checks it; sync never
  rewrites it; deleting it is respected (the manifest entry is the "seeding
  happened" record). Upstream template changes surface as an informational
  note in sync PRs (configurable per slice). See DR-0013.
- **Migration** — a declarative payload shipped with a release describing a
  structural or convention change to *consumer-owned* content that mechanical
  sync cannot perform, with machine-checkable verification obligations.
- **Handoff** — delivering resolved migrations (or watch findings) as a
  deduplicated work item, via the consumer's configured handoff handler.
  Core composes the intent; the handler owns the ticket system.
- **Conformance** — evaluation of a consumer tree + config against the rule spec
  shipped in the pinned release ("is this consumer correctly wired?"). Rules are
  data; detectors are code; pipeline-dialect parsing is keyed by the slice
  config's `ci:` axis.
- **Watch** — the scheduled collector that compares each vendored slice's pin
  against the publisher's latest release (git protocol) and hands findings to
  the handoff handler.
- **Handler** — a consumer-configured executable that receives an intent
  document (PR, handoff, fact-verify) on stdin and delivers it to a vendor
  service, honouring the protocol's idempotency obligations. The only place
  vendor services appear (DR-0014); reference handlers for GitHub and Azure
  DevOps ship with the framework.
- **SCM / CI axes** — the two recorded environment facts (DR-0015): `scm`
  (github | azure-repos — where the repo is hosted) and `ci`
  (github-actions | azure-pipelines | none — what runs the pipelines).
  Independent; `ci: none` is fully manual orchestration.
- **CI output surface** — the in-process adapter for the host CI's output
  dialect (step outputs, summaries, error annotations) — the only
  platform-specific code inside the engine.
- **Tier chain** — publishers consuming other publishers' slices (framework →
  doctrine publisher → domain publisher → leaf). Identity copies at every hop.
- **Retraction** — publisher-side declaration that a released version must not
  be adopted; watch and sync refuse to target retracted releases.
- **Channel** — release stream a consumer follows: `stable` (default; strict
  `vX.Y.Z` only) or `rc` (also pre-release tags `vX.Y.Z-rc.N`).
