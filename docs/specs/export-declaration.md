# Spec: Export declaration

Status: stable (frozen at v1.0.0) · Schema version: 1 · Owner: Layer 0

The export declaration is the publisher-side YAML file that fully defines a
slice. **All slice identity lives here; the tools carry none** (DR-0002). One
declaration per slice; a repository publishing two slices has two declarations.

Default location: `.vendkit/publisher/export-declaration.yml` — publisher-side
configuration sits alongside the consumer control plane under one `.vendkit/`
root, in its own half (DR-0019). Every CLI command accepts `--export-decl <path>`;
a relative path is resolved against the publisher root (`--publisher-root`, or
`--root` for `generate`), so a relocated declaration resolves identically whatever
the working directory.

## 1. Schema

```yaml
schema_version: 1

slice:
  name: docs                  # REQUIRED. Slug: [a-z][a-z0-9-]{0,15}. Namespaces every
                              # consumer artefact: .vendkit/consumer/docs.yml, docs-manifest.json,
                              # docs-sync pipeline, gate findings labels.
  title: "Design docs"        # Optional display title (reports, PR titles). Default: name.
  aliases:                    # Optional. Names this publisher is known by in prose.
    - design-docs             # Vendored into each consumer's slice config, so a
    - "the docs framework"    # consumer can recognise an upward reference to this
                              # publisher without a hand-maintained registry of the
                              # whole family (DR-0019). Self-declared: a publisher
                              # names itself, never its downstreams.

publisher:
  # REQUIRED. Coordinates of this repo, used by scaffolded consumer
  # pipelines and watch entries. `scm` is provenance and shorthand-expansion
  # metadata only — core never branches on it (DR-0015). `repo` is a
  # shorthand (github: owner/repo; azure-repos: org/project/repo) or any
  # git-cloneable URL/path, used verbatim.
  scm: github                 # github | azure-repos
  repo: example-org/design-docs
  manifest_dir: .governance   # Optional. Publisher-local directory holding the
                              # generated manifest. Default: `.vendkit/publisher`;
                              # an explicit "." opts back out to the repo root.
                              # Must be a relative path inside the repo.
                              # PUBLISHER-SIDE ONLY — it never travels with the
                              # slice: a vendored manifest always lands at
                              # `.vendkit/consumer/<manifest_name>` in the consumer.

include:                      # Anchored, repo-relative glob patterns.
  - "docs/standards/**/*.md"  # `**` matches zero or more directories.
  - "tools/lint/**/*"
seed:                         # Optional. Scaffold-once templates (DR-0013):
  - "templates/*.md"          # materialised only when the consumer path does
                              # not exist, then consumer-owned and free to
                              # diverge. Must be disjoint from include (a
                              # path matched by both is a hard error). At
                              # least one of include/seed must be non-empty.
exclude:                      # Optional. Applied to include AND seed results.
  - "**/TEMPLATE.md"
  - "**/tests/**"

adapters:                     # Optional. Named content transforms (DR-0009).
  # Everything not matched by an adapter is an identity copy: verbatim bytes,
  # identical path at every hop. Adapters are the ONLY place file content or
  # location may lawfully differ from the publisher tree.
  - kind: prefix-namespace    # rename files into a reserved namespace
    match: ".github/instructions/*.md"
    prefix: "docs-"           # consumer_path gets the prefix; never shadows local files
  - kind: glob-localise       # prune a declared glob list per consumer profile
    match: ".github/instructions/*.md"
    field: applyTo            # front-matter key holding the glob union
    catalogue:                # profile-owned globs; see §3
      code-repo: ["docs/standards/**", "docs/specifications/**"]
      solution-docs: ["docs/applications/**", "docs/domain/**"]

profiles:                     # Optional. Consumer archetypes.
  code-repo:
    export_slice:             # which subset of the surface this archetype vendors
      include: ["*"]          # fnmatch against exported repo-relative paths
      exclude: ["tools/onboard/*"]
  solution-docs: {}           # empty profile: whole surface, no adapter params

retracted:                    # Optional. Released versions consumers must not adopt.
  - v0.9.0                    # watch skips; sync refuses as target (see releases spec)

manifest_name: docs-manifest.json   # Optional. Default: "<slice.name>-manifest.json".
                                    # A BARE FILENAME: it also names the consumer
                                    # copy under `.vendkit/`, so a path here is a
                                    # hard error — relocate the publisher copy
                                    # with `publisher.manifest_dir`.
```

## 2. Semantics

- **Include/exclude.** `include` uses pathlib-style globbing anchored at the
  repo root; `exclude` uses fnmatch against the resulting repo-relative paths.
  The exported surface is `matched(include) − matched(exclude)`, deduplicated,
  sorted. Directories are never entries; only regular files. Symlinks are
  rejected at generate time (they cannot be identity-copied portably).
- **Aliases.** `slice.aliases` is the publisher's self-declaration: the names it
  answers to in prose. It is copied into each consumer's slice config at onboard
  and sync time, so a consumer's set of legal upward references is the union of
  what its upstreams say about themselves. No repository declares another
  repository's identity, and nothing enumerates downstreams.
- **Manifest location.** `manifest_name` is the slice's manifest filename on both
  sides; `publisher.manifest_dir` relocates only the publisher's generated copy
  (`<manifest_dir>/<manifest_name>`), which is where `generate --check` and the
  release freshness and surface-delta gates read it — including out of git history
  at the previous tag. The consumer copy is always `.vendkit/consumer/<manifest_name>`, so
  `--all` gate discovery and multi-slice coexistence are unaffected.
