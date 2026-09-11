---
status: current
status_since: "{{today}}"
last_verified: "{{today}}"
summary: "{{what this instrument asks a reviewer to do}}"
doc_type: review_prompt
provenance:
  warrant: accepted
  agency: "{{human | agent | mixed}}"
  accepted_by: "{{a human, always named}}"
  evidence_basis: evidenced
relations:
  applied_in:
    - "{{the identifier of the review record this instrument produced, once it exists}}"
---

# {{title}}

## What the reviewer reads

{{The documents under review, named exactly. A review whose scope is "the
specification" cannot be repeated against a later draft.}}

## What the reviewer is asked

{{The questions, in the order they should be answered. This is the instrument,
and it is committed before the review runs so that the findings can be read
against what was asked rather than against what a reader assumes was asked.}}

## What counts as a finding

{{The bar. Without it, two runs of this instrument are not comparable, which
is the whole reason the prompt is a document.}}

## Out of scope

{{What this instrument deliberately does not ask about.}}
