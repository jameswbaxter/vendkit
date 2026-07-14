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
> Go engine (DR-0017): the **single implementation** (cmd/vendkit, internal/,
> embedded assets, one static binary). A Go-native unit + end-to-end scenario
> kit (internal/e2e) is the correctness ratchet; the self-host manifest is
> generated and freshness-checked by the Go binary. The Python reference
> engine, handlers, and pytest suite have been **removed**, and the name is
> locked to VendKit. CI (lint + build + test + self-host gate + a blocking
> govulncheck vuln gate) and a SemVer release workflow run under
> `.github/workflows/`, with the build toolchain pinned to a supported Go
> release through `go.mod` (single source of truth via `go-version-file` +
> `check-latest`) and third-party actions + Go modules kept current by grouped
> Dependabot PRs.
> The consumer-facing cutover is **done**: the Go-native reference handlers
> (`vendkit handler <scm>` for github/ado, in-binary) and all consumer scaffold
> templates now run the compiled `vendkit` binary — `scaffold/*/*.tmpl` fetch +
> checksum-verify the pinned engine (DR-0016), the slice config carries the
> `engine: {version, sha256}` pin advanced in lockstep by the sync PR, and
> `vendkit self-verify` re-asserts the running binary against it. No Python
> runtime residue remains in shipped code or scaffolds. Release-attached
> checksummed binaries + engine pin (DR-0016) are wired in the release workflow,
> and the **first tagged cut is done** — `v0.1.0` (annotated, surface-delta
> +0 -0) is on the remote.
>
> Every feature on the prior Still-open list has now shipped: REST-fixture
> contract tests for the GitHub/ADO reference handlers; the push-hint
> subscribers-file dispatch step (with the `--push-hint` receiver scaffold
> flag); the read-only `fleet` audit (with `conformance --json` widened to
> the fleet-view interchange document); API-verified attestations (the
> reference handlers now confirm controls against the SCM API instead of
> degrading to `unknown`); and a *versioned* docs site (a pure-Go
> `internal/docsgen` generator + tag-triggered gh-pages deploy, no
> Python/Node — DR-0018). The two M4 integration exercises have also
> landed: the behavioural-differences ledger audit (a Go test enforcing
> the platform-integration §6 ledger ↔ code correspondence both ways, plus
> a new entry #7) and the tier-chain demo (an offline e2e scenario driving
> a release framework → mid publisher → leaf with a push-hint at each hop).
> Also shipped since v0.1.0: the GitHub Pages landing page (`site/`) and its
> deploy workflow (now serving *versioned* docs off the `gh-pages` branch),
> live platform-matrix CI (testing §3, GHA surface with a dormant ADO peer
> in `azure-pipelines.yml`), and public-repo hygiene (SECURITY.md, issue
> templates). What remains before the M4 exit criteria (v1.0.0) are a docs
> pass, a live download-and-run of the release-attached binaries end-to-end
> on both platforms, and the 1.0 freeze itself (schema_version freeze, CLI
> surface freeze, compatibility policy in force) — a deliberate one-way
> decision, not yet taken. The current release line is v0.7.0.

## M0 — Skeleton and invariants (foundation)

- Repo scaffold per architecture §4; CI for lint + unit tests; license
  (Apache-2.0) and project naming decision (name locked: **VendKit**).
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
  sync; scaffolder MVP (`init`, both platforms, primary mode only).
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
