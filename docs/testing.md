# Testing strategy and the conformance kit

The invariants (architecture §3) are the product. Testing exists to make each
one executable, on both platforms, forever. Three tiers:

## 1. Unit/property tests (Layer 0)

Pure-function coverage: normalisation (CRLF/CR/trailing-ws/binary round-trips),
glob matching (one shared matcher — resolver, verifier, and gate must use the
same implementation, asserted by test), version grammar/ordering incl. rc and
retraction, declaration validation errors, adapter output stability. The
build-time docs-site generator (`internal/docsgen`) is unit-tested too — it
renders the real `docs/` tree deterministically (stable ordering, no embedded
timestamps) so a byte-diff catches regressions (DR-0018).

Property tests worth the setup:
- **INV-2/INV-3:** for generated random trees + declarations,
  `check` prediction equals `apply` result; `apply` twice = fixed point.
- **INV-1:** `generate → materialise → gate --strict` is clean across random
  profile bindings and adapter configs.

## 2. Scenario kit (the machinery testing itself)

A harness that builds throwaway publisher/consumer git repos and drives the
CLI end-to-end, no network, neutral CI surface, deliveries asserted through
the journal handler. Core scenario matrix:

| Scenario | Asserts |
|---|---|
| clean vendored tree | gate strict passes |
| hand-edit / delete / chmod a vendored file | gate strict fails; advisory reports but exits 0 |
| CRLF re-checkout | gate passes (normalisation) |
| sync same-version | `update-available=false`, no probe |
| sync newer, no content delta | `changed=false`, green stop |
| sync with update/removal/reconcile-addition | correct report classes; files never auto-deleted; output tree passes gate (INV-1) |
| localise + prefix adapters | consumer manifest self-consistent; INV-1 holds |
| two slices, disjoint | gate `--all` passes |
| two slices, colliding path | `collision` finding (INV-7) |
| migration window resolve + verify | window arithmetic incl. multi-version jump; zero obligations = green no-op |
| retracted target | sync refuses, exit 3 |
| seed scaffolded on onboard, then edited | gate strict passes (free to diverge) |
| seed path pre-exists in consumer | adopted: entry added, file byte-untouched |
| upstream template changes | consumer copy untouched; `template-updated` note in PR body (informational default), suppressed by `seeds.notes: silent` |
| consumer deletes a seeded file | never re-seeded; dropping the entry + reconcile re-offers it |
| seed path claimed by second slice | `collision` finding (INV-7 covers seeds) |
| template retired upstream | entry dropped, copy untouched; patch-grade release allowed |
| tag-moved simulation (rewrite tag in fixture repo) | sync refuses; watch raises integrity finding |
| release cut: stale manifest / existing tag / non-monotonic version / missing migration on removal | each refused |
| watch dry-run | no network, exit 0, empty report |
| `ci: none` end-to-end | no pipelines scaffolded; gate/watch/conformance run manually; pipeline rules `skipped`; sync pushes branch, `pr-delivered=false` + intent emitted |
| stray `.vendkit/*.yml` | usage error (strict namespace), never a silent skip |
| init SCM inference | origin remote → `scm:`; no remote + no `--scm` = usage error |
| CODEOWNERS opt-in | absent by default; `--codeowners` writes stanza (GitHub); refused on azure-repos with policy pointer |
| PR/handoff intents | journal handler receives protocol-versioned documents; dedup keys and deterministic branch as specified |

## 3. Platform matrix (Layer 1/2)

The scenario kit (§2) pins `VENDKIT_PLATFORM=neutral` by construction, so it
proves *behaviour* platform-free but never touches the two live output
dialects. The platform matrix closes that gap. It runs in real CI per platform
and has three layers:

- **Surface dialect tests** (`internal/ci`, run on every platform and locally):
  the `github-actions` and `azure-pipelines` `Surface` implementations —
  `GITHUB_OUTPUT` append, `::error::` annotation, step-summary; `##vso`
  `setvariable`/`logissue`/`uploadsummary` mapping — plus `Detect` precedence.
