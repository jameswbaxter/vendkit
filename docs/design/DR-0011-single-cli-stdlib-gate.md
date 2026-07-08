# DR-0011 — One CLI; dependency-free consumer gate path

- **Status:** accepted
- **Date:** 2026-07-08

## Context

The machinery spans a dozen operations across publisher and consumer, wired
into two CI platforms. Shipping each as a separate script multiplies entry
points, argument conventions, and packaging surfaces; and the consumer-side
integrity check runs on every PR of every consumer, where any dependency
install is latency, flake surface, and supply-chain exposure.

## Decision

One entrypoint — `vendkit` — with subcommands sharing global flags, exit-code
semantics, and output conventions (`key=value` + `--json`; ports mirror to
platform outputs). The PR-blocking consumer path (`gate`, `migrations-verify`,
`is-newer`) is **standard-library only** (INV-9): no YAML (manifests are JSON),
no HTTP, no installs. Publisher/sync paths may lazily import one pinned YAML
library. Layer 2 wrappers call the CLI; they never reimplement logic.

## Alternatives considered

- **Script-per-operation.** N packaging surfaces and N argument dialects;
  conventions enforced by discipline instead of shared code.
- **Full dependency freedom (pydantic, requests, click…).** Nicer authoring,
  but the gate would need package installs on every consumer PR — the
  framework guarding supply chains should not itself import one.
- **Compiled single binary (Go/Rust).** Solves distribution, but the engine is
  itself vendored source (DR-0001) — consumers review and run it in-tree;
  interpreted, stdlib-bounded Python keeps that reviewable and portable
  without build infrastructure.

## Consequences

- Manifest format is locked to JSON (a YAML manifest would break INV-9).
- CLI flags and `key=value` names are public API (deprecation = MAJOR +
  migration entry).
- Contributors face a real constraint: gate-path code reviews reject
  third-party imports; CI enforces with an import-linter check.
