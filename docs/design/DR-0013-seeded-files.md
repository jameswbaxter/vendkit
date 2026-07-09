# DR-0013 — Seeded files: a scaffold-once lifecycle class

- **Status:** accepted
- **Date:** 2026-07-09

## Context

The framework knew two file classes: vendored identity copies (drift-gated,
refreshed by every sync) and consumer-owned files (invisible to the machinery).
Publishers also want to ship **starter files** — a `CONTRIBUTING.md` skeleton,
config templates, example docs — that a consumer receives once and then
customises. Treating these as vendored files makes every customisation drift;
leaving them out of the slice loses the hand-out entirely. A third class is
needed: materialised only when the target path does not exist, then owned by
the consumer and free to diverge — never clobbered.

## Decision

The export declaration gains a `seed:` glob surface, disjoint from `include:`
(overlap is a hard error — a path cannot be both drift-gated and
free-to-diverge; the publisher resolves with an exclude or a narrower glob).
Seeds flow through the same adapters and profile scoping as vendored files.

**The manifest entry is the seed's lifecycle record — no new state.** A
`seed: true` entry means "seeding happened here": sync never writes a path
that has an entry, whether the file currently exists or not, so a consumer's
deletion is respected forever. Seeding occurs only through scope
reconciliation for *untracked* paths (already a reviewed-PR event, DR-0010),
and adopts rather than writes when the consumer already has a file at the
target path. The entry's `sha256` records the **template's** hash, not the
consumer file's; the gate skips seed entries entirely (they still claim their
`consumer_path` for INV-7 collision detection). When the upstream template
later changes, sync reports `template_updated` — informational only, never
`changed`, surfaced in the next real sync PR, configurable per slice
(`seeds.notes: informational | silent`, default informational). Retiring a
template upstream drops the entry and leaves every consumer's copy untouched,
so seed removals are exempt from the MAJOR-bump and migration pre-gates.

## Alternatives considered

- **Re-seed when absent (stateless ensure-exists).** Simpler mental model,
  but a consumer could never permanently remove a seeded file without
  fighting every subsequent sync PR.
- **Separate seed-state file** (`.vendkit/<slice>-seeded.json`). A second
  registry to keep in lockstep with the manifest — exactly the multi-file
  drift DR-0012 exists to prevent, and it buys nothing the manifest entry
  doesn't already provide.
- **Seed as an adapter kind.** Adapters transform content; this is a
  *lifecycle* difference (when to write, what the gate checks, what release
  bumps imply). Modelling it as an adapter would smear lifecycle logic across
  the transform layer.
- **Gate-advisory divergence tracking.** Hashing consumer copies of seeds and
  reporting divergence as advisory findings would train consumers to ignore
  the gate. Divergence is the *intended* state; only template updates are
  worth a note, and only in PR bodies.

## Consequences

- INV-1 is unaffected: the gate skips exactly what sync refuses to write.
- INV-3 holds: seeding classification depends only on tree state, so check
  predicts apply.
- INV-7 extends to seeds: a seeded path still claims its `consumer_path`.
- Template hashes in consumer manifests refresh only when a sync PR actually
  ships (a template-only change never forces a PR), so a `template-updated`
  note can repeat across watch cycles until the next real sync — accepted.
- A publisher reclassifying a seed as vendored (moving the glob from `seed:`
  to `include:`) causes the next sync to overwrite consumer copies — a
  deliberate, PR-visible class change; publishers should ship it as a MAJOR
  with a migration note.
- Re-seed escape hatch: remove the manifest entry; the next reconcile
  re-offers the seed in a reviewed sync PR. No dedicated CLI surface.
