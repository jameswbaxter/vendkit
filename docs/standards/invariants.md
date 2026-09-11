---
id: VK-STD-invariants
title: "The ten invariants"
status: current
status_since: "2026-07-08"
last_verified: "2026-07-08"
summary: "Ten numbered properties that every release, lane and consumer tree satisfies, each turned into an executable check by the conformance kit, and each weakened only by a design record."
relations:
  regulates:
    - VK-FS-cli
    - VK-FS-conformance
    - VK-FS-export-declaration
    - VK-FS-handler-protocol
    - VK-FS-migrations
    - VK-FS-releases-and-versioning
    - VK-FS-security-model
    - VK-TS-manifest-and-gate
    - VK-TS-onboarding
    - VK-TS-platform-integration
    - VK-TS-release-watch
    - VK-TS-sync
    - VK-TS-testing
---

# Standard: The ten invariants

## Scope

The invariants are the product. Every other document here describes a mechanism;
this one states the properties those mechanisms exist to hold, and every
specification it regulates cites at least one of them by number.

They are numbered `INV-n` and the numbers are load-bearing: they are cited from
the specifications, the design records and the contributor guide, and a number
is never reused. A change that weakens an invariant needs a design record, not a
diff.

## Requirements

- **INV-1 (Composition).** For any release, declaration, profile and consumer
  tree: `sync apply` produces a tree and manifest that pass `gate verify
  --strict` with zero findings.
- **INV-2 (Purity).** `materialise` output is a pure function of
  (release tree, export declaration, consumer profile, current consumer
  manifest). No clock, no network, no environment dependence.
- **INV-3 (Prediction).** `sync check` reports exactly what `sync apply` would
  write. A successful check always exits 0 and prints `changed=true|false`; any
  non-zero exit is an infrastructure failure — a crash can never masquerade as
  staleness (the *porcelain contract*).
- **INV-4 (Refresh by default).** Without explicit scope reconciliation, sync
  refreshes the currently tracked file set only. Scope changes (additions via
  reconcile, removals detected upstream) are always surfaced in a reviewed PR;
  files are **never deleted from disk automatically**.
- **INV-5 (Immutability).** A released tag is never moved or reused. A fix is a
  strictly newer release. Consumers additionally record the resolved commit SHA
  so tag substitution is detectable (see security model).
- **INV-6 (Engine-version).** Materialise always runs the *target* release's
  engine (the pinned platform reference resolves the same tree that supplies
  both content and engine). Gate verification always runs against the *pinned*
  release's manifest. There is no version skew inside either operation.
  With the compiled engine shipped (DR-0017), this holds as an explicit,
  checksummed engine pin with schema-competence gating (DR-0016): the
  scaffolded lane fetches and `self-verify`s the pinned binary, and the sync
  PR advances the engine pin in lockstep with content. The human-tier CLI
  documents its own relaxation (cli spec).
- **INV-7 (Disjointness).** Across all slices vendored by one consumer, the
  `consumer_path` sets are pairwise disjoint. Checked by the gate lane on every
  PR.
- **INV-8 (Neutral core).** Layer 0 behaves identically on every platform and
  in no-CI (local/dev) execution — it calls no vendor service (DR-0014).
  Platform behaviour differences are confined to Layer 1 (CI surface,
  handlers, dialect parsing) and are enumerated in the platform-integration
  spec's differences ledger.
- **INV-9 (Dependency-free gate).** The consumer-side gate path runs a single
  static binary with no runtime prerequisites at all (DR-0016) — no
  interpreter, no third-party packages, no YAML parsing (the gate reads only
  the JSON manifest).
- **INV-10 (Review sovereignty).** No machinery ever merges, pushes to a
  protected branch, or mutates consumer-owned content directly. Every change
  lands as a PR under the consumer's normal review rules.

## Conformance

The conformance kit turns each requirement above into an executable check that
runs on both platforms; the testing specification states how each tier is
built and which invariant it covers.

The invariants are checked where they are held rather than in one place, which
is what keeps them properties of the system instead of assertions about it.

- **Held by the gate lane, on every consumer PR:** INV-1 and INV-7. The gate
  re-hashes every tracked path and refuses a duplicate `consumer_path` across
  slices.
- **Held by the scenario kit, driving the CLI as a subprocess:** INV-1, INV-2,
  INV-3 and INV-4. Purity and prediction are properties of an output, so they
  are asserted by running the lane twice and by comparing a check against the
  apply it predicted.
- **Held by the release command and the provenance record:** INV-5 and INV-6.
  Prevention is ref protection; detection is the recorded `source.commit`
  compared on every later watch and sync.
- **Held by the compiler and a test that reads imports:** INV-9. The
  consumer gate path imports the standard library and nothing else, which is
  decidable from the source rather than argued for.
- **Held by the platform matrix:** INV-8. The same scenario matrix runs against
  both backends, and every known difference is recorded in the differences
  ledger with its mitigation.
- **Held by the absence of a capability:** INV-10. The framework carries no
  merge capability anywhere, so it cannot be configured into using one, and a
  handler that has one is non-conforming.
