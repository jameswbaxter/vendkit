# DR-0015: Split the platform vocabulary into SCM and CI axes; formats not services

Status: accepted

## Context

`platform: github | ado` appeared in the export declaration, the slice
config, and manifest provenance — and in each place it silently stood in for
up to three different facts: where the repository is hosted (SCM: clone,
tags, PRs, branch policies), what runs the pipelines (CI: workflow dialect,
output conventions, pin-line shape), and where tickets live (work
tracking). "ado" was especially overloaded: Azure Repos, Azure Pipelines,
and Azure Boards are three products that need not travel together. The
mixed case — Azure Pipelines building a GitHub-hosted repository — was
unrepresentable, and runtime env-sniffing (`GITHUB_ACTIONS`/`TF_BUILD`)
picked the wrong axis for SCM-bound operations.

## Decision

1. **Two explicit axes, distinct value vocabularies.**
   - SCM: `github | azure-repos` — publisher coordinates
     (`publisher.scm`), manifest provenance (`source.scm`), and the
     consumer's own host (`scm:` in the slice config, written by init and
     inferred from the origin remote when not given).
   - CI: `github-actions | azure-pipelines | none` — recorded once as `ci:`
     in the slice config. `none` is first-class: no scaffolded pipelines,
     manual orchestration, the manifest's `source.release` is the pin.
   - Work tracking has **no axis**: no core config names a ticket system
     (DR-0014).
2. **The format-vs-service rule.** Core may understand vendor file
   *formats* — pipeline YAML dialects (conformance detectors), CODEOWNERS
   syntax, CI output conventions, clone-URL templates. Core must not call
   vendor *services* — those live in handlers. This is the line that makes
   "is X coupled?" decidable for every future feature.
3. **Upstream reads over the git protocol.** Tag listing (`ls-remote`,
   peeled) and file-at-tag reads (depth-1 fetch) are SCM-neutral, so
   `publisher.scm` demotes to provenance metadata plus a shorthand
   expansion hint; core logic never branches on it.
4. **Dialect selection by recorded config, not env-sniffing.** Conformance
   parses the dialect named by `ci:`, so a fleet audit or local run decides
   identically to CI. Env detection survives only for the CI *output*
   surface, where the ambient host is by definition the right answer.
5. **Ownership follows the SCM axis.** CODEOWNERS is GitHub-only and
   opt-in at init (`--codeowners`); Azure Repos consumers get a
   required-reviewers branch-policy checklist item backed by the
   `required_reviewers_policy` attestation. The `control-plane-owned` core
   rule is `waivable` accordingly.

## Alternatives considered

- **One `platform` value with per-feature overrides.** Keeps the common
  case short but preserves the conflation; the mixed case becomes a pile of
  exceptions. Rejected.
- **Infer everything at runtime, record nothing.** Fails precisely where
  conformance matters most (offline/fleet evaluation) and cannot represent
  `ci: none`. Rejected.
- **Full URLs everywhere, no shorthand.** Considered; kept shorthand
  because publisher coordinates appear in human-maintained YAML, with
  verbatim URL/path always accepted (and used by the scenario kit).

## Consequences

- A GitHub-hosted repo built by Azure Pipelines is now simply
  `scm: github, ci: azure-pipelines` — every feature picks the right axis.
- `ci: none` gives a supported fully-manual mode: gate/watch/sync run by
  hand or cron; pipeline-dependent conformance rules report `skipped`, and
  the forfeited PR-time enforcement is stated on the init checklist rather
  than hidden.
- Schema stays v1 (pre-release, no compatibility debt); the old
  `publisher.platform` key is simply gone.
- CI remains the one axis with in-core knowledge (template packs, dialect
  parsing, output surface) — host-environment adaptation, allowed by the
  format-vs-service rule and irreducible for software that runs inside CI.
