---
id: DR-0021
title: "The documentation is a governed corpus, typed and indexed by a pinned Headwater"
status: current
status_since: "2026-09-11"
last_verified: "2026-09-11"
summary: "The design records, the component specifications and the standards that bind them become one typed corpus checked in CI by a pinned Headwater binary, with the specifications split into functional and technical layers and every shelf index generated."
---

# DR-0021 — The documentation is a governed corpus, typed and indexed by a pinned Headwater

## Context

Two properties of `docs/design/` were asserted by convention and held by
nobody, and `docs/specs/` had no stated structure at all.

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

`docs/specs/` had a third problem, and a larger one. Twelve documents sat in
one flat directory with no stated relationship between them, and nothing said
which were contracts and which were mechanism. A reader asking "what may I rely
on" and a reader asking "how is this built" opened the same directory and got
the same undifferentiated list. Their headings were bespoke and numbered per
document, so nothing could check that a specification stated its scope, and
most of them never said how anyone would know the specification held.

The invariants were the sharpest case of the same problem. Ten numbered
properties carry the whole design, and the rest of the corpus cites them by
number a hundred times — but they lived as section 3 of an architecture
overview, which is orientation rather than specification. The most depended-upon
content in the repository was the least governed, and nothing stated where each
invariant was actually held.

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

5. **The component specifications take the functional/technical ladder.**
   `docs/specs/` becomes a second governed shelf, heterogeneous and
   discriminated by the `spec_layer` facet, so one directory holds both rungs
   and the facet says which each document is. A `functional_spec` states what
   a caller may rely on and requires Scope, Behavior and Acceptance; a
   `technical_spec` states how that is realized and requires Scope, Design and
   Conformance. Seven documents are functional — the command surface, the
   handler protocol, the export declaration, migrations, conformance, releases
   and the security model — and five are technical: the manifest and gate, the
   sync lane, release watch, platform integration and onboarding.

   The prose is refactored to fit, because the section requirements of a
   bundle kind cannot be relaxed: an overlay `override` does not commute with
   the `add` a bundle uses to declare a kind, so the sections are what the
   package says they are. Every existing section keeps its number and its
   position and is demoted one level, so the section references other specs
   make (`conformance spec §4`) keep resolving; what is new is a Scope lead-in
   and a closing Acceptance or Conformance section.

   Where a technical specification realizes a functional one, the pair is
   declared as a reciprocal `realizes` edge. `docs/architecture.md` stays
   excluded with a stated reason: it orients a reader across the whole system
   rather than specifying one component, and no kind in this package fits it.

6. **The standards that bind the specifications are governed too, and the
   corpus is the repository rather than `docs/`.** A `standard` sits above the
   ladder, requires Scope, Requirements and Conformance, and `regulates` the
   specifications it binds. Two documents are standards here.

   The ten invariants move out of `docs/architecture.md` §3 and become
   `docs/standards/invariants.md`. They are cited by number a hundred times
   across this corpus and by location twice, so the thing most depended upon
   was the thing least governed: it sat inside a document this taxonomy
   excludes. Section 3 of the architecture overview stays, as a pointer, so a
   reader arriving from "architecture §3" still lands on the numbers. The
   Conformance section is new and states where each invariant is actually
   held.

   `COMPATIBILITY.md` is a standard by every test the package applies, and it
   stays at the repository root where a reader and a host expect it and where
   six documents already link to it. Reaching it means the corpus root is the
   repository, which costs a `./` prefix on every shelf and exclusion path and
   an exclusion for the vendored package's own prose. That is the price of not
   moving a file for a tool's convenience, and it is worth paying once.

   Its shelf is deliberately absent from `projections`: a shelf index for a
   shelf whose directory is the repository root would be written to
   `./README.md`, over the front page.

7. **The testing strategy joins the specifications as a technical spec.** It
   states how the framework is tested, which is a realization and not a
   contract, so it moves to `docs/specs/testing.md` and takes Scope, Design and
   Conformance. It declares no `realizes` edge because it realizes no single
   contract — it verifies all of them — and is instead `regulated_by` the
   invariants, which every tier in it exists to make executable.

8. **Existing records keep their prose.** Accepted DRs are immutable, so the
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

- **Govern the specs with the base `specification` kind and relaxed
  sections.** The base kind requires only Scope and Behavior, and unlike a
  bundle kind it *can* be overridden from the overlay, so its section list
  could have been emptied to match the headings the specs already had. It was
  measured: doing so leaves the corpus with front matter and nothing else,
  because the ladder is where the value is. No `spec_layer` discriminator, no
  `realizes` edge between a contract and its realization, and no `regulates`
  edge from a standard. That is governance theatre — a checker that types
  documents it cannot say anything about — so the editorial cost of the real
  sections was taken instead.

- **Adopt the `standard` kind for the security model too.** It would fit: the
  document is cross-cutting and constrains the others. It stays a
  `functional_spec` because it reads as a statement of what a consumer may rely
  on, which is what that kind is for, and because moving it to the standards
  shelf would rewrite every cross-reference to it for a distinction the
  `regulates` edges already draw.

- **Adopt the Diátaxis bundle.** Rejected on the package's own evidence. Its
  central obligation — that a document is in one reader mode — is declared
  `unverifiable` there, on the grounds that deciding a page serves two readers
  means reasoning about what the prose asserts. No check reads the facet. The
  value set catches a misspelled mode name and nothing else, so adoption buys a
  label with no enforcement behind it.

- **Adopt `obligation_record` for the invariants or the roadmap.** The
  obligation lifecycle ends in discharge, and an invariant is never discharged;
  a document saying INV-1 is waiting to be paid off would be false. The
  roadmap's outstanding items are decisions not to act, with nothing in flight,
  so each would become a document with a Discharge section describing work
  nobody is doing.

- **Adopt the business- and product-requirement kinds, or the design-spec
  registers.** They model documents this repository does not have, and require
  `doc_type` and `sequence` facets that would be invented here rather than
  recorded.

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
  not. `docs/specs/README.md` is generated on the same terms, which is an
  index the specs shelf never had at all.

- Every specification now states its layer, and the ladder is load-bearing
  rather than decorative: a reader who wants the contract reads a
  `functional_spec`, and one who wants the mechanism follows its `realizes`
  edge down. A functional spec that nothing realizes raises a warning after
  ninety days, which is the corpus asking whether a requirement was ever
  built. Migrations and the security model sit unrealized today, honestly, and
  will raise that warning.

- The invariants are now a document rather than a section, with a stated place
  where each one is held. Extracting them cost two reference updates, because
  the corpus cited them by number rather than by location — which is the
  measure of how load-bearing the numbers had quietly become.

- The corpus root is the repository, so a document added anywhere is walked and
  must be typed or excluded with a reason. That is stricter than scoping to
  `docs/`, and it is the point: a governed corpus that stops at a directory
  boundary governs whatever happens to be inside it.

- Each specification now carries an Acceptance or Conformance section, which
  is the part most of them lacked. Writing them surfaced what was already
  true and merely unstated: that the command surface is locked by a frozen
  snapshot test, that a migration is done when the verifier says so, and that
  "fully onboarded" means `vendkit conformance --strict` passing.

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
