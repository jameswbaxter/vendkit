# Spec: Release watch

Status: draft for implementation · Owner: Layer 0 (compare) + Layer 1 (ref listing, handoff)

The sync lane refreshes content once a pin is advanced, but never *detects* a
new upstream release. Watch closes that gap: a scheduled job in the consumer
that compares each vendored slice's pin against the publisher's latest
qualifying release and hands findings off for remediation.

## 1. Configuration

Watch has **no config file of its own**: it iterates every consumer slice config
(`.vendkit/*.yml`, see onboarding spec), each of which carries the fields watch
needs:

```yaml
# inside .vendkit/docs.yml
publisher:
  platform: github            # selects the port for ref listing
  repo: example-org/design-docs
pin:
  file: .github/workflows/docs-sync.yml
  pattern: "example-org/design-docs@v"   # anchored literal; first match wins
watch:
  channel: stable             # stable | rc
  handoff:                    # where findings become work; see §3
    kind: issue               # issue (GitHub) | workitem (ADO)
    dedup_key: "vendkit-watch-docs"
```

This removes the separate watch-config file and its lockstep risk: adding a
slice automatically adds its watch entry.

## 2. Collector contract

`vendkit watch [--slice <name>] [--dry-run] [--report-out <md>] [--json-out <json>]`

Per slice:

1. **PINNED** — scan `pin.file` for the first line containing `pin.pattern`
   immediately followed by a parsable version. Pattern present but no version →
   `finding: pin-unreadable` (config rot must be loud, not skipped).
2. **LATEST** — list the publisher's tags via the port
   (`releases.list(publisher)`), filter by the channel grammar, drop retracted
   versions (read from the newest release's declaration), take the maximum.
   No qualifying tag → benign skip (`no-releases`).
3. **Provenance check** — if the consumer manifest records `source.commit`,
   verify the pinned tag still resolves to it; mismatch → `finding: tag-moved`
   (severity above everything else; see security model).
4. **Compare** — `pinned < latest` → `finding: update-available` with bump class
   (`patch|minor|major`) and, when the annotation carries them (releases spec
   §3), exact surface-delta counts.

Exit codes: **0 = ran successfully** (findings or not — findings are data, not
failure); 1 = infrastructure/config error. Machine callers read
`findings=<n>` and the JSON document; they never infer state from the exit code
beyond success/failure.

`--dry-run` performs no network calls and emits an empty successful report —
this is the consumer's PR-time self-test (no credentials needed).

## 3. Handoff

Findings become exactly one open work item per slice, via the port's
`workitem.upsert` (ADO) / `issue.upsert` (GitHub):

- **Dedup:** search for an open item tagged/labelled with `dedup_key`; if found,
  append a comment with the new run's findings (chronological trail); else
  create, with title `vendkit(<slice>): update available v<PINNED> → v<LATEST>`,
  the report as body, and routing fields from port configuration (area path /
  labels / assignees).
- `tag-moved` and `pin-unreadable` findings create items regardless of
  `update-available`, under distinct dedup keys (`<dedup_key>-integrity`).
- Optional escalation: platforms with AI coding agents may assign the item for
  automated pin-bump; the sync PR that results still passes normal review
  (INV-10).

## 4. Cadence and interplay with push hints

Watch cadence is a consumer decision (scaffold default: weekly). Where the push
hint is wired (publisher release completion triggers the sync pipeline —
platform-ports spec §4), watch degrades gracefully into the safety net for
missed events and for **tier-chain visibility** (an upstream-of-upstream release
does not push to this consumer). Watch is mandatory in the conformance core
rules; push hints are optional.
