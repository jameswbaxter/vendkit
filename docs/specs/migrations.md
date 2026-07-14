# Spec: Migrations

Status: stable (frozen at v1.0.0) · Owner: Layer 0 (resolve/verify) + Layer 1 (handoff)

Mechanical sync refreshes manifest-tracked files only. A release that also
invalidates **consumer-owned** content — renamed trees, retired conventions,
retargeted glob catalogues — ships a declarative migration payload. Migrations
carry *what must become true*, expressed as machine-checkable obligations; *how*
is left to the consumer (human or AI agent), and a deterministic verifier gates
the result. No migration contains executable code.

## 1. Payload schema

One YAML file per migration under `migrations/` in the publisher repo.
`migrations/` is deliberately **outside the export surface** — payloads
instruct consumers; they are not vendored content.

```yaml
schema_version: 1
id: 2026-07-drop-drafts-tier          # stable, never renamed
applies_from: v2.0.0                  # first release whose adoption requires it
kind: structural                      # mechanical | additive | removal | structural | convention
profiles: ["*"]                       # or a list of profile names
summary: "The docs/drafts tier is retired; content moves to docs/proposals."
rationale: >-
  One paragraph of durable why. May link a design record by URL.
detection:                            # blast-radius probes, run before handoff;
  - glob: "docs/drafts/**"            #   results attached to the work item
  - grep: "\\(\\.\\./drafts/"
instructions: >-                      # natural-language brief for the remediator
  Move each file under docs/drafts/ to docs/proposals/, update inbound links,
  and delete the empty tree.
verification:                         # ≥1 obligation REQUIRED
  must_be_absent: ["docs/drafts/**"]
  must_be_present: ["docs/proposals/**"]
  checks:                             # named checks from the conformance detector
    - kind: file-absent               #   registry — no arbitrary shell (see §5)
      path: "docs/drafts"
```

`kind` semantics: `mechanical`/`additive`/`removal` document changes the sync
lane already performs (informational; excluded from handoff by default);
`structural`/`convention` require consumer judgment and drive the lifecycle
below.

## 2. Resolve

`vendkit migrations --pinned <v> --target <v> [--profile <p>] [--json]`

Selects entries where `kind` is judgment-bearing (default filter), `pinned <
applies_from <= target` (the *window*), and the profile matches (`"*"` matches
all; an unbound consumer matches only `"*"` entries). Obligations across
selected entries are unioned. Output: `count=`, `ids=`, and a JSON document of
applicable entries + aggregated verification.

## 3. Handoff

Resolved migrations render into one `handoff` intent for the configured
handler (handler-protocol spec §3, dedup key
`vendkit-migrate-<slice>-<target>`): per-migration summary, kind,
rationale, detection hits from the consumer tree, instructions, and the
aggregated **definition of done** — the verification obligations plus the exact
verifier invocation. Platforms with AI coding agents may assign the item; the
remediation PR is ordinary and reviewed (INV-10).

## 4. Verify

`vendkit migrations-verify --obligations <json> [--consumer-root <path>]`

Deterministic, over **tracked files only** (`git ls-files`):

- every `must_be_absent` glob matches zero files;
- every `must_be_present` glob matches at least one;
- every named check passes.

Zero obligations is a green no-op — so the verifier is safe to wire as a
required check on every PR (it gates remediation PRs and costs nothing
elsewhere). The glob matcher is the same implementation the resolver uses
(single matching semantics; part of the conformance kit contract).

The sync PR body lists the window's applicable migrations (sync spec §3), so a
consumer reviewing a version bump sees its judgment-bearing consequences in the
same view.

## 5. No arbitrary shell in obligations

Obligations are declarative globs plus **named checks** drawn from the same
registry as conformance detectors. A publisher needing a custom check ships it
as a vendored tool and references it by a `tool` check kind
(`{kind: tool, path: tools/lint/check-x, args: […]}`) — which executes a
*manifest-tracked, gate-verified* file, not an inline string. Rationale: inline
shell from upstream is an unnecessary second code-execution channel with no
integrity anchor; vendored tools are already inside the trust boundary
(security model §1).

## 6. Publisher discipline

The release command's migration pre-gate (releases spec §3) enforces: a MAJOR
release (or any surface removal / adapter change) must ship a matching
`migrations/` entry or an explicit recorded override. Windows compose across
multi-version jumps: a consumer syncing v1.3 → v3.0 resolves every entry in
`(v1.3, v3.0]`, in `applies_from` order.
