# Contributing

This project is in its specification phase — contributions right now are to the
design, not code. The rules below are chosen to still be right once code
exists.

## Ground rules

- **Specs are normative.** Behaviour lives in `docs/specs/`; rationale lives in
  `docs/design/` (DRs). A PR that changes behaviour updates the spec in the
  same PR; a PR that changes a *decision* adds a superseding DR — accepted DRs
  are immutable.
- **Terminology:** [GLOSSARY.md](GLOSSARY.md) wins. Fix specs that diverge.
- **Invariants (architecture §3) are load-bearing.** A change that weakens an
  INV-n needs a DR, not just a diff.
- **Platform parity:** anything platform-visible ships for ADO and GHA
  together, with the differences ledger updated (DR-0007).
- **Gate-path purity:** code on the consumer PR path imports stdlib only
  (DR-0011). CI will enforce this.
- No references to specific companies, products, or internal systems — examples
  use `example-org/*` placeholders.

## Practicalities

- Branch from `main`; PRs small and single-topic; maintainers squash-merge.
- Conventional commit prefixes (`spec:`, `dr:`, `feat:`, `fix:`, `test:`,
  `docs:`).
- Every behavioural PR adds/extends a scenario-kit case (testing.md §2);
  breaking changes follow the [compatibility policy](COMPATIBILITY.md)
  (MAJOR + migration entry — the surface frozen at 1.0 is enumerated there).

## Security

Report suspected vulnerabilities privately — [SECURITY.md](SECURITY.md)
defines the channel. The threat model is
[docs/specs/security-model.md](docs/specs/security-model.md) — read it before
proposing anything that touches credentials, tags, or the PR machinery.
