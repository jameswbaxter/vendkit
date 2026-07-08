# DR-0002 — Slice identity lives in the declaration, not the tools

- **Status:** accepted
- **Date:** 2026-07-08

## Context

Multiple publishers must run the same machinery for different slices (names,
manifests, namespaces, profiles), including publishers who *vendor* the
machinery from a tier above. If identity is embedded in tool code, every new
publisher forks the tools; if it is scattered across CLI flags, every call site
must repeat it consistently.

## Decision

All slice identity — name, labels, manifest filename, namespaces, adapters,
profiles, retractions — lives in the **export declaration**. Tools read
identity from the declaration they are pointed at and carry none of their own.
A second publisher is a second declaration driving byte-identical tools.

## Alternatives considered

- **Fork per publisher.** Every engine fix multiplies by publisher count;
  provenance of the forks is unmanaged. The exact failure the framework exists
  to prevent.
- **CLI flags / environment for identity.** Scatters one fact across N call
  sites (pipelines, scaffolds, docs); drift between call sites becomes a new
  failure class.
- **Central registry of slices.** Introduces a coordination point and breaks
  the "publisher knows no downstream / framework knows no publishers" locality.

## Consequences

- The declaration schema is a public API with the same compatibility
  obligations as the CLI (MAJOR + migration to change).
- Tools must be tested against multiple declarations (the scenario kit runs
  every case under at least two slice identities).
- Manifest files deliberately exclude display labels so they stay byte-stable
  across identity-only changes.
