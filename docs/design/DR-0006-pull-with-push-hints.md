# DR-0006 — Pull reconciliation with optional push hints

- **Status:** accepted
- **Date:** 2026-07-08

## Context

New releases must reach consumers. A pull model (consumers poll on schedule) is
autonomous and self-healing but adds cadence latency, multiplied across tier
chains. A push model (publisher drives updates into consumers) is immediate
but requires publisher-held credentials into every consumer and a downstream
registry — inverting the trust locality that per-consumer review depends on.

## Decision

**Pull is the source of truth; push is a hint.** Every consumer runs scheduled
watch + sync (idempotent reconciliation). Optionally, a publisher release
*triggers* the consumer's existing sync pipeline early: consumer-declared
pipeline-resource triggers on ADO; a `repository_dispatch` receiver plus an
opt-in, consumer-PR-maintained subscribers file on GitHub. A hint changes
*when* the sync runs, never *what* it does; a lost hint costs latency, not
correctness. Hints compose hop-by-hop across tier chains, collapsing
multi-cadence propagation to same-day.

The full-push alternative (publisher opens PRs into consumers) is deliberately
excluded from the architecture's guarantees but not made impossible: because
materialise is pure (INV-2) and the port exposes `open_or_update_pr`, a central
distributor could be built later without engine changes.

## Alternatives considered

- **Pure pull.** Simplest; kept as the mandatory baseline. Rejected as the
  *only* mechanism because tier chains multiply cadence latency (a framework
  fix reaching leaves in weeks).
- **Full push (central distributor).** One policy-exempt, PR-capable credential
  spanning every consumer is a fleet-wide blast radius on compromise; the
  publisher must maintain a consumer registry; consumer-context adapters would
  run outside the consumer. Costs exceed the latency benefit at any
  foreseeable fleet size.
- **Platform event plumbing (service hooks → functions → queue).** Operates new
  infrastructure to deliver what a trigger declaration delivers natively.

## Consequences

- Watch/sync must be safe under double-fire and no-fire (INV-2/3 give this).
- The GHA hint relaxes "publisher knows no downstream" to an opt-in, auditable
  subscribers file — the only such relaxation, documented in the security
  model.
- Conformance mandates the schedules; hints are never a substitute for them.
