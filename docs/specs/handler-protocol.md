# Spec: Handler protocol

Status: stable (frozen at v1.0.0) · Protocol version: 1 · Owner: Layer 1

The engine never integrates with a vendor *service* (DR-0014). Where a core
judgment must become a delivery — a sync PR, a watch or migration work item,
an API-verified platform fact — the engine composes a JSON **intent
document** and hands it to a configured executable: a **handler**. Handlers
own the vendor API, its credentials, and the idempotency obligations this
protocol assigns them. The framework ships reference handlers for GitHub and
Azure DevOps; any executable honouring this protocol replaces them without
an engine change.

The boundary rule (DR-0015): core may understand vendor file *formats*
(pipeline YAML dialects, CODEOWNERS syntax, output conventions); it must not
call vendor *services*. Handlers are where services live.

## 1. Invocation contract

- The engine runs the handler command with the intent document as **JSON on
  stdin** (UTF-8, one document).
- The handler prints **`key=value` fact lines on stdout** (same convention
  as the CLI). Unknown keys are ignored; prose on stdout that is not
  `key=value`-shaped is ignored.
- **Exit 0 = delivered** (facts are trusted). **Nonzero = infrastructure
  failure** — the engine raises it loudly (CLI exit ≥ 4, red pipeline).
  Handlers never encode *judgment* in their exit code: every decision
  (is-newer, retraction, changed-ness, rule pass/fail) happened in core
  before the handler was invoked.
- Handlers inherit the engine's environment (that is how wrapper-provided
  credentials like `VENDKIT_TOKEN_OPEN_PR` reach them) and run with the
  consumer root as working directory.

## 2. Envelope

Every intent document carries:

```json
{
  "vendkit_handler_protocol": 1,
  "kind": "pr" | "handoff" | "fact-verify" | "push-hint",
  ...kind-specific fields...
}
```

A handler must reject (nonzero) a protocol version or kind it does not
understand — silent misdelivery is worse than a red run.

## 3. Kinds

### `pr` — deliver the sync PR

Sent by `sync-pipeline` after the branch is committed and pushed (the engine
does all git work itself; the handler only speaks to the PR API).

```json
{
  "kind": "pr",
  "head_branch": "vendkit/docs/sync-v1.4.2-to-v1.5.0",
  "base_branch": "main",
  "title": "sync(docs): v1.4.2 → v1.5.0",
  "body_md": "…",
  "slice": "docs",
  "repo": "optional explicit target; else the handler uses its platform's
           ambient coordinates (GITHUB_REPOSITORY / SYSTEM_TEAMPROJECT…)"
}
```

Facts: `url=…`, `number=…`.

**Idempotency obligation:** the deterministic `head_branch` is the key. If
an open PR with that head exists the handler must *update* it (title/body),
never open a duplicate. The handler must never merge, approve, or bypass
review (INV-10) — a handler with merge capability is non-conforming.

### `handoff` — deliver a finding as a work item

Sent by `watch` (and migration handoff) per actionable finding.

```json
{
  "kind": "handoff",
  "dedup_key": "vendkit-watch-docs",
  "title": "vendkit(docs): update available v1.4.2 → v1.5.0",
  "body_md": "…the rendered report…",
  "slice": "docs"
}
```

Facts: `url=…`.

**Idempotency obligation:** at most one open item per `dedup_key`. If one
exists (GitHub: open issue labelled with the key; ADO: active work item
tagged with it), append the report as a comment — a chronological trail —
rather than creating a sibling.

### `fact-verify` — verify an attested platform fact

Sent by `conformance --verify-attestations` for each `attested` rule result.

```json
{ "kind": "fact-verify", "fact": "required_check_enforced", "slice": "docs",
  "repo": "octo/demo", "branch": "main" }
```