- **Live wiring smoke** (real runner/agent, not self-injected env):
  - **GHA** — [`.github/workflows/platform-matrix.yml`](../.github/workflows/platform-matrix.yml):
    under a real runner `Detect()` resolves to `github-actions`, so
    `generate --check` must append `fresh=true` to the runner-provided
    `$GITHUB_OUTPUT`, and a downstream step must consume it via
    `steps.<id>.outputs.fresh` (isOutput round-trip). **Live and green.**
  - **ADO** — [`azure-pipelines.yml`](../azure-pipelines.yml): the mirror,
    asserting the `##vso[task.setvariable …;isOutput=true]` directive is emitted
    and consumed across steps. **Authored but dormant** — this repo is on
    GitHub, so it needs an Azure DevOps project + GitHub service connection
    before it can run.
- **Release-binary smoke (DR-0016)** — the live download-and-run of the
  *release-attached* binaries, the one supply-chain step the layers above skip
  (they build from source; consumers fetch a checksummed binary from the
  Release — `scaffold/github-actions/sync.yml.tmpl`). Each run downloads the
  native asset, verifies it against the release `SHA256SUMS.txt` (the exact
  fetch+verify the scaffold ships), then runs the downloaded binary against the
  repo checked out at the released tag until `generate --check` reports
  `fresh=true`.
  - **GHA** — [`.github/workflows/release-smoke.yml`](../.github/workflows/release-smoke.yml):
    triggered on `release: published`, weekly, and on demand; matrixed over
    every OS family GitHub hosts — linux/amd64, linux/arm64, darwin/amd64,
    darwin/arm64, windows/amd64. **Live.** windows/arm64 ships in the release
    but has no GitHub-hosted runner, so it is built-and-checksummed, not run.
  - **ADO** — [`azure-pipelines-release-smoke.yml`](../azure-pipelines-release-smoke.yml):
    the mirror across the three hosted ADO OS families. **Authored but
    dormant** — same GitHub-service-connection prerequisite as the surface
    smoke above.

Still owed (tracked here, not yet built):

- The **full scenario kit re-run per platform** (un-pinning the neutral
  surface) — the smoke above covers the load-bearing wiring, not every scenario.
- Composite-action / template **parameter passthrough** assertions and the
  reference PR handler's **`GITHUB_TOKEN` refusal** (difference #2, present at
  `handler.go` but untested).
- The reference handlers' REST paths: **contract tests against recorded HTTP
  fixtures** plus a small **live smoke suite** in the framework repo's own CI.
  The handler *protocol* itself is already covered platform-free via the
  journal handler.
- **`base64-safe transport of JSON outputs` on ADO is not implemented** — the
  `azure-pipelines` surface emits raw values, so a multi-line/JSON output value
  would break the `##vso` line. All current outputs are single-line scalars, so
  this is latent. The constraint is now recorded as differences-ledger entry #7
  (platform-integration spec §6) and enforced by the ledger audit
  (`cmd/vendkit/ledger_test.go`); the base64-safe transport itself still ships
  with the full-kit matrix before any output value can carry JSON.

**Parity rule:** a scenario or behaviour difference discovered on one platform
must land as (a) a ledger entry (platform-integration spec §6) and (b) a
matrix test on both platforms — the peer-backend claim (INV-8) is only as
true as this matrix. The ledger↔code correspondence itself — every entry ↔ a
live Layer-1 branch (with an anchor), every Layer-1 platform fork ↔ an entry or
an allowlisted dialect divergence — is enforced by the ledger audit
(`cmd/vendkit/ledger_test.go`), which parses §6 at test time.

## 4. Consumer-facing self-tests

Scaffolded pipelines each carry a PR-time dry-run self-test (no secrets):
gate runs for real; sync/watch/conformance run `--dry-run` / `--check` against
the pinned release. A consumer PR that breaks its own vendkit wiring fails
before merge, not at the next scheduled run.
