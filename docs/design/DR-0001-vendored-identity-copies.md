# DR-0001 — Vendored identity copies, not package distribution

- **Status:** accepted
- **Date:** 2026-07-08

## Context

A framework that distributes curated files across repositories must choose a
distribution channel: a package registry (pip/npm/OCI), git-level composition
(submodule/subtree), or vendoring — copying files into the consumer's own tree.
The distributed content includes prose, config, instruction files and small
tools that consumers read, grep, review and run as if first-party.

## Decision

Slices are **vendored as identity copies**: verbatim bytes at identical paths
(except declared adapters), committed in the consumer's tree, tracked by a
checksum manifest, updated only by reviewed PRs. This applies at every hop of a
tier chain — including the framework's own engine, which downstream publishers
vendor rather than install.

## Alternatives considered

- **Package registry.** Adds a registry as trust root and an install step at
  every use; content is no longer in-tree for review/grep; provenance splits
  between lockfile and registry; prose/config don't fit registries. Lost on
  review ergonomics and on forking the provenance model — the manifest would
  guard the tree while the registry guarded the tools.
- **git submodule.** Pins whole repos, not slices; notorious consumer
  ergonomics; no per-file integrity or migration story. Lost immediately.
- **git subtree / transformation pipelines (Copybara-style).** Solves
  directional sync but is commit-oriented, not release-oriented: no SemVer
  windows, no migration payloads, no adoption rules; review diffs are merge
  artefacts rather than clean release deltas.

## Consequences

- Consumers own their copy: readable, greppable, reviewable, vendible offline.
- An integrity mechanism becomes mandatory (DR-0003, DR-0004) — vendored files
  invite hand-edits.
- Updates are PR-sized and human-reviewed by construction (INV-10).
- Storage duplication across consumers is accepted as trivial for the content
  sizes in scope.
