# DR-0004 — Normalised content hashing

- **Status:** accepted
- **Date:** 2026-07-08

## Context

The gate compares vendored files to manifest checksums. Raw byte hashes make
line-ending and trailing-whitespace differences — which git checkout settings
(`core.autocrlf`, `.gitattributes`) produce legitimately and platform-dependently
— indistinguishable from real edits, drowning the gate in false positives that
train consumers to ignore it.

## Decision

Hashes are computed over a canonicalised stream: strict UTF-8 decode, CRLF/CR →
LF, strip trailing whitespace per line, exactly one final newline, SHA-256.
Files that fail UTF-8 decoding are hashed raw and flagged `raw: true` in the
manifest at generate time (recorded, never re-guessed). The recipe identifier
is stored in the manifest; a new recipe is a schema bump.

## Alternatives considered

- **Raw byte hashing.** False positives as above; alternatively forcing
  consumer checkout settings is an unacceptable imposition on unrelated repo
  policy.
- **Normalising the bytes on disk at materialise time.** Mutates content the
  consumer sees vs what the publisher shipped, and still breaks on re-checkout.
  Hash-time normalisation leaves bytes alone.
- **git object hashing (blob SHA).** Ties the recipe to git's and to filter
  configuration; opaque to a stdlib-only implementation.

## Consequences

- Checkout noise can never trip the gate; any substantive edit still changes
  the hash (scenario-kit cases).
- Whitespace-only vandalism inside a line is detected; whitespace-only changes
  at line ends are deliberately invisible — an accepted trade.
- Exec-bit changes are tracked separately per entry (`exec`), since content
  hashing cannot see mode.
