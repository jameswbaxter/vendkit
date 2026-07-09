# Spec: Handler protocol

Status: draft for implementation · Protocol version: 1 · Owner: Layer 1

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
  "kind": "pr" | "handoff" | "fact-verify",
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
{ "kind": "fact-verify", "fact": "required_check_enforced", "slice": "docs" }
```

Facts: `verdict=true|false|unknown`. `true` promotes the rule to `pass`,
`false` makes it a `fail` (the attestation was wrong — that is a finding,
not an error), `unknown` leaves it `attested` (insufficient API scopes is a
normal, non-failing outcome). Exit 0 for all three verdicts.

## 4. Configuration and resolution

Handlers are wired per slice in the consumer config (onboarding spec §1):

```yaml
handlers:
  pr:          { exec: [python3, -m, vendkit.handlers.github] }
  handoff:     { exec: [python3, -m, vendkit.handlers.github],
                 dedup_key: vendkit-watch-docs }
  fact-verify: { exec: [python3, -m, vendkit.handlers.github] }
```

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
**git protocol** operations (`core/upstream.py`): identical against GitHub,
Azure Repos, or a local path, authenticated by ordinary git credentials. No
handler, no vendor API. The only vendor knowledge is the clone-URL template
that expands an `owner/repo` shorthand (`github`) or `org/project/repo`
(`azure-repos`); a full URL or filesystem path is used verbatim.

## 6. Reference handlers

Shipped in-tree, released and conformance-tested with the framework:

| Module | Serves | Notes |
|---|---|---|
| `vendkit.handlers.github` | pr, handoff, fact-verify | PR credential refuses `GITHUB_TOKEN` fallback (differences ledger #2) |
| `vendkit.handlers.ado` | pr, handoff, fact-verify | needs `VENDKIT_ADO_ORG_URL`; Basic-auth PAT / `SYSTEM_ACCESSTOKEN` |
| `vendkit.handlers.journal` | all | records intents to `VENDKIT_NEUTRAL_JOURNAL` (or stderr); the scenario kit's assertion point and the template for new handlers |

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
