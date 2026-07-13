# Spec: Onboarding and consumer configuration

Status: draft for implementation · Owner: Layer 3

## 1. Consumer configuration file

One file per vendored slice: `.vendkit/<slice>.yml`. Consumer-owned (scaffolded
once, then hand-maintained; **not** manifest-tracked — it holds consumer-local
values). This is the single consumer config surface: environment axes,
profile binding, pin location, watch channel, delivery handlers,
attestations, waivers.

```yaml
schema_version: 1
slice: docs
publisher:
  scm: github                         # github | azure-repos — provenance and
  repo: example-org/design-docs       #   shorthand-expansion hint only; any
                                      #   git URL or path is used verbatim
scm: github                           # the CONSUMER's SCM host (DR-0015)
ci: github-actions                    # github-actions | azure-pipelines | none
profile: code-repo                    # optional; must exist in the declaration
pin:
  # First entry is the authoritative read source; the sync PR advances the
  # matching reference line in every listed file, in lockstep. With
  # `ci: none` the pin block must be EMPTY — the manifest's source.release
  # is the pin.
  pattern: "ref: refs/tags/v"
  files:
    - .github/workflows/docs-sync.yml
    - .github/workflows/vendkit-gate.yml
engine:                               # the pinned engine binary (DR-0016)
  version: v1.4.2                     # advanced with the content pin by sync
  sha256:                             # per-platform; blank = advisory
    linux/amd64: "…"
    darwin/arm64: "…"
watch:
  channel: stable                     # stable | rc
handlers:                             # delivery handlers (handler protocol)
  pr:
    exec: [vendkit, handler, github]
  handoff:
    exec: [vendkit, handler, github]
    dedup_key: vendkit-watch-docs
  fact-verify:
    exec: [vendkit, handler, github]
seeds:
  notes: informational                # informational | silent (DR-0013)
attestations:
  branch_protection_enabled: true
  sync_credential_provisioned: true
waivers: []                           # [{rule: <id>, reason: "…"}]
```

Beside it, engine-owned: `.vendkit/<slice>-manifest.json` (manifest spec).
Discovery convention (fixed; INV-8): tools enumerate `.vendkit/*.yml` for
slices and `.vendkit/*-manifest.json` for manifests — nothing else, nowhere
else. The namespace is **strict**: a `.vendkit/*.yml` that does not parse as
a slice config is a usage error, never a silent skip (DR-0012).

Field notes:

- `scm` / `ci` are the two environment axes (DR-0015). They are independent:
  a GitHub-hosted repo built by Azure Pipelines is `scm: github,
  ci: azure-pipelines`. Conformance parses the pipeline dialect named by
  `ci:` — never env-sniffed, so fleet audits decide identically.
- `ci: none` is fully manual mode: no pipelines were scaffolded; gate,
  watch and sync are run by the human (or cron); the pin block is empty;
  pipeline-dependent conformance rules report `skipped`.
- `publisher` coordinates deliberately also appear in the manifest's
  `source:` (immutable provenance of what was actually vendored) and in the
  publisher's own declaration (its identity). The three agree by
  construction; a conformance tree-check flags drift between config and
  manifest.
- `handlers` may be omitted (all deliveries unwired — report-only) and each
  kind is independently optional. `VENDKIT_HANDLER_<KIND>` env vars override
  per run (handler-protocol spec §4).

## 2. The scaffolder

`vendkit init --ci github-actions|azure-pipelines|none [--scm github|azure-repos]
--version vX.Y.Z [--profile <p>] [--mode primary|additive] [--base-branch main]
[--pr-token-secret <name>] [--codeowners <owners>]`   (alias: `onboard`)

Run from a checkout of the **publisher at the release being pinned** (so the
scaffold, engine and content come from one immutable tree). `--scm` defaults
to inference from the consumer's origin remote (`github.com` /
`dev.azure.com` / `*.visualstudio.com`); no remote and no flag is a loud
usage error, never a guess. Three phases:

1. **Vendor.** Seed an empty manifest, then materialise with scope
   reconciliation — the profile's full export slice lands, tracked, with
   provenance recorded.
2. **Scaffold.** Render the CI-keyed template set with the slice identity
   (nothing for `ci: none`). Placeholder substitution must fail loudly on
   any unresolved variable.
3. **Report.** Print the manual step list (§4) and the conformance gaps that
   remain. The scaffolder never performs trust-bootstrap acts itself and never
   re-implements conformance judgment — it *defers* to `vendkit conformance`.

