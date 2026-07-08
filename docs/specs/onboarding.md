# Spec: Onboarding and consumer configuration

Status: draft for implementation · Owner: Layer 3

## 1. Consumer configuration file

One file per vendored slice: `.vendkit/<slice>.yml`. Consumer-owned (scaffolded
once, then hand-maintained; **not** manifest-tracked — it holds consumer-local
values). This is the single consumer config surface: profile binding, pin
location, watch/handoff routing, attestations, waivers.

```yaml
schema_version: 1
slice: docs
publisher:
  platform: github                    # github | ado
  repo: example-org/design-docs
profile: code-repo                    # optional; must exist in the declaration
pin:
  file: .github/workflows/docs-sync.yml
  pattern: "example-org/design-docs@v"
  files:                              # every file whose reference the sync PR advances
    - .github/workflows/docs-sync.yml
    - .github/workflows/vendkit-gate.yml
watch:
  channel: stable                     # stable | rc
  handoff:
    kind: issue                       # issue | workitem
    dedup_key: vendkit-watch-docs
    routing: {}                       # port RoutingConfig (labels / area path / parent)
attestations:
  branch_protection_enabled: true
  sync_credential_provisioned: true
waivers: []                           # [{rule: <id>, reason: "…"}]
```

Beside it, engine-owned: `.vendkit/<slice>-manifest.json` (manifest spec).
Discovery convention (fixed; INV-8): tools enumerate `.vendkit/*.yml` for
slices and `.vendkit/*-manifest.json` for manifests — nothing else, nowhere
else.

## 2. The scaffolder

`vendkit onboard --platform github|ado --profile <p> --version vX.Y.Z
[--mode primary|additive] [--base-branch main] [--pr-token-secret <name>]`

Run from a checkout of the **publisher at the release being pinned** (so the
scaffold, engine and content come from one immutable tree). Three phases:

1. **Vendor.** Seed an empty manifest, then materialise with scope
   reconciliation — the profile's full export slice lands, tracked, with
   provenance recorded.
2. **Scaffold.** Render the platform-keyed template set with the slice
   identity. Placeholder substitution must fail loudly on any unresolved
   variable.
3. **Report.** Print the manual step list (§4) and the conformance gaps that
   remain. The scaffolder never performs trust-bootstrap acts itself and never
   re-implements conformance judgment — it *defers* to `vendkit conformance`.

### Scaffolded outputs

| Output | GitHub Actions | Azure DevOps | Mode |
|---|---|---|---|
| Gate pipeline (all slices, no path filter) | `.github/workflows/vendkit-gate.yml` | `azure-pipelines/vendkit-gate.yml` | primary only |
| Sync pipeline (per slice; schedule + optional push hint) | `.github/workflows/<slice>-sync.yml` | `azure-pipelines/<slice>-sync.yml` | always |
| Watch pipeline (all slices; weekly + PR dry-run self-test) | `.github/workflows/vendkit-watch.yml` | `azure-pipelines/vendkit-watch.yml` | primary only |
| Conformance pipeline (advisory) | `.github/workflows/vendkit-conformance.yml` | `azure-pipelines/vendkit-conformance.yml` | primary only |
| Slice config | `.vendkit/<slice>.yml` | same | always |
| Slice manifest | `.vendkit/<slice>-manifest.json` | same | always |
| Ownership entries | `CODEOWNERS` stanza covering `.vendkit/**` + vendored namespaces | same | primary; appended additively after |

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

- **GHA sync:** the PR-opening step takes its token from
  `secrets.<pr-token-secret>`; the scaffolder refuses `GITHUB_TOKEN` here
  (ports spec §3, difference #2) and writes a comment in the workflow saying
  why.
- **ADO sync/gate:** scaffold emits the pipeline YAML *and* prints the
  branch-policy commands/URLs for Build Validation (which cannot be set from
  YAML). The corresponding conformance rules stay red until done — the
  checklist is executable, not prose.
- **Push hint:** `--push-hint` adds the `resources.pipelines` trigger (ADO) or
  the `repository_dispatch` receiver (GHA) to the sync pipeline (ports spec
  §4).

## 4. Irreducible manual steps (reported, never performed)

1. Grant the CI identity read on the publisher repo (template/action
   resolution + checkout).
2. Provision the PR-capable credential as the named secret; on ADO also
   policy-exempt it, on GitHub prefer a GitHub App.
3. Enable branch protection / branch policies; make the gate a required check
   (GHA: ruleset; ADO: Build Validation).
4. Record the attestations for 1–3 in the slice config.

Each maps 1:1 to a mandatory conformance rule, so "fully onboarded" is
`vendkit conformance --strict` passing — the onboarding checklist and the
conformance spec can never diverge.
