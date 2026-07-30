# DR-0019 — `.vendkit/` has two halves: `consumer/` and `publisher/`

- **Status:** accepted
- **Date:** 2026-07-30
- **Supersedes:** refines [DR-0012](DR-0012-consumer-config-consolidation.md) (the
  per-slice two-file rule holds; the directory gains a `consumer/` level)

## Context

`.vendkit/` held the consumer control plane — slice configs and vendored
manifests — discovered by two non-recursive globs: `.vendkit/*.yml` and
`.vendkit/*-manifest.json`. The first is strict by design: every YAML file there
must parse as a slice config, and a stray one is a usage error rather than a
silent skip (DR-0012).

That strictness made the directory unusable for anything else, including a
publisher's own configuration. Publishers therefore kept their export
declaration, generated manifest, conformance rule set, and subscribers file at
the repository root — five machine-read files in the most-read directory in the
repo, and no way to group them without inventing a second convention per
publisher. A repository that both publishes and consumes had no way to express
that; its own manifest, dropped into `.vendkit/`, would have been discovered as a
vendored slice manifest and compared against its own tree.

A downstream publisher hit this directly: it had grouped its machine-read
configuration into a directory of its own and found three of the five files were
engine contract, not its choice.

## Decision

`.vendkit/` has two halves, and no glob reaches across them:

| Half | Holds | Owner |
|---|---|---|
| `.vendkit/consumer/` | `<slice>.yml`, `<slice>-manifest.json` | consumer (config) / engine (manifest) |
| `.vendkit/publisher/` | `export-declaration.yml`, `<slice>-manifest.json`, `conformance-rules.yml`, `subscribers.yml` | publisher |

The publisher-side paths become **defaults**, not merely permitted values:
`--export-decl` defaults to `.vendkit/publisher/export-declaration.yml`,
`publisher.manifest_dir` defaults to `.vendkit/publisher` (an explicit `"."` opts
back out to the repo root), and `conformance --rules` resolves
`<publisher-root>/.vendkit/publisher/conformance-rules.yml` when the flag is
absent.

Two supporting changes ride along:

- **`slice.aliases`** — a publisher's self-declared names, copied into each
  consumer's slice config at onboard and sync time. A consumer's set of known
  upstreams is then the union of what its upstreams say about themselves, so
  tooling that reasons about cross-repository references never needs a
  hand-maintained registry of the whole family, and no repository declares
  another's identity.
- **A `rules=core-only|core+publisher` fact** on every `conformance` run.
  `--rules` having no default meant a consumer could evaluate only the embedded
  core rules and read the clean result as "the publisher's slice rules passed".
  An explicit `--rules` that cannot be read remains a hard error; a missing
  conventional file is skipped, but never silently.

## Consequences

- **Positive:** publisher and consumer roles are legible in one root, and a
  repository can hold both without collision. Publishers get a home for their
  machine-read configuration without inventing a convention. The defaults remove
  four flags from a typical publisher's call sites. Conformance can no longer
  silently evaluate half its rule set.
- **Negative:** MAJOR, with a migration every existing consumer must apply. The
  publisher manifest moves out of the repo root, so any path filter, CODEOWNERS
  entry, or branch policy naming it directly needs updating — entries scoped to
  `.vendkit/` as a whole still cover both halves.
- **Neutral / operational:** ships as v2.0.0 with migration
  `2026-07-vendkit-dir-split`, whose obligations `migrations-verify` enforces.
  VendKit's own repository moves with the change (self-hosting).

## Alternatives considered

### Keep the publisher's files at the repo root

The status quo. Rejected: it forces every publisher to accumulate machine-read
files in its most-read directory, and offers no answer at all to a repository
that both publishes and consumes.

### Let publishers choose any directory, with no default

Shipped in v1.1.0 as `publisher.manifest_dir` plus explicit `--export-decl` and
`--rules`. Rejected as the end state: it works, but every publisher invents its
own layout, every call site must pass flags, and a forgotten flag is silent —
which is exactly how the conformance rule set came to be skipped in practice.

### One flat `.vendkit/` with filename conventions

E.g. `publisher-*.yml` beside `<slice>.yml`. Rejected: the strict-namespace rule
over `.vendkit/*.yml` would have to weaken to a prefix test, trading a hard error
for a heuristic on the one surface where a silent skip is most expensive.