The intent carries a **stable machine `fact` key** (never the human `detail`
prose) plus the coordinate the handler needs to query the platform: `repo`
(else the handler's own env — `GITHUB_REPOSITORY` / `SYSTEM_TEAMPROJECT`),
`branch` (the protected branch; when omitted, GitHub handlers resolve the
repo's default branch), and any fact-specific parameters (e.g. `event` for the
`_enforcement` facts, `check` for a named required status check). The core
composes these from the conformance rule and CLI `--repo` / `--base-branch`;
**core itself calls no vendor API** (conformance spec §4).

Facts: `verdict=true|false|unknown`. `true` promotes the rule to `pass`,
`false` makes it a `fail` (the attestation was wrong — that is a finding,
not an error), `unknown` leaves it `attested`. `unknown` is the non-failing
outcome the handler emits when the token lacks scope (a 401/403 → `unknown`,
**never** `false`), the endpoint is unavailable, a required coordinate is
missing, or the `fact` key is one the handler does not verify
(forward-compatible). Exit 0 for all three verdicts.

The reference handlers verify these facts against the platform API:

| `fact` key | GitHub (`handler github`) | Azure DevOps (`handler ado`) |
|---|---|---|
| `required_check_enforced` | `GET /repos/{repo}/branches/{branch}/protection` → `required_status_checks` present (member `check` if named) | `GET .../_apis/policy/configurations` → an enabled **blocking** Build-validation policy |
| `pull_request_enforcement` | — (tree-decidable on GitHub) | an enabled Build-validation policy (the gate runs on PRs) |
| `required_reviewers_policy` | — (CODEOWNERS is tree-decidable) | an enabled Required-reviewers policy |

Any other key (e.g. `branch_protection_enabled`) is `unknown`. Verification
needs a read-scoped token: `VENDKIT_TOKEN_FACT_VERIFY` (GitHub falls back to
`GITHUB_TOKEN`/`GH_TOKEN` with `repo`/`administration:read`; ADO falls back to
`SYSTEM_ACCESSTOKEN`/`ADO_PAT` with policy read).

### `push-hint` — nudge a subscriber's sync pipeline

Sent by `push-hint` (the publisher-side dispatch step) once per **github-actions**
subscriber after a release is published (platform-integration spec §4, DR-0006).

```json
{
  "kind": "push-hint",
  "repo": "acme/leaf-consumer",
  "event_type": "vendkit-release",
  "client_payload": { "version": "v1.5.0", "tag": "v1.5.0", "publisher": "acme/framework" }
}
```

GitHub: POST `/repos/{repo}/dispatches` with body
`{"event_type": …, "client_payload": …}`, authenticated by the dispatch-scoped
`VENDKIT_TOKEN_PUSH_HINT` (distinct from the PR token; the reference handler
refuses to run without it). Facts: `dispatched=true`, `event_type=…`,
`repo=…`. Azure DevOps: a deliberate **no-op** — the push hint there is the
consumer's own `resources.pipelines` trigger, so the publisher takes no action
(`dispatched=false`, `skipped=ado-pull-trigger`).

Push is a hint, not the reconciler: a lost dispatch costs latency, not
correctness. The *engine's* dispatch step therefore treats a nonzero handler
exit for one subscriber as a warning and continues — but the handler itself
still obeys the protocol (a genuine API failure is a nonzero exit; the engine
decides it is non-fatal *here* because push is best-effort).

## 4. Configuration and resolution

Handlers are wired per slice in the consumer config (onboarding spec §1):

```yaml
handlers:
  pr:          { exec: [vendkit, handler, github] }
  handoff:     { exec: [vendkit, handler, github],
                 dedup_key: vendkit-watch-docs }
  fact-verify: { exec: [vendkit, handler, github] }
```

`vendkit handler <scm>` is the built-in reference handler (same static
binary, DR-0016): it reads the intent on stdin and dispatches on `kind`.
`<scm>` is `github` or `ado`. Any other protocol-honouring executable can
replace it.

Resolution order per kind: `VENDKIT_HANDLER_<KIND>` environment override
(shell-split; `-` → `_`), then the slice config, else **unwired**. Unwired
behaviour is defined per producer and always visible, never silent:

- `pr` unwired → sync stops green after the push with `pr-delivered=false`
  and the full intent on `pr-intent=` — the manual-orchestration workflow
  (`ci: none`), not an error.
- `handoff` unwired → findings are reported (stdout, summary) with
  `handoff=unwired`; exit 0.
- `fact-verify` unwired → `--verify-attestations` is a usage error; without
  the flag, attestation degradation applies as normal.

## 5. Upstream reads are NOT handler territory

Listing a publisher's release tags and reading a file at a tag are plain
**git protocol** operations (`internal/core/upstream.go`): identical against GitHub,
Azure Repos, or a local path, authenticated by ordinary git credentials. No
handler, no vendor API. The only vendor knowledge is the clone-URL template
that expands an `owner/repo` shorthand (`github`) or `org/project/repo`
(`azure-repos`); a full URL or filesystem path is used verbatim.

## 6. Reference handlers

Shipped in-tree, released and conformance-tested with the framework:

| Command | Serves | Notes |
|---|---|---|
| `vendkit handler github` | pr, handoff, fact-verify, push-hint | PR credential refuses `GITHUB_TOKEN` fallback (differences ledger #2); push-hint uses `VENDKIT_TOKEN_PUSH_HINT`; fact-verify reads branch protection (`required_check_enforced`) with `VENDKIT_TOKEN_FACT_VERIFY`/`GITHUB_TOKEN` |
| `vendkit handler ado` | pr, handoff, fact-verify, push-hint | needs `VENDKIT_ADO_ORG_URL`; Basic-auth PAT / `SYSTEM_ACCESSTOKEN`; push-hint is a no-op (pull trigger); fact-verify reads branch policies (`required_reviewers_policy`, `pull_request_enforcement`, `required_check_enforced`) |

`github` / `ado` are built into the engine binary (DR-0016) — no separate
install, no interpreter. The neutral journal handler used by the scenario
kit (`internal/e2e/journalhandler`) records intents to
`VENDKIT_NEUTRAL_JOURNAL` and is the template for new handlers.

Writing a third handler (Jira, Slack, GitLab…) = one executable + a
`handlers:` edit in the slice config. The scenario kit's handler tests run
against the journal handler; a new handler can be smoke-tested by pointing
`VENDKIT_HANDLER_HANDOFF` at it and running `vendkit watch`.

## 7. Security posture

Handlers run with the credentials the consumer's pipeline grants them —
least scope per purpose (security model §4). The engine passes no secrets in
the intent document; credentials travel by environment only. A handler is
consumer-configured, consumer-audited code: it lives in the consumer's trust
boundary like any other pipeline step, and the reference handlers are
manifest-tracked, gate-verified files when the machinery slice is vendored.
