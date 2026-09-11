---
id: VK-FS-security-model
title: "The trust boundary and what guards it"
status: current
status_since: "2026-07-08"
last_verified: "2026-07-08"
spec_layer: functional_spec
summary: "Write access to a publisher plus release capability is code execution in every consumer, and the framework makes that boundary explicit, narrow and tamper-evident rather than pretending otherwise."
relations:
  regulated_by:
    - VK-STD-invariants
---

# Spec: Security model

## Scope

This document states the security guarantees a consumer may rely on, the
threats they answer, and the risks that remain accepted. It is cross-cutting:
it constrains every other specification here rather than describing one
component, and it is frozen public API at v1.0.0.

Threats considered: upstream tag substitution, hand-edits to vendored files
whether malicious or accidental, credential theft and rot, a compromised
publisher account, and abuse of the PR machinery through auto-merge or silent
scope growth.

## Behavior

### 1. Trust boundary — stated plainly

**Write access to a publisher repository plus the ability to cut its releases
is code execution in every consumer's CI.** Consumers vendor executable tools
and run pinned pipeline components from the publisher tree. VendKit does not
pretend otherwise; it makes the boundary explicit, narrow, and tamper-evident:

- Consumers adopt a release only by advancing a pin in a **reviewed PR**
  (INV-10) — a malicious release still has to pass human review of its diff at
  every consumer, and the sync PR shows every changed byte in-tree.
- Everything executed at the consumer is either (a) the pinned framework
  component resolved from an immutable tag, or (b) a manifest-tracked,
  gate-verified vendored file. There is no third channel: migration
  obligations and conformance extensions are declarative or `tool`-kind
  (vendored executables), **never inline shell strings from upstream**
  (migrations spec §5).

Threats considered: upstream tag substitution, hand-edits to vendored files
(malicious or accidental), credential theft/rot, a compromised publisher
account, PR-machinery abuse (auto-merge, silent scope growth).

### 2. Tag integrity — prevention

Tags are immutable by contract (INV-5) but not by Git. Per platform:

- **GitHub:** repository ruleset protecting `refs/tags/v*`: creation restricted
  to the release workflow's identity; update/delete denied to everyone.
- **ADO:** ref-level security on `refs/tags/*`: Create Tag restricted to the
  release pipeline's build identity; Force Push / Delete denied (ADO has no
  ruleset concept; use explicit ref permissions).

Both are non-tree-decidable → publisher-side conformance `attest` rules with
API upgrade (conformance spec §4). The release pipeline itself is
manual-trigger only.

### 3. Tag integrity — detection

Prevention can be misconfigured; detection is therefore layered in
(releases spec §6): the consumer manifest records `source.commit` at sync;
watch and sync independently verify that the pinned tag still resolves to it.
A mismatch is `tag-moved` — the highest-severity finding, raising an integrity
work item and refusing further syncs of that slice until resolved. Retraction
(not deletion) is the sanctioned way to withdraw a bad release, precisely so
deletion remains unambiguous evidence of trouble.

### 4. Credential model

Four purposes (platform-integration spec §3): read-upstream and push-branch
are ordinary **git credentials** spent by git itself; open-PR and work-items
are API tokens spent by the **handlers** that deliver those intents.
Principles:

- **Least scope per purpose** — never one broad PAT for everything. Prefer
  platform-native identities (GitHub App with narrow permissions;
  ADO build-service identity with explicit grants) over PATs; PATs rot.
- **Fail loud** — a missing/expired credential is a red pipeline, never a
  silent skip. The scaffolded *credential liveness probe* exercises each
  purpose read-only on the watch schedule, so expiry surfaces on cadence, not
  at the next release.
- **Consumer-held only** — no publisher-held write credentials into consumers.
  The one relaxation is the optional GHA push-hint dispatch token
  (`VENDKIT_TOKEN_PUSH_HINT`, spent by the `push-hint` handler —
  platform-integration spec §4), which is dispatch-scoped, opt-in, and does
  not bypass any review: `repository_dispatch` only *triggers* the consumer's
  existing sync pipeline, which still opens a reviewed PR. A lost or refused
  hint costs latency, not correctness.
- The sync PR credential deliberately **must satisfy branch review, not bypass
  it**: exemptions (ADO policy allowances) apply only to *creating* the PR,
  never to merging it.

### 5. Machinery abuse resistance

- **No auto-merge anywhere** (INV-10). The framework never carries a merge
  capability, so it cannot be confused into using one.
- **Scope is review-gated** (INV-4): reconcile additions and upstream removals
  surface as PR content; a compromised publisher cannot silently widen what a
  consumer executes.
- **Gate self-protection:** `.vendkit/**` (manifests, configs) and the gate
  pipeline itself should require owner review — CODEOWNERS on GitHub,
  a required-reviewers branch policy on Azure Repos (core conformance rule
  `control-plane-owned`, waivable with a recorded reason) — so disabling the
  gate is itself a reviewed, owner-approved change.
- **Disjointness (INV-7)** prevents a second slice from overwriting another
  slice's files as a smuggling path.

### 6. Supply-chain posture of the framework itself

The framework repo is its own publisher (self-hosted): its releases are cut by
its own release command behind the same tag protections, its tree is
gate-verified, and its conformance core rules apply to it. Dependency policy:
Layer 0 consumer path is stdlib-only (INV-9); publisher/sync paths may use one
pinned YAML library; the reference handlers use stdlib HTTP. No transitive
dependency sprawl — the machinery that guards supply chains must itself be a
minimal one. Handlers are consumer-configured executables and sit inside the
consumer's trust boundary like any pipeline step (handler-protocol spec §7).

### 7. Residual risks (accepted, documented)

- A consumer that rubber-stamps sync PRs gets upstream code with one click.
  Mitigation is social (review culture) plus the small, readable diffs that
  identity-copy vendoring produces.
- The engine executes from the *target* checkout during sync (INV-6) — i.e.
  target-release code runs before its PR is reviewed, in the sync pipeline's
  context. Scope-limit that pipeline's credentials (push-branch + open-PR
  only); with tag protection + provenance checks, the exposure equals "the
  publisher released it", which is the §1 boundary, not an extra channel.
- Platform APIs (required-check status, policy configuration) can be
  misconfigured out-of-band; conformance attestation + API verification
  narrows but cannot eliminate this.

## Acceptance

Every control below is either decidable from the consumer tree or attested
and API-verifiable. That split is what keeps the model checkable rather than
aspirational.

- **Tree-decidable controls are conformance rules.** Gate wiring, manifest
  tracking, pin immutability and `.vendkit/**` ownership under CODEOWNERS are
  read from the tree by `vendkit conformance`.
- **Platform controls are attested, then verified.** Branch protection, build
  validation policies and required reviewers are invisible in-tree on both
  platforms, so they are asserted in the slice config and upgraded to facts
  by the fact-verify handler.
- **Tag immutability is checked from both ends.** Prevention is ref
  permissions (§2); detection is the recorded `source.commit` compared on
  every watch and sync (§3). Prevention can be misconfigured, which is why
  detection is not optional.
- **Credential liveness is exercised on a cadence.** The scaffolded probe
  spends each purpose read-only on the watch schedule, so expiry surfaces
  then rather than at the next release.
- **The residual risks in §7 are accepted, not mitigated.** They are listed
  so that a reader can tell a known limit from an oversight.
