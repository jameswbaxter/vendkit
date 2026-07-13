# Testing strategy and the conformance kit

The invariants (architecture §3) are the product. Testing exists to make each
one executable, on both platforms, forever. Three tiers:

## 1. Unit/property tests (Layer 0)

Pure-function coverage: normalisation (CRLF/CR/trailing-ws/binary round-trips),
glob matching (one shared matcher — resolver, verifier, and gate must use the
same implementation, asserted by test), version grammar/ordering incl. rc and
retraction, declaration validation errors, adapter output stability.

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
and has two layers:

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
  this is latent; land it with the full-kit matrix (and a differences-ledger
  entry) before any output value can carry JSON.

**Parity rule:** a scenario or behaviour difference discovered on one platform
must land as (a) a ledger entry (platform-integration spec §6) and (b) a
matrix test on both platforms — the peer-backend claim (INV-8) is only as
true as this matrix.

## 4. Consumer-facing self-tests

Scaffolded pipelines each carry a PR-time dry-run self-test (no secrets):
gate runs for real; sync/watch/conformance run `--dry-run` / `--check` against
the pinned release. A consumer PR that breaks its own vendkit wiring fails
before merge, not at the next scheduled run.
