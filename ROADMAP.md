# Roadmap

**Status:** 1.0 shipped. The build-out milestones (M0–M4) are complete and the
surface is frozen — schema_version v1, the CLI command set, the declaration
schema, and the `key=value` output contract are now public API under the
[compatibility policy](COMPATIBILITY.md). What follows is only the work that
remains *ahead* of 1.0; the milestone history lives in the git log and the
design records (`docs/design/`).

## Outstanding

Nothing is in flight. The items below are deliberately deferred — recorded here
so they are chosen intentionally, not re-litigated casually. Each is now a
post-1.0 change and, where it touches a frozen surface, a MAJOR event with a
migration entry.

- **Central push distributor** — a hub that fans release hints out to
  subscribers, rather than the current publisher-side dispatch. DR-0006 keeps
  the door open; no current need.
- **Additional adapter kinds** — beyond the shipped set. DR-0009: a new kind is
  a MAJOR event, so it waits for a real consumer requirement.
- **A third CI platform** — the template pack + reference handler + behavioural
  ledger make it tractable (DR-0007 / DR-0014). Post-1.0 by choice.
- **Signed manifests / attestations** — provenance SHA + ref protection are the
  1.0 trust basis; revisit signing if consumers begin crossing trust domains.
