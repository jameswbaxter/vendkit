# DR-0005 — The tag is the release: immutable, SHA-anchored, retractable

- **Status:** accepted
- **Date:** 2026-07-08

## Context

Consumers pin a publisher version. The release medium determines the trust
story: what exactly does a pin resolve to, can it change underneath a fixed
pin, and how is a bad release withdrawn? Both target platforms natively resolve
git refs in their pipeline-reference primitives (ADO repository resources, GHA
`uses:`/checkout), and the sync engine materialises from a checkout of the
pinned tree.

## Decision

A release is an **annotated SemVer tag**; the tree at the tag is the entire
payload (manifest, declaration, migrations, conformance spec included). Tags
are never moved or reused (INV-5); enforcement is layered: release-command
refusal, platform ref protection, and consumer-side detection via the
`source.commit` SHA recorded in the manifest at sync (`tag-moved` refusal). A
bad release is **retracted** by declaration, never deleted — so tag deletion
remains unambiguous evidence of tampering. Surface-delta-aware bump rules and a
migration pre-gate make the SemVer classes enforceable rather than aspirational.

## Alternatives considered

- **Release artefacts (archives/packages).** A second distribution channel to
  secure and version; splits provenance between tag and artefact; DR-0001
  already rejected registries.
- **Branch or floating alias pins (`v1`, `latest`).** A fixed pin whose bytes
  change is precisely the vulnerability class this framework exists to close.
- **Commit-SHA-only pins.** Maximally tamper-proof but human-opaque; no SemVer
  ordering for watch/migration windows. Adopted *in combination*: the tag is
  the human/ordering handle, the recorded SHA the integrity anchor.
- **Yank by tag deletion.** Breaks pinned consumers' resolution and is
  indistinguishable from an attack. Retraction lists keep resolution intact
  while steering upgrades away.

## Consequences

- Watch must read the retraction list from the newest release, not the pinned
  one (bootstrapping quirk, releases spec §4).
- Ref protection is platform-specific and non-tree-decidable → publisher
  conformance attest rules (security model §2).
- rc-channel tags extend the grammar without touching stable consumers.
