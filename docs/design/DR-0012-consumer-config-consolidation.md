# DR-0012 — One consumer config file per slice under `.vendkit/`

- **Status:** accepted
- **Date:** 2026-07-08

## Context

Consumer-side state could scatter across many small files (a watch config, a
compliance config, a profile file, per-pipeline values), each with its own
discovery convention and its own way of drifting out of lockstep with the
others — e.g. a slice added to sync but forgotten in watch. Equally it could
over-consolidate into one repo-global file that every slice's sync PR then
contends over.

## Decision

Per slice, exactly two files in `.vendkit/`: the **slice config**
(`<slice>.yml` — consumer-owned: publisher coordinates, profile binding, pin
location, watch channel + handoff routing, attestations, waivers) and the
**slice manifest** (`<slice>-manifest.json` — engine-owned). Discovery is
fixed: tools enumerate `.vendkit/*.yml` and `.vendkit/*-manifest.json`,
nothing else (INV-8). Watch, gate, and conformance are slice-agnostic loops
over that enumeration, so **adding a slice cannot forget a wiring** — there is
no second registry to update. Pipeline files remain platform-native (they are
CI code, not framework state) and pins stay in them as the single
source of truth, *located by* the slice config.

## Alternatives considered

- **Separate files per concern (watch config, conformance config, profile
  file).** Every new slice touches three registries; lockstep between them
  becomes a new conformance-rule class to police a self-inflicted problem.
- **One repo-global `vendkit.yml`.** Multi-slice consumers get merge contention
  in every sync PR; slice deletion becomes an edit instead of a file removal;
  per-slice CODEOWNERS is coarser.
- **Pin duplicated into the slice config.** Two sources of truth for the most
  load-bearing value in the system; the pipeline reference must exist anyway
  (the platform needs it), so the config records *where it is*, not *what it
  is*.

## Consequences

- `.vendkit/**` must be CODEOWNERS-covered (core conformance rule) — it is the
  control plane.
- The engine treats an unparseable slice config as a hard error for every
  command (a half-configured slice must be loud).
- Migrating a consumer between platforms touches pipelines + the `pin.file`
  values only; the rest of `.vendkit/` is portable as-is.
