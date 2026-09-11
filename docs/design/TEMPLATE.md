---
id: DR-NNNN
title: "The decision, stated as a decision and not as a topic"
status: draft
status_since: "YYYY-MM-DD"
last_verified: "YYYY-MM-DD"
summary: "One sentence a reader scans in the generated index. The scent, not the title."
---

# DR-NNNN — Title

The front matter above is what Headwater types this record against (DR-0021):
`status` is one of `draft`, `current`, `superseded` or `deprecated`, and the
two dates are ISO. Run `headwater check --fix` once the file exists — it writes
the identifier claim under `.headwater/ids/` that reserves `DR-NNNN` against
every other branch. Do not edit `README.md`: `headwater generate` writes it.

## Context

What forces are in play; what problem must be decided. Written so a reader with
no project history can follow it.

## Decision

The decision, in one or two declarative paragraphs.

## Alternatives considered

Each rejected alternative with the reason it lost. This section is the DR's
main value — keep it honest.

## Consequences

What becomes easier, what becomes harder, what obligations this creates
(invariants, tests, docs).
