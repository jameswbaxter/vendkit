---
status: current
status_since: "{{today}}"
last_verified: "{{today}}"
summary: "{{what this run of the instrument found}}"
doc_type: review_record
provenance:
  warrant: accepted
  agency: "{{human | agent | mixed}}"
  accepted_by: "{{a human, always named}}"
  evidence_basis: evidenced
relations:
  applies: "{{the identifier of the review prompt this run applied}}"
  assesses:
    - "{{the identifier of each document that this review read}}"
---

# {{title}}

## The run

{{When it ran, who or what ran it, and against which revision of the documents
named above. A finding against a draft that later changed is still a fact about
that draft, and the revision is what keeps it one.}}

## Findings

### {{F1}} — {{the finding, in one line}}

**What.** {{The defect, stated so that somebody who did not run the review can
check it.}}

**Where.** {{The document and the section.}}

**Why it matters.** {{What breaks, or what a reader gets wrong.}}

**Disposition.** {{Accepted, refused with a reason, or deferred to an entry in
the obligation register.}}

## What the instrument missed

{{What this run could not see. It is the input to the next version of the
prompt, and the reason both documents are kept.}}
