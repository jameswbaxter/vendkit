# Spec: Sync lane and materialisation

Status: draft for implementation · Owner: Layer 0 (materialise), Layer 1 (PR), Layer 2 (pipeline)

The sync lane keeps a consumer's vendored slice fresh: it detects that the pin
is behind a target release, rewrites the tracked files and manifest from that
release, and opens **one reviewed PR**. It never merges, never deletes files
from disk, and never changes scope silently (INV-4, INV-10).

## 1. Version model

Per slice, two versions exist during a sync:

- **PINNED** — the release currently vendored. The consumer manifest's
  `source.release` is authoritative (it records what was actually
  materialised); the pin location declared in the slice config (`pin.file` +
  `pin.pattern`) is the bootstrap fallback before the first sync, and is what
  *watch* reads as the consumer's intent. The pin is the platform-native
  reference line — an ADO repository-resource `ref: refs/tags/vX.Y.Z` or a
  GHA checkout `ref:` — never a separate version file.
- **TARGET** — the release the sync pipeline itself is pinned to. Because the
  pipeline reference resolves the publisher tree at TARGET, **the checkout that
  supplies the content also supplies the engine that materialises it** (INV-6).

`vendkit is-newer --pinned <v> --target <v>` is the pure comparison (strict
SemVer; see releases spec). Retracted targets are refused (exit 3,
`refused=retracted`).

## 2. Materialise semantics

`vendkit sync --check|--apply --consumer-root <path> --pinned <v> --target <v>
[--reconcile-scope] [--porcelain]`

Given the target release tree (the engine's own checkout), the export
declaration in it, the consumer's current manifest and bound profile:

1. **Tracked refresh (default).** For every manifest entry whose `path` is still
   exported by the target: run adapters, write bytes to `consumer_path`, set the
   exec bit to the source's (INV-2 purity: output depends only on the inputs
   listed above). Classification is against the **consumer working tree**, not
   the recorded hash — so a locally drifted or missing file simply counts as
   `updated`, and `--check` predicts `--apply` exactly (INV-3).
2. **Removals.** A tracked `path` no longer exported by the target is reported
   `removed-upstream` and dropped from the refreshed manifest; the vendored file
   is **left on disk** for the PR to delete under review.
3. **Additions (opt-in).** With `--reconcile-scope`, exported files inside the
   consumer profile's `export_slice` that are not yet tracked are materialised,
   added to the manifest, and reported `added`. Reconciliation can only widen
   the tracked slice, never narrow it.
4. **Manifest rewrite.** The consumer manifest is regenerated as a projection of
   the target's export set filtered to the tracked slice, with per-entry hashes
   computed from the **vendored** (post-adapter) bytes, and `source:` set to
   `{platform, repo, release: TARGET, commit: <resolved SHA>}`.

Report classes: `updated`, `removed-upstream`, `added`, `seeded`,
`seed-retired`, `template-updated`, plus the summary `changed=true|false`
(`changed` = any class except `template-updated` non-empty; see §6).

**Porcelain contract (INV-3).** With `--porcelain`, a successful `--check` run
always exits 0 and prints exactly one `changed=` line; machine callers treat any
non-zero exit as an infrastructure failure, never as "changes exist".

## 3. Pipeline behaviour (Layer 2, per platform)

The scaffolded sync pipeline, on both platforms:

1. Resolve PINNED from the pin location; TARGET from its own reference.
2. `is-newer` — if not newer, emit `update-available=false` and stop green.
3. `sync --check --porcelain` — if `changed=false`, stop green.
4. `sync --apply`, re-assert exec bits, advance the pin line(s) in every file
   listed under `pin.files` (an anchored string substitution of the old release
   for the new), commit on branch `vendkit/<slice>/sync-v<PINNED>-to-v<TARGET>`,
   push, open the PR via the port.
5. **PR idempotency:** before opening, query for an existing open PR with the
   same head branch; if present, update it (force-push the branch) rather than
   opening a duplicate. The deterministic branch name is the idempotency key.

PR content: title `sync(<slice>): v<PINNED> → v<TARGET>`; body lists the report
classes, the resolved source commit, retraction/channel notes, and links the
release. The PR is never auto-merged and requires the consumer's normal review
(INV-10). Where the platform supports it, the sync PR also surfaces applicable
**migrations** for the window (see migrations spec §4).

Triggering: `schedule` (cron, consumer-chosen cadence) plus the optional push
hint (publisher release completion → run now; see platform-ports spec §4).
Both trigger paths run the identical pipeline — push is a latency optimisation,
pull remains the reconciler (DR-0006).

## 4. Content adapters at materialise time

Adapters (declared in the export declaration, DR-0009) run inside step 1/3:

- `prefix-namespace` — writes the file to its prefixed `consumer_path`.
- `glob-localise` — prunes the declared glob union to the consumer's profile
  before hashing/writing, per the catalogue rules in the declaration spec §3.

Everything else is an identity copy. Adapters must be deterministic and
byte-stable (INV-2); an adapter that needs consumer context may read only the
bound profile name and adapter parameters — never the consumer tree.

## 5. Failure and race behaviour

- Two concurrent syncs of one slice converge on the same branch name; the
  second force-push wins with identical content (materialise is pure) — worst
  case is a no-op update, never corruption.
- A sync racing a consumer PR that touches vendored paths resolves at merge
  time: whichever merges second re-runs the gate; a stale sync PR goes red and
  is refreshed by the next scheduled run.
- A target tag deleted mid-sync fails the checkout — loud infrastructure
  failure, by design (INV-5; see security model).
- Credential loss (push/PR denied) must fail the pipeline red — the sync lane
  is only trustworthy if its failure is visible (see security model §4).

## 6. Seeded files (scaffold-once — DR-0013)

Paths under the declaration's `seed:` surface are templates handed out once,
then consumer-owned. Semantics, all decided by the manifest entry (which is
the "seeding happened" record — no other state):

- **Untracked seed path** (reconcile only): target absent → write it and add a
  `seed: true` entry (`seeded` report class); target **already exists** →
  *adopt without writing* — add the entry, never clobber the consumer's file.
- **Tracked seed entry**: never written again, whether the file exists, has
  diverged, or was deleted — deletion is respected. The entry carries the
  template's current hash; when it differs from the stored one **and** the
  consumer's file exists, the path is reported `template-updated` —
  informational only, never sets `changed`, and (when the slice config's
  `seeds.notes` is `informational`, the default) listed in the next real sync
  PR's body. `seeds.notes: silent` suppresses the note.
- **Template retired upstream**: the entry is dropped (`seed-retired`); the
  consumer's copy is untouched and simply stops being tracked. Seed removals
  are exempt from the release command's MAJOR-bump and migration pre-gates.
- **Re-seed escape hatch**: remove the entry from the manifest; the next
  reconcile re-offers the seed as a reviewed PR addition.
- The gate never hash-checks seed entries but they still claim their
  `consumer_path` (INV-7). INV-1 is unaffected: the gate skips exactly what
  sync refuses to write. Template hashes refresh only when a sync PR actually
  ships, so a `template-updated` note may repeat until the next real sync.
