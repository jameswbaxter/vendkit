# DR-0007 — ADO and GitHub Actions as peer backends behind a port interface

- **Status:** accepted — the peer-backend stance stands; the port's *service* operations (PR, work items, fact verification, upstream reads) are superseded by [DR-0014](DR-0014-handler-protocol.md) and [DR-0015](DR-0015-scm-ci-axes.md); the CI output surface remains.
- **Date:** 2026-07-08

## Context

The framework must run natively on Azure DevOps Pipelines and GitHub Actions,
with more platforms plausible later. The platforms differ not just in API
dialect but in *semantics*: PR gating enforcement (Build Validation policies vs
required checks), token behaviour (workflow-token PRs not triggering workflows
on GitHub), trigger primitives (consumer-declared pipeline resources vs
publisher-sent dispatch), and tag protection primitives. A naive "abstract
everything" layer grows until it reimplements both platforms badly; a naive
"one platform first, port later" bakes the first platform's idioms into core
logic.

## Decision

A **six-operation port interface** (release listing, PR open/update, work-item
upsert, CI output/summary emission, platform-fact verification, credential
resolution) is the *only* place platforms appear. Layer 0 is platform-blind
(INV-8); Layer 2 wrappers are logic-free packaging; conformance detectors and
scaffold templates are **platform-keyed data** behind neutral rule/parameter
names. Both bindings ship from day one and run the identical scenario matrix
(testing §3); semantic differences are not abstracted away but recorded in a
**behavioural-differences ledger** with per-platform mitigations, and where a
fact is not tree-decidable on a platform, the detector degrades to attestation
with an API upgrade path.

## Alternatives considered

- **ADO-first, port later.** Retrofitting the port after platform idioms have
  leaked into engine, spec wording and rule schemas costs more than designing
  the seam now — and the seam is small (six operations).
- **A general CI abstraction layer (support "any platform").** Abstractions
  chosen without a second concrete implementation are guesses. Two real
  backends keep the interface honest; a third platform is the test of the
  design, not a day-one goal.
- **Separate per-platform tools sharing a library.** Doubles every CLI surface
  and scaffold; the differences ledger shows the divergence is in wiring and
  enforcement, not in operations — wrong axis to split on.

## Consequences

- Every new feature must answer "what is the port operation?" before merging;
  features that can't be expressed portably need a ledger entry and an
  explicit per-platform story.
- Double Layer 2 maintenance (templates *and* actions) — accepted; they are
  logic-free by rule, so the cost is packaging, not behaviour.
- The conformance kit's platform matrix becomes a permanent CI cost (the price
  of the "peer backends" claim).
