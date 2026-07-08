# DR-0008 — Declarative migrations with deterministic verification

- **Status:** accepted
- **Date:** 2026-07-08

## Context

Mechanical sync refreshes manifest-tracked files. Some releases also invalidate
**consumer-owned** content — trees the consumer authored under conventions the
new release retires. The publisher cannot edit that content (it doesn't own
it, and every consumer's is different), yet leaving it silently stale defeats
the framework's purpose.

## Decision

Releases ship **declarative migration payloads**: what changed, whom it applies
to (profile + version window), how to detect the blast radius, natural-language
instructions for a remediator (human or AI agent), and machine-checkable
**verification obligations** (`must_be_absent` / `must_be_present` / named
checks). The lifecycle is resolve (pure window arithmetic) → handoff (deduped
work item) → verify (deterministic gate on the remediation PR; zero obligations
= green no-op, so it is safe as an always-on required check). Payloads contain
**no executable code**; custom checks reference vendored, gate-verified tools
only. A release-time pre-gate forces reshaping releases to carry a payload or
an explicit recorded override.

## Alternatives considered

- **Executable migration scripts.** Upstream-authored code running against
  consumer-owned content, per consumer, unreviewed at authoring time — a
  second code-execution channel with unbounded blast radius; and scripts
  cannot anticipate every consumer's local layout anyway. Instructions +
  obligations delegate the *how* to someone with local context while keeping
  the *what* checkable.
- **Release-notes prose only.** Unenforceable; the framework would know a
  structural change happened and not be able to say whether any consumer
  absorbed it.
- **Refusing structural change (append-only conventions).** Ossifies the
  published content; the pressure escapes as out-of-band manual edits, which
  is the drift the framework exists to prevent.

## Consequences

- The obligation glob matcher must be the same implementation everywhere
  (resolver, verifier, gate) — a stated conformance-kit contract.
- AI-agent remediation slots in naturally (work item → agent → PR → verify
  gate) without the framework trusting the agent: the deterministic verifier
  and human review hold the line (INV-10).
- Publishers accept authoring burden per reshaping release; the pre-gate makes
  forgetting impossible rather than relying on discipline.
