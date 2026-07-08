# Spec: Platform ports

Status: draft for implementation · Owner: Layer 1 (+ Layer 2 packaging)

The port is the **only** boundary where CI platforms appear. ADO and GitHub
Actions are peer, first-class backends: neither is the reference implementation;
the conformance kit runs the same scenario matrix against both bindings
(INV-8). A third platform is added by implementing this interface plus a
Layer 2 packaging and a scaffold template set — with no engine change.

## 1. Port selection

`vendkit.ports.detect()`:

- `GITHUB_ACTIONS=true` → `github`
- `TF_BUILD=True` → `ado`
- otherwise → `neutral` (local runs, tests, fleet audit)

Explicit override: `--platform github|ado|neutral` / `VENDKIT_PLATFORM`. Note
the port of the *runner* and the platform of a *publisher* are independent: a
GHA consumer can watch an ADO publisher; `releases.list` is keyed by the
upstream's declared `publisher.platform`, not by the runner.

## 2. The interface

Python `Protocol`; every method total (raises `PortError` with a remediation
hint on failure). Six operation groups:

```python
class Port(Protocol):
    # 1. Release listing (used by: watch, release cutter, provenance checks)
    def list_release_tags(self, repo: RepoRef) -> list[TagInfo]:
        """All tags with names + resolved commit SHAs. Grammar filtering is
        Layer 0's job; the port just lists refs."""

    # 2. Pull requests (used by: sync lane)
    def find_open_pr(self, repo: RepoRef, head_branch: str) -> PrRef | None: ...
    def open_or_update_pr(self, repo: RepoRef, head_branch: str,
                          base_branch: str, title: str, body_md: str) -> PrRef: ...

    # 3. Work items (used by: watch handoff, migration handoff, conformance)
    def upsert_work_item(self, dedup_key: str, title: str, body_md: str,
                         routing: RoutingConfig) -> WorkItemRef:
        """Find-open-by-dedup-key → comment; else create. GitHub: issue +
        label. ADO: work item + tag (type/fields from RoutingConfig)."""

    # 4. CI output surface (used by: every Layer 2 wrapper)
    def emit_output(self, key: str, value: str) -> None:
        """GITHUB_OUTPUT file / ##vso setvariable;isOutput. Neutral port:
        `key=value` to stdout (which ALL ports also emit, for log-greppability)."""
    def emit_summary(self, markdown: str) -> None:
        """GITHUB_STEP_SUMMARY / ##vso uploadsummary. Neutral: stderr."""
    def emit_error(self, message: str) -> None:
        """error annotation; neutral: stderr + nonzero-exit responsibility stays
        with the caller."""

    # 5. Attestation/API verification (used by: conformance --verify-attestations)
    def verify_platform_fact(self, fact: PlatformFact) -> bool | None:
        """None = cannot verify with current credential scopes (stay attested)."""

    # 6. Credential resolution
    def credential(self, purpose: Purpose) -> Credential:
        """purpose ∈ {read_upstream, push_branch, open_pr, work_items}.
        Resolution order and platform caveats in §3."""
```

Structured values (`TagInfo{name, commit}`, `RepoRef{platform, repo}`,
`RoutingConfig`) are plain dataclasses defined in `ports/base.py`.

Explicitly **not** in the port: file I/O, hashing, globbing, YAML, version
comparison, PR body composition, branch naming — all Layer 0.

## 3. Credential models

| Purpose | GitHub Actions | Azure DevOps |
|---|---|---|
| read upstream (watch, template/action resolution) | `GITHUB_TOKEN` if same-org + granted; else fine-grained PAT / App token | build service identity granted Read on publisher repo; else PAT |
| push sync branch | `GITHUB_TOKEN` (contents: write) | `System.AccessToken` (Contribute) |
| open sync PR | **PAT or App token — not `GITHUB_TOKEN`**: PRs opened with the workflow token do not trigger `pull_request` workflows, so the sync PR would silently skip its own gate. Scaffold refuses to wire `GITHUB_TOKEN` here. | PR-capable credential; must be exempt from / able to satisfy branch policies. `System.AccessToken` works when the project grants it PR-create. |
| work items / issues | `GITHUB_TOKEN` (issues: write) | PAT or granted `System.AccessToken` (work item write) |

Port duties: resolve per §above with env-var conventions
(`VENDKIT_TOKEN_<PURPOSE>` overrides), **fail loud** on missing/expired
credentials (`PortError`, red pipeline — silent skips are forbidden), and
support a scheduled *credential liveness probe* (each purpose exercised
read-only) that the scaffold wires beside watch, so expiry is a visible failure
on a schedule rather than a missed release later.

## 4. Push hints (release → consumer sync trigger)

Pull (schedule) is the reconciler; push is a latency optimisation (DR-0006).
Mechanisms differ by *consumer* platform:

- **ADO consumer:** the sync pipeline declares the publisher's release pipeline
  as a `resources.pipelines` entry with `trigger: true` (same organisation).
  Consumer-declared: the publisher keeps no registry. This is the preferred
  shape.
- **GHA consumer:** GitHub has no consumer-declared cross-repo trigger. The
  sync workflow adds `on: repository_dispatch: types: [vendkit-release]`; the
  *publisher's* release workflow optionally dispatches to subscribers listed in
  a `subscribers:` file maintained **by consumer PRs** to the publisher repo
  (opt-in, self-service, auditable). This requires a publisher-held token with
  dispatch scope per subscriber — the one place the "publisher knows no
  downstream" stance is relaxed, which is why it is optional and the schedule
  remains mandatory.
- **Tier chains:** hints compose hop-by-hop (each tier's release triggers the
  next tier's sync), collapsing multi-cadence propagation latency to same-day
  while every hop stays a reviewed PR.

## 5. Layer 2 packaging

Per component (gate, sync, watch, conformance, migration-verify, release),
each platform ships a thin wrapper:

- **ADO:** step template `platforms/ado/templates/<component>.yml`; consumed via
  a pinned `resources.repositories` alias. Parameters only; body = setup Python,
  run CLI, map outputs via port.
- **GHA:** composite action `platforms/github/actions/<component>/action.yml`
  (consumed `uses: <framework-repo>/platforms/github/actions/<component>@vX.Y.Z`)
  plus, where a whole job is the natural unit (sync), a reusable workflow.

Wrapper rules (architecture §1): no logic, no platform REST calls (the CLI +
port do that), identical parameter names across platforms wherever the concept
exists on both. The scaffolder (onboarding spec) generates the consumer-side
callers for the chosen platform.

## 6. Behavioural differences ledger

Every known cross-platform behaviour difference must be recorded here as it is
discovered, with its mitigation. Seed entries:

| # | Difference | Mitigation |
|---|---|---|
| 1 | Azure Repos ignores YAML `pr:` triggers | PR gating = Build Validation policy; conformance decides via attest/API (conformance spec §3) |
| 2 | `GITHUB_TOKEN`-opened PRs don't trigger workflows | sync PR credential must be PAT/App; scaffold enforces (§3) |
| 3 | No consumer-declared cross-repo trigger on GitHub | repository_dispatch + subscribers file, optional (§4) |
| 4 | ADO tags lack protection rules; GitHub rulesets can protect `refs/tags/*` | ADO: ref-level Manage/Force-push permissions; both: provenance SHA check (security model §2) |
| 5 | Required-check enforcement is invisible in-tree on both | conformance attest with API upgrade path (conformance spec §4) |
