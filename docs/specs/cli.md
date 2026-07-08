# Spec: CLI surface

Status: draft for implementation · Owner: Layer 0 (surface), delegating per command

One entrypoint: `vendkit`. Single Python package, Python ≥ 3.10. Global flags:
`--platform` (port override), `--export-decl`, `--consumer-root`, `--json`,
`--quiet`.

| Command | Role | Layer | Spec |
|---|---|---|---|
| `vendkit generate [--check]` | build/verify publisher manifest | 0 | manifest-and-gate §3 |
| `vendkit gate [--strict] [--all\|--manifest <p>]` | consumer integrity verify + INV-7 | 0 | manifest-and-gate §4 |
| `vendkit sync --check\|--apply --target <v> [--reconcile-scope] [--porcelain]` | materialise (low-level) | 0 | sync §2 |
| `vendkit sync-pipeline --slice <s> [--base-branch <b>]` | full sync lane: versions, probe, apply, pin advance, branch, push, PR | 0+1 | sync §3 |
| `vendkit is-newer --pinned <v> --target <v> [--retracted <v>]…` | pure version compare; retracted list passed by the (YAML-capable) caller | 0 | releases §2, §4 |
| `vendkit release --bump <b>\|--version <v>` | cut a release | 0+1 | releases §3 |
| `vendkit watch [--slice <s>] [--dry-run]` | detect upstream releases, handoff | 0+1 | release-watch |
| `vendkit migrations --pinned <v> --target <v>` | resolve migration window | 0 | migrations §2 |
| `vendkit migrations-verify --obligations <json>` | deterministic obligation check | 0 | migrations §4 |
| `vendkit conformance [--strict] [--verify-attestations]` | adoption check | 0+1 | conformance |
| `vendkit onboard --target-platform <p> --profile <p> --version <v>` | scaffold a consumer (`--target-platform` avoids colliding with the global port `--platform`) | 3 | onboarding §2 |

## Conventions (uniform across commands)

- **Exit codes:** 0 success; 1 findings-in-strict-mode; 2 usage/config error;
  3 refusal (`refused=` reason emitted: `retracted`, `tag-moved`, …);
  ≥4 infrastructure failure. Watch never encodes findings in its exit code
  (release-watch §2).
- **Output:** human-readable report to stdout; every machine-relevant fact
  *also* as `key=value` lines; `--json` for the full document. Ports mirror
  `key=value` facts into platform outputs (`emit_output`) — Layer 2 wrappers
  never parse prose.
- **Summaries:** commands accept `--summary` to emit a Markdown summary via the
  port (`emit_summary`).
- **Dependency rule (INV-9):** `gate`, `migrations-verify` and `is-newer` must
  import stdlib only. YAML-needing commands import the YAML library lazily.
- **No hidden state:** no config files other than those specified (export
  declaration, slice configs, manifests); no environment reads outside the
  port.
- **Deprecation policy:** CLI flags and `key=value` output names are a public
  API once released; removal requires a MAJOR release and a migration entry.
