<!-- headwater:generated shelf_index. `headwater generate` writes this file, and `headwater generate --check` holds it. Edit the corpus, not this file. -->

# Component specifications

12 documents on this shelf, in the reading order this corpus derives.

- [The vendkit command surface](cli.md) — One static binary whose command set, flags and key=value facts are frozen public API, split into a machine tier and a human tier composed from it.
- [Conformance rules and the fleet view](conformance.md) — A rule set shipped inside each release answers whether a consumer is correctly wired, degrading to attestation where a platform fact is not decidable from the tree.
- [The export declaration that defines a slice](export-declaration.md) — One publisher-side YAML file carries every piece of slice identity, the exported and seeded surfaces, the adapters, and the consumer profiles, so the tools carry none of their own.
- [The handler protocol for vendor deliveries](handler-protocol.md) — Core judgments become deliveries by composing a JSON intent and handing it to a configured executable, so any protocol-honouring handler replaces the reference ones without an engine change.
- [Manifest schema and the gate lane](manifest-and-gate.md) — The manifest is the integrity contract between publisher and consumer, and the gate lane re-hashes vendored files on every consumer PR so a hand-edit or deletion cannot merge.
- [Declarative migrations for consumer-owned content](migrations.md) — A release that invalidates consumer-owned content ships a declarative payload stating what must become true, which a deterministic verifier gates and which contains no executable code.
- [Consumer configuration and the scaffolder](onboarding.md) — One config file per slice holds every consumer-local value, and the scaffolder vendors the slice, renders the CI-keyed pipelines, and reports the manual steps it refuses to perform itself.
- [CI surfaces, credentials and the differences ledger](platform-integration.md) — Everything platform-flavoured that is not the handler protocol: the CI output dialect, who resolves which credential, how push hints reach a consumer, and where cross-platform differences are recorded.
- [Releases, version grammar and retraction](releases-and-versioning.md) — An annotated SemVer tag is the entire release, its bump class is computed from the export-surface delta rather than asserted, and a harmful release is retracted rather than deleted.
- [Release detection and handoff](release-watch.md) — A scheduled consumer job compares each slice's pin against the publisher's latest qualifying release over the git protocol, producing findings that a handoff handler turns into work.
- [The trust boundary and what guards it](security-model.md) — Write access to a publisher plus release capability is code execution in every consumer, and the framework makes that boundary explicit, narrow and tamper-evident rather than pretending otherwise.
- [The sync lane and materialisation](sync.md) — Materialise rewrites the tracked files and manifest from a target release and opens one reviewed PR, never merging, never deleting from disk, and never widening scope without review.