- **Seed.** Same glob and exclusion semantics as `include`, producing the
  scaffold-once surface (DR-0013, sync spec §6). The two surfaces must be
  disjoint; overlap is a generate-time hard error. Seeds flow through the
  same adapters (a `prefix-namespace` rename applies; `glob-localise` runs
  once at seed time) and the same profile `export_slice` scoping.
- **Determinism.** The exported set and all adapter outputs depend only on the
  declaration and the tree (INV-2). Generate on the same tree is byte-stable.
- **Adapters** apply in declaration order; at most one `prefix-namespace` and at
  most one `glob-localise` may match a given file (generate fails otherwise).
  Adapter `kind`s are an extension point but v1 ships exactly these two; unknown
  kinds are a hard generate-time error, so a consumer engine can trust that a
  manifest it reads was produced by adapters it understands.
- **Profiles.** A profile with no `export_slice` takes the whole surface. A
  consumer binds to at most one profile (in its slice config); an unbound
  consumer takes the whole surface and verbatim adapter output (no
  localisation). `export_slice` affects **scope reconciliation only** — it never
  narrows an already-tracked consumer slice (INV-4).
- **Validation.** `vendkit generate --check` and the publisher CI must fail on:
  unknown keys, unknown adapter kinds, empty export set, adapter match
  collisions, a `retracted` entry that is not release-shaped, or a `slice.name`
  that fails the slug rule.

## 3. Glob-localise catalogue rules

A glob listed under a profile in a `glob-localise` catalogue is *owned* by that
profile. When materialising for a consumer bound to profile P, the adapter keeps
a glob in the file's declared union iff it is owned by P or owned by no profile
(universal). Globs owned only by other profiles are dropped. The pruned result
is what the consumer manifest hashes — so localisation is drift-gate-safe.

`glob-localise` is the only adapter that transforms content, and both of its
failure modes are silent to publisher and consumer alike: over-pruning ships a
rule that matches nothing, under-pruning ships globs for shelves the consumer
does not have. The two subsections below are the engine-level verification of
that transformation (issue #10).

### 3.1 Declaration-validity findings (`generate --check`)

`vendkit generate --check` evaluates consistency rules over each
`glob-localise` adapter against the matched tree — exported and seeded files
both, since both flow through adapters (§2). No fixture or external input is
involved; this is the declaration checked against itself and the tree.

| Rule | Condition |
|---|---|
| `profile-unknown` | a catalogue key that is not a declared profile |
| `glob-uncatalogued` | a glob in a matched rule's field that no catalogue entry claims — it vendors to every profile (universal); deliberate universals may stay, the finding is informational |
| `catalogue-glob-orphan` | a catalogue glob appearing in no matched rule's field (dead entry) |
| `localisation-empty` | a rule whose localised field for some profile would be **empty** — it arrives matching nothing and can never load |

Findings are **advisory**: printed, counted as `localisation-findings=<n>`
(full documents under `--json`), and never the exit code, so a currently-green
declaration does not turn red. The checks parse the field exactly as the
adapter does, and `localisation-empty` applies the actual transform, so they
cannot disagree with materialisation.

### 3.2 The expectation oracle — `vendkit verify-localisation`

```
vendkit verify-localisation --expected <file> [--profile P] [--consumer-root DIR] [--write]
```

The publisher declares the *intended* per-profile, per-rule localised field
values in a file it owns:

```yaml
schema_version: 1
expectations:
  code-repo:                              # profile
    ".github/instructions/std.md":        # publisher repo-relative rule file
      - "docs/standards/**"               # the localised field, in order
```

The command materialises each profile's output in memory through the real
adapter chain (or, with `--consumer-root`, reads an already-materialised
consumer tree — the profile then comes from `--profile` or the consumer's
slice config) and diffs actual against expected. Findings, each exit-1:

| Finding | Meaning |
|---|---|
| `mismatch` | the localised field differs from the expectation (ordered comparison) |
| `rule-absent` | an expected rule has no localised field — not in the exported surface or profile scope, or the field is missing |
| `expectation-stale` | a localised rule has no expectation entry — the expected file has fallen behind the surface |

The oracle covers the drift-gated (exported) surface only; seeds are
consumer-owned after materialisation and lawfully diverge. Without
`--profile`, all profiles in declared ∪ expected are verified.

Two constraints are load-bearing:

- **The expected file is publisher-authored input, never engine-derived at
  check time.** `--write` refreshes it from current engine output as a
  deliberate, reviewed step; deriving it while checking would compare the
  engine against itself and always pass.
- **No predictor lives in the engine.** An independent reimplementation of
  the pruning that computes expected output from the catalogue must stay with
  the publisher that wants one: a predictor sharing a codebase with the
  adapter agrees with the adapter's bugs by construction. VendKit owns the
  harness and the diff; the oracle values are input.

## 4. What the declaration must never contain

- Consumer identities or any downstream registry (the publisher does not know
  its consumers; see DR-0006 for the one optional exception, push hints).
- Credentials, tokens, org-internal URLs.
- Platform pipeline logic (that is Layer 2's job).

## 5. Open questions

- OQ-1: should `include` support an explicit single-file form with a required
  flag (fail if missing) to catch typos? Leaning yes: `- path: docs/x.md`
  object form, `required: true` default.
- OQ-2: adapter for line-ending forcing on materialise (consumers with
  `.gitattributes` quirks) or is normalisation-at-hash enough? Leaning: hash
  normalisation is enough; do not mutate bytes.
