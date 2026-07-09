# Spec: CLI surface

Status: draft for implementation · Owner: Layer 0 (surface), delegating per command

One entrypoint: `vendkit`. Single Python package, Python ≥ 3.10. Global flags:
`--platform` (CI output surface override: `github-actions` |
`azure-pipelines` | `neutral`), `--export-decl`, `--consumer-root`,
`--json`, `--quiet`.

| Command | Role | Layer | Spec |
|---|---|---|---|
| `vendkit generate [--check]` | build/verify publisher manifest | 0 | manifest-and-gate §3 |
| `vendkit gate [--strict] [--all\|--manifest <p>]` | consumer integrity verify + INV-7 | 0 | manifest-and-gate §4 |
| `vendkit sync --check\|--apply --target <v> [--reconcile-scope] [--porcelain]` | materialise (low-level) | 0 | sync §2 |
| `vendkit sync-pipeline --slice <s> [--base-branch <b>]` | full sync lane: versions, probe, apply, pin advance, branch, push, PR intent → handler | 0+1 | sync §3 |
| `vendkit release --bump <b>\|--version <v>` | cut a release | 0 | releases §3 |
| `vendkit watch [--slice <s>] [--dry-run] [--no-handoff]` | detect upstream releases; findings → handoff handler | 0+1 | release-watch |
| `vendkit migrations --pinned <v> --target <v>` | resolve migration window | 0 | migrations §2 |
| `vendkit migrations-verify --obligations <json>` | deterministic obligation check | 0 | migrations §4 |
| `vendkit conformance [--strict] [--verify-attestations]` | adoption check; verification → fact-verify handler | 0+1 | conformance |
| `vendkit init --ci <c> [--scm <s>] --version <v> [--profile <p>] [--codeowners <o>]` | scaffold a consumer (alias: `onboard`). `--ci none` = fully manual mode; `--scm` inferred from the origin remote when omitted | 3 | onboarding §2 |

Removed pre-1.0: `is-newer` (an artefact of step-wise wrappers; the compare
is internal to `sync-pipeline` and `watch`, and remains unit-tested as
`core.versions.is_newer`).

## Human tier

Human-first verbs, layered strictly as **compositions of the machine tier**
— never a parallel code path, so the invariants cover what humans actually
run. Their formatting is exempt from the `key=value` stability promise (the
machine tier keeps it); scripts must not parse them.

| Command | Does |
|---|---|
| `vendkit status [--slice <s>]` | per-slice rollup: pinned vs latest (git protocol), update/bump class, drift finding count, ci mode. THE entry point. |
| `vendkit diff [--slice <s>] [--target <v>]` | unified diff of every file `update` would write, against a throwaway depth-1 checkout of the target (default: latest). Read-only. |
| `vendkit update [--slice <s>] [--target <v>] [--local\|--pr]` | the whole upgrade. `--local` (default): materialise + manifest + pin advance in the working tree; you review and commit. `--pr`: delegates to `sync-pipeline` against the fetched checkout. |
| `vendkit explain [<topic>\|list]` | what a finding / refusal / status token means and the sanctioned fix. |
| `vendkit init` (§ machine table) | prompts for `--ci`, `--version`, profile, and un-inferable `--scm` on a TTY; fully flag-driven (and loudly failing) when non-interactive. |

`--slice` may be omitted when exactly one slice is configured.

**INV-6 relaxation (documented):** the human tier runs the *installed*
engine against a fetched target tree, unlike the CI sync lane where the
pinned checkout supplies both content and engine. The declaration/manifest
schema-version gates make skew loud rather than silent; consumers wanting
the strict property use the scheduled lane.

## Conventions (uniform across commands)

- **Exit codes:** 0 success; 1 findings-in-strict-mode; 2 usage/config error;
  3 refusal (`refused=` reason emitted: `retracted`, `tag-moved`, …);
  ≥4 infrastructure failure — including a nonzero handler exit. Watch never
  encodes findings in its exit code (release-watch §2).
- **Output:** human-readable report to stdout; every machine-relevant fact
  *also* as `key=value` lines; `--json` for the full document. The CI output
  surface mirrors `key=value` facts into platform outputs — Layer 2 wrappers
  never parse prose.
- **Summaries:** commands accept `--summary` to emit a Markdown summary via
  the CI surface.
- **Deliveries:** commands never call vendor APIs; where a delivery is
  needed they compose an intent and invoke the configured handler
  (handler-protocol spec). Unwired handlers are visible, defined states —
  `pr-delivered=false` + `pr-intent=`, `handoff=unwired` — never silent
  skips.
- **Dependency rule (INV-9):** `gate` and `migrations-verify` must import
  stdlib only. YAML-needing commands import the YAML library lazily.
- **No hidden state:** no config files other than those specified (export
  declaration, slice configs, manifests); no environment reads outside
  `ci.detect()`, credential passthrough to handlers, and the
  `VENDKIT_HANDLER_<KIND>` overrides.
- **Deprecation policy:** CLI flags and `key=value` output names are a public
  API once released; removal requires a MAJOR release and a migration entry.
