# Security Policy

## Reporting a vulnerability

**Please report suspected vulnerabilities privately — do not open a public
issue.**

Use GitHub's private vulnerability reporting: on the repository's
**Security** tab, choose **Report a vulnerability**. This opens a private
advisory visible only to the maintainers and you.

Please include, as far as you can:

- the affected version(s) or commit,
- which side is affected — **publisher** (release/export) or **consumer**
  (gate/sync/watch) — and the `scm` / `ci` axes if relevant,
- a minimal reproduction, and
- the impact you believe it has.

You can expect an acknowledgement within a few days. We will confirm the
issue, agree a disclosure timeline with you, and credit you in the advisory
unless you'd rather stay anonymous.

## Supported versions

VendKit is pre-1.0. Until the 1.0 release, security fixes land only on the
**latest** release; there is no back-porting to earlier `v0.x` tags. Upgrade
to the newest release to receive fixes. Once 1.0 ships, this section will
state the supported range.

## Scope — what "a vulnerability" means here

The trust boundary is stated plainly in the threat model,
[docs/specs/security-model.md](docs/specs/security-model.md): **write access
to a publisher repository plus the ability to cut its releases is code
execution in every consumer's CI.** VendKit does not pretend otherwise — it
makes that boundary explicit, narrow, and tamper-evident (reviewed-PR
adoption, immutable SHA-anchored tags, the manifest gate). Read that spec
before reporting anything that touches credentials, tags, or the PR
machinery.

In scope — a break in one of the guarantees the tool claims, for example:

- a hand-edit or deletion of a vendored file that the **gate lane** fails to
  catch (INV-1 / gate integrity),
- upstream **tag substitution** that the consumer's recorded commit SHA does
  not make detectable (INV-5),
- silent **scope growth** or an auto-merge — any path by which machinery
  mutates consumer-owned content without a reviewed PR (INV-10),
- execution of **inline upstream shell** through a migration or conformance
  extension (the security model forbids this channel),
- a normalisation or checksum flaw that lets a real content change masquerade
  as no-drift.

Out of scope — the boundary the threat model explicitly accepts: a malicious
publisher with legitimate release access whose poisoned release is still
surfaced for human review in a sync PR at every consumer. That is the
documented, accepted risk, not a vulnerability in the tool.
