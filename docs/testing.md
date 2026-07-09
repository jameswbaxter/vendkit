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

A harness that builds throwaway publisher/consumer git repos and drives the CLI
end-to-end, no network, `neutral` port. Core scenario matrix:

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
| retracted target | `is-newer`/sync refuse, exit 3 |
| seed scaffolded on onboard, then edited | gate strict passes (free to diverge) |
| seed path pre-exists in consumer | adopted: entry added, file byte-untouched |
| upstream template changes | consumer copy untouched; `template-updated` note in PR body (informational default), suppressed by `seeds.notes: silent` |
| consumer deletes a seeded file | never re-seeded; dropping the entry + reconcile re-offers it |
| seed path claimed by second slice | `collision` finding (INV-7 covers seeds) |
| template retired upstream | entry dropped, copy untouched; patch-grade release allowed |
| tag-moved simulation (rewrite tag in fixture repo) | sync refuses; watch raises integrity finding |
| release cut: stale manifest / existing tag / non-monotonic version / missing migration on removal | each refused |
| watch dry-run | no network, exit 0, empty report |

## 3. Platform matrix (Layer 1/2)

The same scenario kit re-run under each port binding, in real CI:

- **GHA:** kit runs inside a workflow; asserts `GITHUB_OUTPUT`/step-summary
  wiring, composite-action parameter passthrough, and the sync PR credential
  refusal (difference #2).
- **ADO:** kit runs inside a pipeline; asserts `##vso` output mapping, template
  parameter passthrough, base64-safe transport of JSON outputs.
- Port REST methods (`list_release_tags`, `open_or_update_pr`,
  `upsert_work_item`) get contract tests against recorded HTTP fixtures plus a
  small live smoke suite in the framework repo's own CI (it is its own
  publisher — self-hosting is the permanent integration environment).

**Parity rule:** a scenario or behaviour difference discovered on one platform
must land as (a) a ledger entry (ports spec §6) and (b) a matrix test on both
platforms — the peer-backend claim (INV-8) is only as true as this matrix.

## 4. Consumer-facing self-tests

Scaffolded pipelines each carry a PR-time dry-run self-test (no secrets):
gate runs for real; sync/watch/conformance run `--dry-run` / `--check` against
the pinned release. A consumer PR that breaks its own vendkit wiring fails
before merge, not at the next scheduled run.
