# DR-0016: The engine becomes a pinned, checksummed artefact (INV-6 revision)

Status: accepted (takes effect when the compiled engine ships — DR-0017)

## Context

INV-6 ("engine-version") currently holds *for free*: the sync pipeline pins
the publisher at TARGET, and because the engine is interpreted Python living
in that same tree, the checkout that supplies the content also supplies the
engine that materialises it. A compiled engine (DR-0017) breaks the free
ride — running Go from source on every consumer runner would require a Go
toolchain, which is heavier than the Python interpreter it replaces and
reintroduces a build step into the integrity path.

## Decision

1. **The engine is a released binary artefact**: built per platform
   (linux/amd64, linux/arm64, darwin/arm64, windows/amd64) by the framework
   repo's release pipeline and attached to the release; each release also
   publishes a checksum file.
2. **Consumers pin the engine explicitly.** The slice config for the
   machinery slice gains `engine: {version, sha256: {<platform>: <hex>}}`;
   the scaffolded pipelines' first step is fetch → verify checksum → cache
   (platform tool cache where available). `vendkit self-verify` re-asserts
   the running binary against the pinned expectation.
3. **INV-6 is restated**: *materialise runs an engine whose declared
   schema competence covers the target release.* Concretely: the engine
   refuses declaration/manifest `schema_version`s newer than it knows
   (already the unknown-key/schema hard-error rule), and a declaration may
   state `min_engine` to demand a newer engine explicitly. The sync PR that
   advances the content pin also advances the engine pin when the target
   requires it — one reviewed change, no skew window.
4. **The human tier keeps its documented relaxation** (cli spec): the
   installed engine runs against fetched trees, guarded by the same schema
   gating.

## Alternatives considered

- **`go run` from the pinned checkout** — preserves the original INV-6
  mechanics but puts a compiler on every runner and a compile in every lane;
  slower and a larger supply-chain surface than a checksummed binary.
  Rejected.
- **Vendoring the engine binary into consumer trees** — makes the gate
  hash its own executor (attractive!) but bloats every consumer repo by
  tens of MB per platform and churns it every engine release. Rejected;
  the checksum pin gives the same tamper-evidence without the bytes.
- **Keep the engine interpreted** — see DR-0017's alternatives; the
  dependency-free-binary win was judged worth this DR's cost.

## Consequences

- A network fetch and a supply-chain surface enter the integrity path;
  both are mitigated by the consumer-held checksum pin (a swapped artefact
  fails verification loudly) and release-pipeline provenance.
- Offline/air-gapped consumers can mirror artefacts and override the fetch
  URL; the checksum pin still decides trust.
- INV-9 strengthens from "stdlib-only Python" to "a static binary with no
  runtime prerequisites at all" — the gate no longer even needs Python.
- Until DR-0017 completes, the Python engine still rides the checkout and
  INV-6 applies in its original form; architecture.md carries a forward
  note.
