# Spec: Conformance

Status: draft for implementation · Owner: Layer 0 (engine) + Layer 1 (detector bindings)

Conformance answers "is this consumer correctly wired?" against a rule spec
that **ships inside each release** — so advancing a pin brings the current rule
set into force, and improving the framework surfaces the new gap in every
not-yet-adopted consumer.

Two rule sources compose:

- **Core rules** (`conformance/core-rules.yml`, shipped by the framework):
  platform-neutral wiring requirements every slice needs — gate wired and
  required, sync scheduled, watch present, manifest committed, pins are tags.
- **Publisher rules** (optional `conformance.yml` beside the export
  declaration): slice-specific requirements a publisher adds (e.g. "profile
  declared", domain-specific checks via `tool` detectors).

## 1. Rule schema (v1)

```yaml
schema_version: 1
rules:
  - id: gate-wired                    # stable kebab-case, never renamed
    title: "Gate lane wired and required on PRs"
    category: enforcement             # prerequisites | protection | vendoring |
                                      # enforcement | sync | promotion | custom
    severity: mandatory               # mandatory (unwaivable) | waivable | advisory
    detector:
      kind: pipeline-wired            # see registry §2
      component: gate                 # gate | sync | watch | migration-verify
      pinned: true                    # the reference must be an immutable tag
      events: [pull_request]          # pull_request | schedule
      required_check: true            # must be enforced, not merely present
    remediation: >-
      Wire the gate pipeline (scaffold: `vendkit init`), pin it to a release
      tag, and make it a required check / build-validation policy.
```

Statuses per rule: `pass`, `fail`, `waived`, `attested`, `skipped`, `error`.
`fail`/`error` are gaps. `vendkit conformance [--strict]`: strict exits 1 on
any non-waived gap; default is advisory (exit 0, report only). Waivers live in
the consumer slice config (`waivers: [{rule, reason}]`) and are honoured only
for `severity: waivable` rules — a waiver on a mandatory rule is itself a
finding.

## 2. Detector registry

Platform-neutral kinds, implemented in Layer 0 unless noted:

| Kind | Decides | Notes |
|---|---|---|
| `file-exists` | path present | |
| `manifest-tracked` | path is a `consumer_path` in the slice manifest | |
| `profile-bound` | slice config declares a profile the declaration defines | |
| `codeowners-covers` | ownership review covers given patterns | SCM-axis: GitHub parses CODEOWNERS; Azure Repos does not honour it → degrades to the `required_reviewers_policy` attestation (DR-0015) |
| `paths-lockstep` | if the gate pipeline path-filters, the filter covers every `consumer_path` | parses the dialect named by the slice config's `ci:`; `skipped` under `ci: none` |
| `pipeline-wired` | a pipeline references the given framework component, pinned, on the given events, enforced | dialect by `ci:`, §3; `skipped` under `ci: none` (manual mode is visible, not hidden) |
| `attest` | consumer config asserts a non-tree-decidable fact | §4 |
| `tool` | a manifest-tracked executable exits 0 | shared with migration checks |

Pipeline parsing is *format* knowledge and stays in core; anything needing a
vendor *service* goes through the fact-verify handler (§4) — the
format-vs-service rule (DR-0015).

Publishers may not invent detector kinds; they extend via `tool` (vendored,
gate-verified executables). This keeps the checker's trusted computing base
fixed.

## 3. `pipeline-wired`: per-CI decidability

The same rule is decided differently per CI dialect (selected by the slice
config's `ci:` field, never env-sniffed — a fleet audit decides identically
to CI):

| Aspect | github-actions | azure-pipelines |
|---|---|---|
| Locate pipelines | `.github/workflows/*.yml` | `azure-pipelines/*.yml`, `azure-pipelines.yml` |
| "references component" | the pipeline invokes the component's CLI subcommand (direct call, composite action, or template all bottom out in the invocation) | same |
| `pinned` | a `refs/tags/vX.Y.Z` checkout ref, `@vX.Y.Z`, or 40-hex SHA | `resources.repositories` `ref:` starts `refs/tags/` |
| `events: [pull_request]` | tree-decidable: `on: pull_request` | **not tree-decidable**: Azure Repos ignores `pr:`; requires Build Validation policy → `attest` (or fact-verify handler, §4) |
| `events: [schedule]` | tree-decidable: `on: schedule` | tree-decidable: `schedules:` block |
| `required_check` | branch protection / ruleset → `attest` or fact-verify | Build Validation policy → `attest` or fact-verify |
| `ci: none` | every `pipeline-wired`/`paths-lockstep` rule reports `skipped` — manual orchestration forfeits automated enforcement, stated in every report | same |

The rule *spec* stays identical across platforms; only the dialect binding
changes (INV-8). Where a platform fact is not tree-decidable the binding
degrades to an `attest` sub-check automatically, with the fact-verify
handler as the upgrade path (§4).

## 4. Attestations

Non-tree-decidable prerequisites (branch protection enabled, sync credential
provisioned and policy-exempt, publisher repo readable by CI identity) are
asserted in the consumer slice config:

```yaml
attestations:
  branch_protection_enabled: true
  sync_credential_provisioned: true
```

Attested rules report `attested`, not `pass` — dashboards can distinguish
verified facts from asserted ones. `conformance --verify-attestations`
sends each attested fact to the configured **fact-verify handler**
(handler-protocol spec §3): verdict `true` promotes `attested` → `pass`,
`false` demotes to `fail` (a wrong attestation is a finding), `unknown`
(insufficient scopes) leaves it attested. Core calls no vendor API itself.

## 5. Fleet view

`vendkit conformance --json` emits a machine document (per-rule status,
gap count, pin, pin lag, slice, profile). A **fleet audit** — a scheduled,
read-only job that clones each consumer, runs conformance + gate verify +
watch compare locally, and publishes one aggregated dashboard — is the intended
consumer of this format. It requires only read access (no inversion of the
trust model) and is specified as a Layer 3 optional component in the roadmap
(M4), not core.
