# Spec: Manifest and gate lane

Status: stable (frozen at v1.0.0) · Manifest schema version: 1 · Owner: Layer 0

The manifest is the integrity contract between publisher and consumer. The gate
lane is its enforcement point: a check on every consumer PR that re-hashes
vendored files and fails on any hand-edit or deletion.

## 1. Manifest schema (v1)

One manifest per slice. Publisher-side it lives at the repo root under
`manifest_name`; consumer-side at `.vendkit/<manifest_name>`.

```json
{
  "schema_version": 1,
  "slice": "docs",
  "profile": "code-repo",
  "source": {
    "scm": "github",
    "repo": "example-org/design-docs",
    "release": "v1.4.2",
    "commit": "8c5f2f0e…40-hex…"
  },
  "normalisation": "utf8;lf;strip-trailing-ws;single-final-newline;sha256",
  "entries": [
    {
      "path": "docs/standards/writing.md",
      "consumer_path": "docs/standards/writing.md",
      "sha256": "…64-hex…",
      "exec": false,
      "raw": false
    }
  ]
}
```

Field rules:

- `slice` — the declaration's `slice.name`. The gate uses it in findings; tools
  use it to pair manifest ↔ consumer config.
- `profile` — publisher-side: the literal string `"*"` (whole surface).
  Consumer-side: the bound profile name, or `"*"` if unbound.
- `source` — **consumer-side only** (absent in the publisher manifest, which
  describes the working tree, not a release). Written by materialise from the
  resolved target: `release` is the tag, `commit` the SHA it resolved to at sync
  time; `scm` is provenance metadata (DR-0015). This is the tamper-evidence
  anchor (INV-5; see security model §3), and under `ci: none` the `release`
  field is also the slice's pin (sync spec §1).
- `normalisation` — the canonicalisation recipe identifier. v1 defines exactly
  one recipe (below). A future recipe is a new string and a schema bump.

**Canonical serialisation** (normative — `generate --check` compares
serialised bytes, and any future second engine implementation must reproduce
them exactly): JSON with keys sorted lexicographically at every level,
2-space indentation, no trailing whitespace, `ensure_ascii` escaping of
non-ASCII, no HTML escaping of `<`/`>`/`&`, and a single trailing newline.
- `entries` — sorted by `path`, unique on `path` **and** on `consumer_path`.
  - `path` — publisher-repo-relative source path.
  - `consumer_path` — where the file lands in the consumer. Differs from `path`
    only via a `prefix-namespace` adapter.
  - `sha256` — normalised checksum of the file **as vendored** (i.e. after
    adapters), so the consumer manifest is self-consistent with the consumer
    tree.
  - `exec` — POSIX executable bit as vendored. Materialise sets it; the gate
    verifies it (a chmod is drift).
  - `raw` — `true` when the file is not valid UTF-8; the hash is then over raw
    bytes with no normalisation. Text/binary is decided at generate time and
    recorded, never re-guessed at verify time.
  - `seed` — optional, `true` for scaffold-once entries (DR-0013). The entry
    is the "seeding happened" lifecycle record; its `sha256` is the
    **template's** hash (publisher-side at publish, consumer-side as of the
    last shipped sync) — a divergence-note comparator, never a gate input.

## 2. Normalisation recipe v1

`utf8;lf;strip-trailing-ws;single-final-newline;sha256`:

1. Decode UTF-8 (strict). If decoding fails → `raw: true`, hash raw bytes, stop.
2. Convert CRLF and lone CR to LF.
3. Strip trailing spaces/tabs from every line.
4. Collapse trailing newlines to exactly one; add one if absent.
5. SHA-256 the re-encoded UTF-8 bytes.

Rationale: a consumer's checkout/autocrlf settings can never masquerade as
drift, while any substantive edit changes the hash (DR-0004).

## 3. Publisher operations

- `vendkit generate` — build the manifest from the declaration + working tree.
- `vendkit generate --check` — fail (exit 1) if the committed manifest differs
  from a fresh build. Wired into publisher PR CI and as the **release freshness
  pre-gate**: a release cannot be cut from a stale manifest.

## 4. Gate lane (consumer)

`vendkit gate [--strict] [--manifest <path> | --all]`

- `--all` (the scaffolded default) discovers every `.vendkit/*-manifest.json`.
- For each manifest entry: recompute the normalised (or raw) hash of
  `consumer_path` and compare `sha256` and `exec`. Findings:
  - `changed` — hash or exec bit differs (hand-edit, chmod);
  - `removed` — file missing.
  There is deliberately **no `added` finding**: consumers own every path outside
  the tracked slice.
- **Seed entries are exempt from hash/exec/removed checks** — a seeded file is
  consumer-owned and free to diverge or disappear (DR-0013). They still claim
  their `consumer_path` for the collision check below, and are exempt from the
  `paths-lockstep` conformance coverage requirement.
- **INV-7 enforcement:** with `--all`, fail if any `consumer_path` appears in
  more than one manifest (`collision` finding), regardless of hashes.
- `--strict` exits 1 on any finding; without it, findings are reported, exit 0
  (advisory mode for staged adoption).
- **Dependency-free (INV-9):** the gate reads only JSON manifests and files —
  no YAML, no third-party packages, no network, no export declaration. It must
  run as a bare static binary — no interpreter, no runtime prerequisites (DR-0016).

### Gate wiring per platform

The gate must run on every PR that *could* touch a vendored path, and the
platform's enforcement mechanism must make it required:

- **GitHub:** reusable workflow / composite action on `pull_request`, marked as
  a required status check via branch protection or a ruleset.
- **ADO:** step template run by a pipeline attached as a **Build Validation
  branch policy** (Azure Repos does not act on YAML `pr:` triggers).

Path-filtering the gate is a permitted optimisation, but then the filter must
cover every `consumer_path` (the conformance `paths-lockstep` rule checks this;
see conformance spec). The scaffolded default is **no path filter** — run the
gate on every PR; it is cheap (hashing a few hundred files) and the lockstep
trap disappears.

## 5. Provenance markers (optional, adapter-provided)

For reviewable text formats a publisher may enable an in-band provenance marker
(e.g. front-matter `vendored: <slice>`) via a future adapter. The marker is
versionless — releases must not churn every vendored file — and is a courtesy
for humans; the manifest, not the marker, is authoritative.

## 6. Conformance kit obligations (see testing.md)

- Round-trip: generate → materialise → gate = clean (INV-1).
- CRLF re-checkout does not trip the gate; a one-character edit does.
- Exec-bit flip trips the gate.
- Binary file (`raw: true`) round-trips byte-identically.
- Two manifests claiming one path trip `collision` (INV-7).
