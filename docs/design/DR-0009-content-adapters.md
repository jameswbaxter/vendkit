# DR-0009 — Content adapters: identity copy by default, named transforms by declaration

- **Status:** accepted
- **Date:** 2026-07-08

## Context

Vendoring is most trustworthy when the consumer's copy is byte-identical to the
publisher's at the same path — diffable against upstream, identical at every
tier hop. But two real needs break pure identity: vendored files landing in a
shared directory can shadow consumer-owned files with the same name
(namespacing), and some file formats carry publisher-side unions that should be
pruned to what a given consumer actually hosts (localisation) to avoid
permanent dead configuration.

## Decision

The **identity copy is the default and the norm**: verbatim bytes, identical
path. Deviations exist only as **named adapters declared in the export
declaration** — v1 ships exactly two (`prefix-namespace`, `glob-localise`).
Adapters must be deterministic pure functions of (file bytes, adapter params,
consumer profile name); they may never read the consumer tree. The consumer
manifest hashes the **post-adapter** bytes, so adapted files are drift-gated
exactly like identity copies, and unknown adapter kinds are a hard error rather
than a silent skip.

## Alternatives considered

- **Pure identity, no adapters.** Namespacing collisions push consumers to
  rename vendored files by hand — instant drift; dead union config generates
  permanent noise that trains consumers to ignore advisories.
- **Arbitrary transform hooks (scripts in the declaration).** Unbounded,
  unreviewable transforms would break INV-2 (purity) and the security model's
  "no inline upstream code" rule.
- **Consumer-side patch overlays (quilt-style).** Local patches over vendored
  files are precisely hand-drift with extra steps; they invert the ownership
  model.

## Consequences

- New adapter kinds require a framework MAJOR (consumers' engines must
  understand every kind their manifests were produced with).
- Adapter output stability gets property tests (testing §1); INV-1 must hold
  across adapter configurations.
- Cross-hop identity means a tier-2 publisher re-exports vendored files
  byte-identically — enabling, with INV-7 disjointness, coherent multi-slice
  consumers.
