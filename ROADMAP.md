# Roadmap

Milestones are cumulative; each ends with the framework repo dogfooding
everything built so far (self-hosting is the permanent integration
environment). "Both platforms" always means the full scenario matrix on ADO
and GHA (testing.md §3).

> **Status:** M0 is complete and most of M1–M3's Layer 0 scope shipped with it
> (gate incl. INV-7, sync-pipeline, release with bump/migration pre-gates,
> watch incl. tag-moved, migrations resolve/verify, conformance with attest
> degradation, onboard for both platforms — all covered by the scenario kit
> against local git repos + the journal handler). Deliberate deviation from the original M1 shape:
> scaffolded consumer pipelines invoke the CLI directly from the pinned
> publisher checkout instead of going through `platforms/` wrapper files —
> same guarantees, fewer moving parts; wrappers remain M4 packaging polish.
> Still open: live platform-matrix CI (testing §3), REST-fixture contract
> tests for the GitHub/ADO reference handlers, push-hint dispatch step, fleet audit,
> API-verified attestations, public-repo hygiene.

## M0 — Skeleton and invariants (foundation)

- Repo scaffold per architecture §4; CI for lint + unit tests; license
  (Apache-2.0) and project naming decision (drop the provisional name).
- Layer 0: export-declaration parser + validation; normalisation + manifest
  build (`generate`, `generate --check`); version grammar + `is-newer`.
- Neutral CI surface + journal handler; CLI skeleton with the exit-code/output conventions (cli.md).
- Scenario-kit harness (throwaway git fixtures) with the manifest round-trip
  cases.
- **Exit criteria:** INV-2 property tests green; this repo generates and
  checks its own manifest from its own `vendkit-export.yml`.

## M1 — The two lanes, single platform pair

- `gate` (with `--all` + INV-7 collision detection); `sync --check/--apply`
  with adapters, reconcile-scope, provenance recording; porcelain contract.
- ADO + GHA CI surfaces (`emit_output`/`emit_summary`) and reference handlers; Layer 2 wrappers for gate and
  sync; scaffolder MVP (`onboard`, both platforms, primary mode only).
- Scenario matrix for INV-1/3/4/7 on both platforms.
- **Exit criteria:** a demo consumer on each platform vendors a slice from this
  repo, gate-protected, and takes a sync PR end-to-end. This repo release
  v0.1.0 cut by hand.

## M2 — Releases, watch, handoff

- `release` (freshness pre-gate, surface-delta bump enforcement, annotated
  tags); tag-protection setup docs + publisher conformance attests.
- `watch` (pin scan, channel filter, retraction, provenance `tag-moved`
  check); git-protocol tag listing + handoff handlers (issue + work item);
  credential purposes + liveness probe.
- Retraction + rc channels in `is-newer`/sync.
- **Exit criteria:** demo consumers detect and adopt a real release of this
  repo on both platforms, from watch finding → work item → sync PR → merge.
  This repo's releases now cut by `vendkit release`.

## M3 — Migrations and conformance

- Migration payload schema, `migrations` resolve, handoff rendering,
  `migrations-verify` as always-on green-no-op gate; release-time migration
  pre-gate.
- Conformance engine + core rules; `pipeline-wired` bindings per platform with
  attest degradation and `--verify-attestations` upgrade; waivers; scaffolder
  reports conformance gaps (onboarding checklist = conformance spec).
- Additive onboarding mode (multi-slice consumers); disjointness scenario.
- **Exit criteria:** a deliberately reshaping release (v0.x → v0.y with a
  removal) propagates to demo consumers via migration work item and verified
  remediation PR, on both platforms.

## M4 — Fleet features and 1.0 hardening

- Push hints: ADO pipeline-resource trigger scaffold flag; GHA
  repository_dispatch receiver + subscribers-file dispatch step; tier-chain
  demo (framework → mid publisher → leaf).
- Fleet audit (read-only aggregation over consumers; conformance `--json` as
  the interchange format).
- Behavioural-differences ledger audit; docs pass; public-repo hygiene
  (SECURITY.md, issue templates, versioned docs site).
- **Exit criteria:** v1.0.0 — schema_version freeze, CLI surface freeze,
  compatibility policy in force (MAJOR + migration entry for any breaking
  change).

## Deliberately deferred (recorded so they aren't re-litigated casually)

- Central push distributor (DR-0006 keeps the door open; no current need).
- Additional adapter kinds (DR-0009: new kinds are MAJOR events).
- Third CI platform (DR-0007/DR-0014: template pack + handler + ledger make it tractable; not before
  1.0).
- Signed manifests/attestations (provenance SHA + ref protection first; revisit
  if consumers cross trust domains).
