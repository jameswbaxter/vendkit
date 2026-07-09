# Spec: Export declaration

Status: draft for implementation · Schema version: 1 · Owner: Layer 0

The export declaration is the publisher-side YAML file that fully defines a
slice. **All slice identity lives here; the tools carry none** (DR-0002). One
declaration per slice; a repository publishing two slices has two declarations.

Default filename: `vendkit-export.yml` at the publisher repo root. Every CLI
command accepts `--export-decl <path>`.

## 1. Schema

```yaml
schema_version: 1

slice:
  name: docs                  # REQUIRED. Slug: [a-z][a-z0-9-]{0,15}. Namespaces every
                              # consumer artefact: .vendkit/docs.yml, docs-manifest.json,
                              # docs-sync pipeline, gate findings labels.
  title: "Design docs"        # Optional display title (reports, PR titles). Default: name.

publisher:
  # REQUIRED. Coordinates of this repo, used by scaffolded consumer
  # pipelines and watch entries. `scm` is provenance and shorthand-expansion
  # metadata only — core never branches on it (DR-0015). `repo` is a
  # shorthand (github: owner/repo; azure-repos: org/project/repo) or any
  # git-cloneable URL/path, used verbatim.
  scm: github                 # github | azure-repos
  repo: example-org/design-docs

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
```

## 2. Semantics

- **Include/exclude.** `include` uses pathlib-style globbing anchored at the
  repo root; `exclude` uses fnmatch against the resulting repo-relative paths.
  The exported surface is `matched(include) − matched(exclude)`, deduplicated,
  sorted. Directories are never entries; only regular files. Symlinks are
  rejected at generate time (they cannot be identity-copied portably).
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