### Scaffolded outputs

| Output | github-actions | azure-pipelines | none | Mode |
|---|---|---|---|---|
| Gate pipeline (all slices, no path filter) | `.github/workflows/vendkit-gate.yml` | `azure-pipelines/vendkit-gate.yml` | — | primary only |
| Sync pipeline (per slice; schedule + optional push hint) | `.github/workflows/<slice>-sync.yml` | `azure-pipelines/<slice>-sync.yml` | — | always |
| Watch pipeline (all slices; weekly + PR dry-run self-test) | `.github/workflows/vendkit-watch.yml` | `azure-pipelines/vendkit-watch.yml` | — | primary only |
| Conformance pipeline (advisory) | `.github/workflows/vendkit-conformance.yml` | `azure-pipelines/vendkit-conformance.yml` | — | primary only |
| Slice config | `.vendkit/<slice>.yml` | same | same | always |
| Slice manifest | `.vendkit/<slice>-manifest.json` | same | same | always |
| CODEOWNERS stanza covering `.vendkit/` | **opt-in** via `--codeowners` (scm github only) | refused: Azure Repos does not honour CODEOWNERS — required-reviewers policy on the checklist instead | scm-dependent | opt-in |

**`primary` vs `additive`:** the first slice onboards the shared machinery
(gate, watch, conformance pipelines — which are *slice-agnostic* by design,
architecture §5); each further slice adds only its own sync pipeline, config
and manifest. Additive onboarding must be idempotent: re-running detects the
existing slice and repairs rather than duplicates.

Scaffolded pipeline callers are consumer-owned after generation (they embed
consumer-local values: schedules, secret names, base branch) and are therefore
**not** drift-gated. The framework components they call are pinned by tag; the
conformance `pipeline-wired` rules keep the wiring honest.

## 3. Platform notes baked into the scaffolds

- **github-actions sync:** the PR handler takes its token from
  `secrets.<pr-token-secret>` via `VENDKIT_TOKEN_OPEN_PR`; the reference
  handler refuses a `GITHUB_TOKEN` fallback (differences ledger #2) and the
  workflow carries a comment saying why.
- **azure-pipelines sync/gate:** scaffold emits the pipeline YAML *and*
  prints the branch-policy commands/URLs for Build Validation (which cannot
  be set from YAML). The corresponding conformance rules stay red until done
  — the checklist is executable, not prose.
- **Handlers:** the slice config's default handler commands are keyed by the
  consumer's `scm` (`vendkit handler github` / `vendkit handler ado`, built
  into the engine binary); `ci: none` leaves them unwired with a comment
  showing how to wire one.
- **Engine (DR-0016):** the scaffolded lanes' first step fetches the pinned
  engine binary, verifies it against the release `SHA256SUMS.txt`, caches it,
  and runs `vendkit self-verify` against the `engine.sha256` pin before the
  lane proper — no interpreter, no build step on the runner.
- **Push hint:** `--push-hint` adds the `resources.pipelines` trigger
  (azure-pipelines) or the `repository_dispatch` receiver (github-actions)
  to the sync pipeline (platform-integration spec §4).

## 4. Irreducible manual steps (reported, never performed)

1. Grant the CI identity read on the publisher repo (content checkout +
   engine artefact fetch).
2. Record the engine checksums in `engine.sha256` from the pinned release's
   `SHA256SUMS.txt` (one hex per platform). Until filled, `vendkit
   self-verify` is advisory; the fetch step still verifies each download
   against `SHA256SUMS.txt` (DR-0016). (Skipped for `ci: none`.)
3. Provision the PR-capable credential as the named secret for the PR
   handler; on ADO also policy-exempt it for *creation* only, on GitHub
   prefer a GitHub App. (Skipped for `ci: none`.)
4. Enable branch protection / branch policies; make the gate a required
   check (github: ruleset; azure-repos: Build Validation). Under `ci: none`
   this step instead states plainly: there is no PR-time gate — automated
   drift enforcement is forfeited and `vendkit gate --strict` must be run
   by hand.
5. On azure-repos: add a required-reviewers branch policy covering
   `.vendkit/**` (the CODEOWNERS equivalent) and record the
   `required_reviewers_policy` attestation.
6. Record the attestations for the above in the slice config.

Each maps 1:1 to a conformance rule, so "fully onboarded" is
`vendkit conformance --strict` passing — the onboarding checklist and the
conformance spec can never diverge.
