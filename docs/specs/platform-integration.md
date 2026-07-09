# Spec: Platform integration — CI surfaces, credentials, differences ledger

Status: draft for implementation · Owner: Layer 1 (+ Layer 2 packaging)

This spec covers everything platform-flavoured that is *not* the handler
protocol (which has [its own spec](handler-protocol.md)). After DR-0014 and
DR-0015 the shape is:

| Concern | Where it lives | Vocabulary |
|---|---|---|
| Vendor **services** (PRs, work items, fact APIs) | handler executables | — |
| Upstream reads (tags, file-at-tag) | core, git protocol | `publisher.scm` (metadata only) |
| CI **output dialect** | in-process CI surface (this spec §2) | `github-actions` \| `azure-pipelines` \| `neutral` |
| CI pipeline **dialect parsing** | conformance detectors (formats, not services) | slice config `ci:` |
| Template packs | Layer 3 scaffolds, one directory per CI | `scaffold/<ci>/` |

GitHub and Azure DevOps remain peer, first-class backends: the conformance
kit runs the same scenario matrix against both template packs and reference
handlers (INV-8). A third platform is added by a template pack, a handler
executable, and (if its output dialect differs) one small CI surface class —
with no engine change.

## 1. CI surface selection

`vendkit.ci.detect()`:

- `GITHUB_ACTIONS=true` → `github-actions`
- `TF_BUILD=True` → `azure-pipelines`
- otherwise → `neutral` (local runs, tests, fleet audit)

Explicit override: `--platform` / `VENDKIT_PLATFORM`. Env detection is
correct *here* — the ambient host is by definition the right output dialect —
and only here; every other platform decision reads recorded config
(DR-0015). Note the runner's CI, the consumer's `ci:`, and the publisher's
`scm` are three independent facts.

## 2. The CI output surface

The whole in-process interface:

```python
class OutputSurface(Protocol):
    def emit_output(self, key: str, value: str) -> None:
        """GITHUB_OUTPUT file / ##vso setvariable;isOutput. ALL surfaces
        also print the plain key=value line, for log-greppability."""
    def emit_summary(self, markdown: str) -> None:
        """GITHUB_STEP_SUMMARY / ##vso uploadsummary. Neutral: stderr."""
    def emit_error(self, message: str) -> None:
        """::error:: / ##vso logissue. Neutral: stderr; exit codes stay
        the caller's responsibility."""
```

Explicitly **not** here (all removed by DR-0014/DR-0015): release listing,
file reads, PRs, work items, fact verification, credential objects. File
I/O, hashing, globbing, version compare, PR body composition, and branch
naming were never here (Layer 0).

## 3. Credential model

Credentials are resolved *by the party that spends them*:

- **Git operations** (clone/fetch/ls-remote upstream, push the sync branch)
  use ordinary git credentials — the checkout step's token, a credential
  helper, or the URL. The engine adds nothing.
- **Handlers** resolve their own API tokens: `VENDKIT_TOKEN_<PURPOSE>`
  overrides, then vendor conventions.

| Purpose | GitHub | Azure DevOps |
|---|---|---|
| read upstream (checkout/fetch) | `GITHUB_TOKEN` if same-org + granted; else fine-grained PAT / App token | build service identity granted Read on the publisher repo; else PAT |
| push sync branch | `GITHUB_TOKEN` (contents: write) | `System.AccessToken` (Contribute) |
| deliver sync PR (`pr` handler) | **PAT or App token — not `GITHUB_TOKEN`**: PRs opened with the workflow token do not trigger `pull_request` workflows, so the sync PR would silently skip its own gate. The reference handler refuses the fallback. | PR-capable credential; must be able to satisfy branch policies. `System.AccessToken` works when the project grants it PR-create. |
| work items / issues (`handoff` handler) | `GITHUB_TOKEN` (issues: write) | PAT or granted `System.AccessToken` (work-item write) |

Failure duties: a missing/expired credential is a loud failure (nonzero
handler exit → red pipeline; git failure → red pipeline) — silent skips are
forbidden. The scaffold wires a scheduled *credential liveness probe*
(each purpose exercised read-only) beside watch, so expiry is a visible
failure on a cadence rather than a missed release later.

## 4. Push hints (release → consumer sync trigger)

Pull (schedule) is the reconciler; push is a latency optimisation (DR-0006).
Mechanisms differ by *consumer CI*:

- **azure-pipelines:** the sync pipeline declares the publisher's release
  pipeline as a `resources.pipelines` entry with `trigger: true` (same
  organisation). Consumer-declared: the publisher keeps no registry. This is
  the preferred shape.
- **github-actions:** GitHub has no consumer-declared cross-repo trigger.
  The sync workflow adds `on: repository_dispatch: types: [vendkit-release]`;
  the *publisher's* release workflow optionally dispatches to subscribers
  listed in a `subscribers:` file maintained **by consumer PRs** to the
  publisher repo (opt-in, self-service, auditable). This needs a
  publisher-held token with dispatch scope per subscriber — the one place
  the "publisher knows no downstream" stance is relaxed, which is why it is
  optional and the schedule remains mandatory.
- **ci: none:** no push target exists; the human (or their cron) is the
  trigger. Watch remains the safety net.
- **Tier chains:** hints compose hop-by-hop, collapsing multi-cadence
  propagation latency to same-day while every hop stays a reviewed PR.

## 5. Layer 2 packaging

Per component (gate, sync, watch, conformance, migration-verify, release),
each CI platform ships a thin wrapper:

- **azure-pipelines:** step template `platforms/azure-pipelines/templates/
  <component>.yml`; consumed via a pinned `resources.repositories` alias.
- **github-actions:** composite action `platforms/github-actions/actions/
  <component>/action.yml` plus, where a whole job is the natural unit
  (sync), a reusable workflow.

Wrapper rules (architecture §1): no logic, no API calls (the CLI + handlers
do that), identical parameter names across platforms wherever the concept
exists on both. The scaffolder generates the consumer-side callers.

## 6. Behavioural differences ledger

Every known cross-platform behaviour difference must be recorded here as it
is discovered, with its mitigation. Seed entries:

| # | Difference | Mitigation |
|---|---|---|
| 1 | Azure Repos ignores YAML `pr:` triggers | PR gating = Build Validation policy; conformance decides via attest/API (conformance spec §3) |
| 2 | `GITHUB_TOKEN`-opened PRs don't trigger workflows | `pr` handler credential must be PAT/App; reference handler refuses the fallback (§3) |
| 3 | No consumer-declared cross-repo trigger on GitHub | repository_dispatch + subscribers file, optional (§4) |
| 4 | ADO tags lack protection rules; GitHub rulesets can protect `refs/tags/*` | ADO: ref-level Manage/Force-push permissions; both: provenance SHA check (security model §2) |
| 5 | Required-check enforcement is invisible in-tree on both | conformance attest with fact-verify upgrade path (conformance spec §4) |
| 6 | Azure Repos does not honour CODEOWNERS | ownership = required-reviewers branch policy + `required_reviewers_policy` attestation; CODEOWNERS is GitHub-only and opt-in at init (DR-0015) |
