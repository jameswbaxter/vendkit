# Compatibility policy

Status: in force from v1.0.0 · Owner: Layer 0

VendKit is 1.0. This document is the single authoritative statement of what is
frozen, what "a breaking change" means, and how one is allowed to ship. It is
the "compatibility policy in force" that [CONTRIBUTING.md](CONTRIBUTING.md) and
[SECURITY.md](SECURITY.md) refer to.

## 1. Versioning

Releases are annotated Git tags `vMAJOR.MINOR.PATCH` (the tag *is* the release —
[docs/specs/releases-and-versioning.md](docs/specs/releases-and-versioning.md)).
SemVer is interpreted for a *content* framework, and the meaning is declared,
not inferred:

| Bump | Meaning |
|---|---|
| PATCH | Refresh-only: no frozen-surface change, no migrations |
| MINOR | Additive only: new commands/flags/fields/rules; additive migrations at most; nothing existing removed or reshaped |
| MAJOR | A breaking change to any frozen surface below — always paired with a migration entry |

`vendkit release` computes the export-surface delta against the previous
release and refuses a bump smaller than the delta implies, so the table is a
gate, not a convention.

## 2. What is frozen at 1.0

The following are public API. A change that removes, renames, or reshapes any of
them — or alters its documented semantics — is a **breaking change**:

- **`schema_version` = 1.** The field set and semantics of every v1 config
  document — export declaration, slice config, manifest, migrations,
  subscribers, conformance rules. The engine accepts `schema_version: 1` and
  refuses newer values (`internal/core`); bumping the schema is a MAJOR event
  that ships a migration.
- **The CLI surface.** The top-level command set and each command's documented
  flags in [docs/specs/cli.md](docs/specs/cli.md). The command set is
  test-enforced against the binary — see `cmd/vendkit/surface_test.go`
  (`frozenSurface`), which fails CI if a command is added or removed without a
  deliberate edit.
- **The `key=value` output contract.** Machine-tier fact names and refusal
  tokens (`refused=…`) and the exit-code conventions (0/1/2/3/≥4). The
  human-tier verbs (`status`, `diff`, `update`, `explain`) are explicitly *not*
  covered by the formatting promise — scripts must not parse them (cli.md).
- **The declaration schema** as a whole (DR-0002): the export declaration is a
  public API with the same obligations as the code surface.
- **The handler protocol** — the intent-in / facts-out contract that reference
  and third-party handlers implement
  ([docs/specs/handler-protocol.md](docs/specs/handler-protocol.md)).

## 3. How a breaking change ships

A breaking change to anything in §2 requires, together:

1. a **MAJOR** version bump, and
2. a **migration entry** (`migrations/` payload with `applies_from` = the new
   version) so consumers have a mechanical or documented remediation path
   ([docs/specs/migrations.md](docs/specs/migrations.md)). The release command's
   migration pre-gate enforces this on the removal/reshape path; where a MAJOR
   genuinely needs no consumer action, that is recorded explicitly via
   `--no-migrations-needed` in the tag annotation.

There is no back-door: deprecations still land as a MAJOR removal with a
migration, never as a silent drop within a MINOR/PATCH.

## 4. Supported versions

Security fixes and bug fixes land on the **latest** release line. There is no
back-porting to superseded `v0.x` tags or to older `v1.x` lines; upgrade to the
newest release to receive fixes. Retraction — not deletion — is the mechanism
for a release found harmful (releases-and-versioning §4): a pinned consumer must
always keep resolving its pin (INV-5).
