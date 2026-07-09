# Design records

Durable rationale for load-bearing decisions. A DR records *why*, so future
contributors change decisions knowingly rather than accidentally. DRs are
immutable once accepted; a reversal is a new DR that supersedes the old one.

Format: see [TEMPLATE.md](TEMPLATE.md). IDs are `DR-NNNN`, allocated
sequentially, never reused.

| ID | Title | Status |
|---|---|---|
| [DR-0001](DR-0001-vendored-identity-copies.md) | Vendored identity copies, not package distribution | accepted |
| [DR-0002](DR-0002-identity-in-declaration.md) | Slice identity lives in the declaration, not the tools | accepted |
| [DR-0003](DR-0003-two-lane-model.md) | Two-lane distribution: sync PRs + PR-time gate | accepted |
| [DR-0004](DR-0004-normalised-hashing.md) | Normalised content hashing | accepted |
| [DR-0005](DR-0005-tag-is-the-release.md) | The tag is the release: immutable, SHA-anchored, retractable | accepted |
| [DR-0006](DR-0006-pull-with-push-hints.md) | Pull reconciliation with optional push hints | accepted |
| [DR-0007](DR-0007-platform-ports.md) | ADO and GitHub Actions as peer backends behind a port interface | accepted; service ops superseded by DR-0014/15 |
| [DR-0008](DR-0008-declarative-migrations.md) | Declarative migrations with deterministic verification | accepted |
| [DR-0009](DR-0009-content-adapters.md) | Content adapters: identity copy by default, named transforms by declaration | accepted |
| [DR-0010](DR-0010-review-gated-scope.md) | Scope changes are reviewed PR events, never automatic | accepted |
| [DR-0011](DR-0011-single-cli-stdlib-gate.md) | One CLI; dependency-free consumer gate path | accepted |
| [DR-0012](DR-0012-consumer-config-consolidation.md) | One consumer config file per slice under `.vendkit/` | accepted |
| [DR-0013](DR-0013-seeded-files.md) | Seeded files: a scaffold-once lifecycle class | accepted |
| [DR-0014](DR-0014-handler-protocol.md) | Detection and delivery split by an exec handler protocol | accepted |
| [DR-0015](DR-0015-scm-ci-axes.md) | SCM and CI axes; formats not services | accepted |
