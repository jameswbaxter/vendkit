---
id: VK-TS-release-watch
title: "Release detection and handoff"
status: current
status_since: "2026-07-08"
last_verified: "2026-07-08"
spec_layer: technical_spec
summary: "A scheduled consumer job compares each slice's pin against the publisher's latest qualifying release over the git protocol, producing findings that a handoff handler turns into work."
relations:
  realizes:
    - VK-FS-releases-and-versioning
  regulated_by:
    - VK-STD-invariants
---

# Spec: Release watch

## Scope

The sync lane refreshes content once a pin is advanced, but never *detects* a
new upstream release. Watch closes that gap: a scheduled job in the consumer
that compares each vendored slice's pin against the publisher's latest
qualifying release. **Watch is pure detection** (DR-0014): it produces
findings; turning findings into tickets is the handoff handler's job (§3).

## Design

### 1. Configuration

Watch has **no config file of its own**: it iterates every consumer slice config
(`.vendkit/consumer/*.yml`, see onboarding spec), each of which carries the fields watch
needs:

```yaml
# inside .vendkit/consumer/docs.yml
publisher:
  scm: github                 # shorthand-expansion hint for the clone URL
  repo: example-org/design-docs
pin:
  pattern: "example-org/design-docs@v"   # anchored literal; first match wins
  files: [.github/workflows/docs-sync.yml, …]   # first entry is read
watch:
  channel: stable             # stable | rc
handlers:
  handoff:                    # where findings become work; see §3
    exec: [vendkit, handler, github]
    dedup_key: "vendkit-watch-docs"
```

This removes the separate watch-config file and its lockstep risk: adding a
slice automatically adds its watch entry.

### 2. Collector contract

`vendkit watch [--slice <name>] [--dry-run] [--no-handoff]`

Per slice:

1. **PINNED** — scan `pin.files[0]` for the first line containing
   `pin.pattern` immediately followed by a parsable version. Pattern present
   but no version → `finding: pin-unreadable` (config rot must be loud, not
   skipped). Under `ci: none` there are no pin lines: the manifest's
   `source.release` is read instead.
2. **LATEST** — list the publisher's tags over the **git protocol**
   (`git ls-remote --tags`, annotated tags peeled — no vendor API, DR-0015),
   filter by the channel grammar, drop retracted versions (read from the
   newest release's declaration via a depth-1 fetch), take the maximum.
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

### 3. Handoff

Each actionable finding becomes one `handoff` intent to the configured
handler (handler-protocol spec §3), which owns the vendor API and the
idempotency contract:

- **Dedup (handler obligation):** at most one open item per `dedup_key`; an
  existing item gets the new run's report as a comment (chronological
  trail); else create, with title `vendkit(<slice>): update available
  v<PINNED> → v<LATEST>` and the report as body. Routing (labels, area
  paths, assignees) is the handler's own concern — core carries none.
- `tag-moved` and `pin-unreadable` findings are handed off regardless of
  `update-available`, under distinct dedup keys (`<dedup_key>-integrity`).
- **Unwired handler** → findings are still reported (stdout, summary,
  `handoff=unwired`) and the run exits 0 — report-only mode, the `ci: none`
  default.
- Optional escalation: platforms with AI coding agents may assign the item for
  automated pin-bump; the sync PR that results still passes normal review
  (INV-10).

### 4. Cadence and interplay with push hints

Watch cadence is a consumer decision (scaffold default: weekly). Where the push
hint is wired (publisher release completion triggers the sync pipeline —
platform-integration spec §4), watch degrades gracefully into the safety net for
missed events and for **tier-chain visibility** (an upstream-of-upstream release
does not push to this consumer). Watch is mandatory in the conformance core
rules; push hints are optional.

## Conformance

Watch is mandatory in the conformance core rules, so its correctness is
checked in three places.

- **Findings are data, not failure.** Exit 0 means the run succeeded whether
  or not it found anything; exit 1 is an infrastructure or config error.
  Machine callers read `findings=<n>` and never infer state from the exit
  code beyond success or failure.
- **Config rot is loud.** A pin pattern present with no parsable version
  raises `pin-unreadable` rather than skipping the slice, because a watch
  that quietly stops watching is worse than one that fails.
- **The dry run is the PR-time self-test.** `--dry-run` makes no network
  call, needs no credentials, and emits an empty successful report, so a
  consumer PR can prove the wiring without reaching the publisher.
- **The wiring itself is a conformance rule.** `pipeline-wired` for the
  watch component checks that a pinned pipeline invokes it on a schedule,
  and reports `skipped` under `ci: none` so manual mode is visible rather
  than hidden.
