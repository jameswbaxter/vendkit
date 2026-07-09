# DR-0014: Detection and delivery split by an exec handler protocol

Status: accepted · Supersedes: the port-interface service methods of DR-0007

## Context

The original port interface (DR-0007) bundled two different responsibilities
behind one Protocol: *judgments* the engine owns (version compares, drift
findings, conformance rule evaluation) and *deliveries* to vendor services
(open a PR, upsert a work item, verify a platform fact via API). The
delivery half hardcoded a vendor list — GitHub Issues and Azure DevOps work
items — and every further ticket system (Jira, Linear, ServiceNow, Slack…)
would have meant an engine release. The list could only ever be "some but
not all".

A second conflation surfaced at the same time: `platform: github | ado`
stood in for the SCM host, the CI host, *and* the work-tracking system —
three products that do not have to travel together (see DR-0015).

## Decision

Split **detection from delivery** everywhere, with delivery behind an
exec-based protocol (handler-protocol spec):

1. Core commands produce *judgments* and, where a delivery is needed,
   compose a JSON **intent document** (PR intent, handoff finding,
   fact-verify query).
2. A consumer-configured **handler executable** receives the intent on
   stdin, delivers it to its vendor's API, and reports facts as `key=value`
   on stdout. Idempotency semantics (branch-keyed PR upsert, dedup-keyed
   work-item upsert) are protocol obligations on the handler.
3. **PR delivery is included** in the split. The reviewed PR is
   invariant-bearing (INV-10), which argued for keeping it in core — but by
   the format-vs-service rule (DR-0015) PR upsert is a vendor service call
   like any other, and it targets the *consumer's* SCM, a platform fact the
   engine otherwise would not need to know. The invariant-bearing parts
   (deterministic branch name, body composition, never-merge) stay in core
   and in the protocol's obligations; only the API call moves out.
4. Reference handlers for GitHub and Azure DevOps ship in the framework
   release itself — first-class support is preserved, as shipped-and-tested
   handlers rather than compiled-in clients. A journal handler is the test
   kit's assertion point.
5. Upstream reads (tag listing, file-at-tag) move to the **git protocol**
   (`core/upstream.py`) — no vendor API and therefore no handler needed.

The engine ends up vendor-service-free: git + filesystem + subprocess.

## Alternatives considered

- **Status quo (service methods on the port).** Simplest, and the port did
  isolate the calls — but the binary's feature list becomes a vendor list,
  and every new ticket system is an engine change. Rejected.
- **A second CLI holding all platform REST.** Cleaner-sounding separation,
  but it splits the invariant-bearing sync sequence across two versioned,
  separately pinned artefacts and doubles the distribution story, for no
  isolation the exec protocol doesn't already provide. Rejected.
- **PR delivery stays in core as a bounded exception.** Defensible
  (PRs are the product's central mechanism), but it would keep two vendor
  REST clients in the engine forever and require a consumer-SCM detection
  path that the mixed case (Azure Pipelines building a GitHub repo) gets
  wrong. Rejected in favour of protocol symmetry.

## Consequences

- Adding a delivery target is a consumer-side config edit plus one
  executable; the engine and its invariants are untouched (INV-8 becomes
  stronger: the core is identical everywhere because it does nothing
  platform-specific).
- Unwired handlers are a defined, visible state (`pr-delivered=false` +
  intent emitted; `handoff=unwired`) — which is exactly the fully-manual
  `ci: none` mode, not a degraded one.
- The sync lane's end-to-end promise now spans two processes; the scenario
  kit therefore asserts through the journal handler, and the protocol
  version field guards against contract drift.
- The port interface shrinks to the CI output surface (emit output/summary/
  error) — the only in-process platform adaptation left.
