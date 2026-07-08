# DR-0010 — Scope changes are reviewed PR events, never automatic

- **Status:** accepted
- **Date:** 2026-07-08

## Context

Over a slice's life the publisher's export surface grows and shrinks. Consumer
scope could track it automatically (always vendor everything currently
exported) or manually (scope frozen until someone acts). Automatic tracking
means a publisher release silently adds files to — or deletes files from —
every consumer tree; frozen scope means consumers rot away from the surface.

## Decision

The tracked slice recorded in the consumer manifest is the unit of consent.
Default sync is a **refresh of exactly that slice** (INV-4). Growth happens
only via `--reconcile-scope`, which stages additions *inside the sync PR* where
they are individually visible and rejectable; the scaffolded sync uses it so
growth is offered by default but always through review. Shrinkage is
report-only: files removed upstream leave the manifest but **stay on disk**
until the PR (or a human) deletes them. Nothing in the framework deletes a
consumer file, ever.

## Alternatives considered

- **Auto-track the full surface.** A compromised or careless publisher writes
  arbitrary new files into every consumer on next sync with no per-file
  consent; deletion symmetric and worse.
- **Frozen scope with out-of-band adoption.** New shared content requires
  manual vendoring per consumer — precisely the toil the framework should
  remove; in practice scope freezes forever.
- **Auto-delete on upstream removal.** A publisher mistake (or glob typo)
  cascades into fleet-wide file deletion in a merged PR titled "routine sync".
  Deletion must look like deletion in review.

## Consequences

- Sync PRs carry three visibly distinct classes (`updated`,
  `added`, `removed-upstream`) — report format is part of the review UX, not
  cosmetics.
- Profiles' `export_slice` bounds what reconcile may offer each archetype.
- A consumer can deliberately run a narrowed slice indefinitely; conformance
  may advise but never forces widening.
