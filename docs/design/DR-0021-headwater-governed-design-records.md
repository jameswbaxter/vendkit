---
id: DR-0021
title: "Design records are a governed corpus, typed and indexed by a pinned Headwater"
status: current
status_since: "2026-09-11"
last_verified: "2026-09-11"
summary: "The design-record shelf becomes a typed corpus checked in CI by a pinned Headwater binary, which also generates the shelf index the repository used to maintain by hand."
---

# DR-0021 — Design records are a governed corpus, typed and indexed by a pinned Headwater

## Context

Two properties of `docs/design/` were asserted by convention and held by
nobody.

The first is the shelf index. `docs/design/README.md` carried a hand-written
table of every DR and its status. Nothing compared that table to the shelf, so
a DR added without a table row, or a status changed in a record but not in the
table, was invisible until a reader noticed. DR-0018 and DR-0020 both shaped
the docs pipeline and neither addressed index staleness, because the pipeline
renders Markdown and has no model of what a design record *is*.

The second is the shape of a record itself. `TEMPLATE.md` states the form —
Context, Decision, Alternatives considered, Consequences, a status, a date —
and the shelf had already drifted from it. DR-0014 through DR-0018 carry a
`Status:` line in body prose instead of the template's bullet header, and none
of the five records a date at all; their dates survive only in git history.
Three records carry a `Supersedes:` line whose meaning is partial — one
*refines*, one *amends*, one supersedes a named part of an earlier record —
and no reader or tool could tell those from a full replacement.

Headwater is a taxonomy-driven typing and validation engine for documentation
corpora. Its `decision-record` bundle matches this shelf's tradition closely:
one decision per document, Nygard's three sections, immutable once accepted,
amended by succession. Issue #20 evaluated it in 2026-09 and deferred on one
blocker — no release binary existed, only a `cargo build` from source. Version
0.1.2 publishes a checksummed per-platform binary, so the blocker is gone.

## Decision

1. **`docs/design/` becomes a governed corpus.** Every `DR-NNNN` file carries
   YAML front matter stating `id`, `title`, `status`, `status_since`,
   `last_verified` and `summary` — the base `governed_document` requirement
   plus the name the generated index renders. `README.md` and `TEMPLATE.md` are
   declared exclusions, each with a stated reason: one is a projection of the
   shelf, the other is the blank form.

2. **The taxonomy is vendored and pinned, the engine is fetched and verified.**
   `headwater/standard` 4.2.0 and its `decision-record` and
   `evidence-and-obligation` bundles sit in `packages/` as identity copies,
   admitted against the digest the publisher printed (DR-0001). The engine
   itself is a binary artefact pinned by version and SHA-256 in
   `tools/headwater-fetch.sh` and fetched at CI time — the same shape DR-0016
   gives the VendKit engine. `.headwater/taxonomy.lock` is committed, and CI
   re-resolves it and fails on a diff.

3. **Headwater runs as a check and a generator, never as the site build.**
   `internal/docsgen` still renders the docs site (DR-0018) and Headwater is
   never imported by `cmd/vendkit` or reached from the consumer gate path
   (DR-0011). CI runs `taxonomy validate`, `check --strict` and
   `generate --check`. The worst failure this can produce is a red docs check,
   never a broken binary or an unbuildable site.

4. **The shelf index is generated.** `headwater generate` writes
   `docs/design/README.md` from the corpus, carrying each record's title and
   its `summary`, and `generate --check` fails CI when the committed file and
   the shelf disagree. The hand-maintained status table is retired.

5. **The scope is the design shelf only.** `docs/specs/` stays ungoverned:
   those documents map only loosely onto the `design-spec` and
   `standards-spec` bundles, and a taxonomy fit for that shelf is a separate
   decision to be taken after this one has run for a while.

6. **Existing records keep their prose.** Accepted DRs are immutable, so the
   backfill adds front matter and removes the header lines that the front
   matter now states, and changes no argument. Every record is `current`: the
   three partial `Supersedes:` lines are kept as prose and declared as no
   succession edge, because an edge would assert a full replacement that did
   not happen. Four `voice.forbidden_construction` warnings stand unfixed for
   the same reason — they are warnings, they do not gate, and silencing them
   would mean editing the prose of accepted records.

## Alternatives considered

- **Keep the hand-maintained table and add a bespoke checker.** A short Go
  program in `internal/docsgen` could compare the table to the directory
  listing. It closes the narrower half of the gap — a missing row — and closes
  none of the wider one, because a status, a date or a section that drifted is
  invisible to a program that only counts files. It is also a second
  docs-shaped tool to own.

- **Adopt Headwater across all of `docs/`.** Rejected on fit rather
  than on principle. The specs shelf would need taxonomy adaptation the design
  shelf does not, and adapting a taxonomy while also learning what the engine
  does in CI conflates two risks. Point 5 holds the question open.

- **Build the engine from source in CI.** This is what 0.1.0 forced and what
  deferred the decision in #20. A Rust toolchain in the docs job reintroduces
  exactly the interpreter-and-lockfile weight DR-0018 rejected mkdocs and
  Docusaurus for. The pinned binary is what makes the answer different this
  time.

- **Let Headwater render the docs site.** It does not render its own site with
  itself; it is a typing engine, not a generator. Replacing `internal/docsgen`
  would trade a working pure-Go build-time tool for a dependency in the build
  path, and would make a docs finding into a site outage. Additive adoption
  keeps the failure mode small.

## Consequences

- The DR index cannot go stale without CI saying so, and it now carries each
  record's summary, so the shelf is scannable in a way the status table was
  not.

- A new DR has obligations a new file did not: front matter, the three
  required sections, and an identifier claim under `.headwater/ids/`. The
  engine writes the claim mechanically (`headwater check --fix`), and
  `TEMPLATE.md` carries the front matter so a record is born typed.

- Two pins now need maintenance: the engine version and digest in
  `tools/headwater-fetch.sh`, and the taxonomy version and digest in
  `.headwater/taxonomy.yml`. Both fail closed — a fetch whose bytes miss the
  digest fails, and an artifact that is not the pinned one is refused.

- `internal/docsgen` gained front-matter stripping, because goldmark carries no
  front-matter extension and would otherwise render the block as page content.
  That is a few lines of stdlib and no new module dependency, so DR-0018's
  dependency posture is unchanged.

- Headwater is pre-1.0 and its own README marks its measurement layer
  unfinished. The exposure is bounded by point 3: the tool gates documentation
  and nothing else, so the cost of it being wrong, or of abandoning it, is
  deleting a CI job and a directory of front matter.
