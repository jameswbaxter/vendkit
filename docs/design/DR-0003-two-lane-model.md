# DR-0003 — Two-lane distribution: sync PRs + PR-time gate

- **Status:** accepted
- **Date:** 2026-07-08

## Context

Vendored copies (DR-0001) decay two ways: they go **stale** (upstream releases
newer content) and they **drift** (someone hand-edits or deletes a vendored
file). One mechanism handling both couples concerns with different cadences,
credentials and failure modes: freshness needs network, write access and a
schedule; integrity needs to run on every PR, offline, in milliseconds.

## Decision

Two independent lanes:

- **Sync lane (freshness):** scheduled/push-triggered; materialises the target
  release; opens one reviewed PR; needs credentials.
- **Gate lane (integrity):** every consumer PR; re-hashes vendored files
  against the manifest; dependency-free, offline; required check.

Bound by the **composition invariant (INV-1)**: sync output always passes the
strict gate — files and manifest are rewritten from the same release tree in
one operation, so a sync PR is green by construction.

## Alternatives considered

- **One combined job.** The PR-time path inherits the sync path's credentials
  and network dependence, so a registry/network blip blocks unrelated merges
  and the integrity check stops being deterministic.
- **Integrity by CODEOWNERS alone.** Ownership review deters but cannot
  *detect* — a reviewer sees a diff, not a divergence-from-upstream. CODEOWNERS
  is kept as defence-in-depth for the machinery's own files.
- **Integrity by periodic audit instead of PR gate.** Drift merges first and is
  found later; remediation then fights history. Blocking at the PR is strictly
  cheaper.

## Consequences

- INV-1 becomes the framework's central testable guarantee (scenario kit).
- The gate must stay dependency-free (INV-9) so it can run anywhere, including
  minimal CI images — this constrains the manifest to JSON.
- Two lanes → two conformance wiring rules; the scaffolder wires both or the
  consumer isn't onboarded.
