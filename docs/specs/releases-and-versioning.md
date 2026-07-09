# Spec: Releases and versioning

Status: draft for implementation · Owner: Layer 0 (+ Layer 1 for ref listing)

## 1. The tag is the release

A release is an **annotated Git tag** `vMAJOR.MINOR.PATCH` on the publisher
repository. There is no artefact, package, or release object: the pinned tree
at the tag — including the committed manifest, export declaration, conformance
spec and migrations — *is* the payload. (On GitHub, creating a Release object
from the tag is a permitted nicety for humans; tags remain canonical and are
what watch reads.)

## 2. Version grammar and ordering

- **Stable:** `v` + strict `MAJOR.MINOR.PATCH` (no leading zeros).
- **Pre-release (rc channel only):** `vMAJOR.MINOR.PATCH-rc.N`, N ≥ 1.
- Ordering: SemVer §11. Anything not matching the grammar is invisible to the
  machinery (not an error — publishers may tag other things).

Semantics for a *content* framework, declared not inferred:

| Bump | Meaning |
|---|---|
| PATCH | Refresh-only: no export-surface additions/removals, no migrations |
| MINOR | Surface may grow; additive migrations at most |
| MAJOR | Surface may shrink/reshape; judgment-bearing migrations expected |

The release command **computes** surface deltas against the previous release
(diff of export sets) and refuses a bump smaller than the delta implies
(e.g. removals demand MAJOR). This turns the table from convention into a gate.

## 3. Cutting a release

`vendkit release --bump patch|minor|major | --version vX.Y.Z [--summary <text>]`

1. **Freshness pre-gate:** `generate --check` must pass (committed manifest
   matches the tree). Refuse otherwise.
2. **Migration pre-gate:** if the surface delta or any changed adapter implies
   consumer-owned reshaping (MAJOR path), require at least one `migrations/`
   entry with `applies_from` = the new version, or an explicit
   `--no-migrations-needed` override recorded in the tag annotation.
3. Determine the latest existing release from remote tag listing; a listing
   failure is a hard error (never compute from an unknown baseline).
4. Enforce: target strictly newer than latest; tag does not already exist
   (local **and** remote).
5. Create the annotated tag (annotation carries summary, surface-delta counts,
   bump class) and push it. The remote's ref rejection is the serialisation
   point for concurrent cuts — no locking needed.

The release pipeline is manual-trigger only, and the tag namespace must be
write-restricted to it (security model §2).

## 4. Retraction

A shipped release found harmful is **retracted, not deleted**: add its version
to `retracted:` in the export declaration and cut a new (patch) release.
Consumers' watch skips retracted versions when computing LATEST; sync
refuses a retracted TARGET with a distinct exit (`refused=retracted`).
Deleting a tag is never the mechanism — pinned consumers must keep resolving
their pin (INV-5), and deletion is indistinguishable from tampering.

Note the bootstrapping quirk: retraction data lives at HEAD-of-latest-release,
so watch reads the *newest* release's declaration for the retraction list, not
the pinned one.

## 5. Channels

A consumer follows a channel per slice (slice config, default `stable`):

- `stable` — only `vX.Y.Z` tags exist for LATEST computation.
- `rc` — pre-release tags also qualify. Intended for canary consumers that
  absorb a release before the fleet (ring rollout). An `rc` consumer's sync
  targets the rc tag exactly like a release; the subsequent stable tag
  supersedes it.

Publishers are not obliged to cut rc tags; the channel mechanism simply makes
them adoptable when they exist.

## 6. Provenance

At sync time the consumer records `source.commit` — the SHA the target tag
resolved to (manifest spec §1). Verification duties:

- **sync (next run):** if the *pinned* tag no longer resolves to the recorded
  SHA, fail loudly (`refused=tag-moved`) — do not materialise from a
  substituted tree.
- **watch:** same check, surfaced as a `tag-moved` finding (highest severity).

This is the detection half of tag immutability; the prevention half is ref
permissions (security model §2).
