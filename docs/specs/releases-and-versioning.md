---
id: VK-FS-releases-and-versioning
title: "Releases, version grammar and retraction"
status: current
status_since: "2026-07-08"
last_verified: "2026-07-08"
spec_layer: functional_spec
summary: "An annotated SemVer tag is the entire release, its bump class is computed from the export-surface delta rather than asserted, and a harmful release is retracted rather than deleted."
relations:
  realized_by:
    - VK-TS-release-watch
  regulated_by:
    - VK-STD-invariants
---

# Spec: Releases and versioning

## Scope

This document states what a release is, how versions are written and ordered,
what a consumer may rely on when it adopts one, and how a bad release is
withdrawn. It is owned by Layer 0, with Layer 1 supplying the tag listing,
and is frozen public API at v1.0.0.

The release mechanism is deliberately small: there is no artefact registry
and no release object to keep in step with the tree. What the sections below
add to a bare Git tag is a declared meaning for each bump class, a gate that
enforces it, and a provenance record that makes tag substitution detectable.

## Behavior

### 1. The tag is the release

A release is an **annotated Git tag** `vMAJOR.MINOR.PATCH` on the publisher
repository. There is no artefact, package, or release object: the pinned tree
at the tag — including the committed manifest, export declaration, conformance
spec and migrations — *is* the payload. (On GitHub, creating a Release object
from the tag is a permitted nicety for humans; tags remain canonical and are
what watch reads.)

### 2. Version grammar and ordering

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

### 3. Cutting a release

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

### 4. Retraction

A shipped release found harmful is **retracted, not deleted**: add its version
to `retracted:` in the export declaration and cut a new (patch) release.
Consumers' watch skips retracted versions when computing LATEST; sync
refuses a retracted TARGET with a distinct exit (`refused=retracted`).
Deleting a tag is never the mechanism — pinned consumers must keep resolving
their pin (INV-5), and deletion is indistinguishable from tampering.

Note the bootstrapping quirk: retraction data lives at HEAD-of-latest-release,
so watch reads the *newest* release's declaration for the retraction list, not
the pinned one.

### 5. Channels

A consumer follows a channel per slice (slice config, default `stable`):

- `stable` — only `vX.Y.Z` tags exist for LATEST computation.
- `rc` — pre-release tags also qualify. Intended for canary consumers that
  absorb a release before the fleet (ring rollout). An `rc` consumer's sync
  targets the rc tag exactly like a release; the subsequent stable tag
  supersedes it.

Publishers are not obliged to cut rc tags; the channel mechanism simply makes
them adoptable when they exist.

### 6. Provenance

At sync time the consumer records `source.commit` — the SHA the target tag
resolved to (manifest spec §1). Verification duties:

- **sync (next run):** if the *pinned* tag resolves to a SHA other than the
  recorded one, fail loudly (`refused=tag-moved`) — do not materialise from a
  substituted tree.
- **watch:** same check, surfaced as a `tag-moved` finding (highest severity).

This is the detection half of tag immutability; the prevention half is ref
permissions (security model §2).

## Acceptance

The version grammar is only worth as much as the refusals behind it, and the
release command is where they sit.

- **The bump class is computed, not claimed** (§2). The release command
  diffs export sets against the previous release and refuses a bump smaller
  than the delta implies, which turns the bump table from a convention into
  a gate.
- **Two pre-gates run before a tag exists** (§3). The freshness pre-gate
  requires `generate --check` to pass, so a release cannot be cut from a
  stale manifest. The migration pre-gate requires a matching payload for a
  consumer-reshaping change, or an override recorded in the annotation.
- **An unknown baseline is a hard error.** A failure to list remote tags
  stops the cut rather than computing a version from an unknown latest.
- **The remote is the serialisation point.** Concurrent cuts resolve by the
  remote's ref rejection, so no locking is needed and no cut silently wins.
- **Provenance is checked on every later run** (§6). Sync refuses
  `tag-moved` and watch raises it as the highest-severity finding, which is
  the detection half of tag immutability.
