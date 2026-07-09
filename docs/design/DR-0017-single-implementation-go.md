# DR-0017: Single engine implementation in Go, with the scenario kit as the parity ratchet

Status: accepted

## Context

The engine's job profile is hashing, globbing, YAML/JSON, git subprocesses,
and (in handlers) small REST calls — nothing that needs the Python
ecosystem. Meanwhile the consumer-side promises keep bumping against the
interpreter: INV-9's "stdlib-only" discipline exists solely because
consumers run the gate on bare CI Python; wrapper steps pay `setup-python`
on every run; Windows agents are awkward. A dependency-free static binary
makes the strongest promise ("runs anywhere, needs nothing") a property of
the artefact instead of a discipline. DR-0014/DR-0015 already shrank what a
port must reproduce: the core is git + filesystem + subprocess, with vendor
services in separately-shipped handler executables — which the
language-agnostic exec protocol lets migrate independently.

## Decision

1. **Go becomes the single engine implementation**; the Python engine is
   retired once parity is proven. No permanent dual maintenance.
2. **The scenario kit is the parity ratchet.** It drives the CLI as a
   subprocess, so it is implementation-blind: `VENDKIT_CLI=<binary>` runs
   the identical matrix against the Go engine. Porting is complete when the
   full matrix passes both ways. (Two in-process Python tests — the
   `_pr_body` unit and the INV-9 no-YAML import test — stay with the Python
   engine until retirement, then translate.)
3. **Golden fidelity vectors** (`tests/vectors/`) pin the behaviours where
   ports silently diverge, generated from the reference implementation and
   consumed by both test suites:
   - *normalisation*: byte inputs → digest + raw flag (strict UTF-8
     validation, CRLF/CR folding, trailing-ws/newline rules, BOM kept as
     content);
   - *two glob dialects*: fnmatch semantics for exclude/adapters/profiles/
     obligations (`*` and `?` cross `/`, dotfiles match, case-sensitive)
     and pathlib-glob semantics for include/seed surfaces (`*` does NOT
     cross `/`, `**` spans directories including zero); off-the-shelf Go
     glob libraries match NEITHER exactly — implement to the vectors;
   - *canonical manifest JSON*: sorted keys, 2-space indent, ensure-ASCII
     escaping, no HTML escaping, trailing newline — `generate --check`
     compares bytes, so this must be exact (Go's `encoding/json`
     HTML-escapes and randomises maps; a custom marshaller is required).
4. **Dependency budget**: Go stdlib + one YAML library (`gopkg.in/yaml.v3`,
   compiled in, invisible to consumers). YAML 1.1 quirks (`yes/no`
   booleans, octals) are dodged by schema validation, matching the Python
   engine's strictness.
5. **CLI surface, flags, exit codes, `key=value` outputs, and refusal
   tokens are contractually identical** — the conformance detectors'
   invocation regexes (`vendkit gate`, `vendkit sync-pipeline`…) match the
   binary unchanged.

## Alternatives considered

- **Stay on Python + zipapp/shiv** — one file but still needs an
  interpreter; solves nothing on bare/Windows runners. Rejected.
- **PyInstaller/Nuitka binaries** — cross-platform builds are fragile and
  large; the supply-chain story (bundled interpreter) is worse than a Go
  static binary. Rejected.
- **Rust** — same properties as Go at higher implementation cost for this
  job profile. Rejected.
- **Permanent dual implementation (Python reference + Go product)** —
  doubles every invariant's enforcement surface for no consumer benefit.
  Rejected; the vectors + kit make the single-implementation cutover safe
  instead.

## Consequences

- INV-6 must be restated for a compiled engine — split into DR-0016.
- The repo hosts `go/` alongside `vendkit/` during transition; the export
  declaration ships the Go sources (and later, releases attach binaries per
  DR-0016). Python retirement is a MAJOR release with a migration entry.
- Handlers may remain Python (or anything) indefinitely: the exec protocol
  is the boundary, though shipping Go reference handlers alongside the
  engine keeps the zero-prerequisite story whole.
