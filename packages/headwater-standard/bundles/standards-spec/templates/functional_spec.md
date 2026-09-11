---
id: "{{ACME-FS-delivery, minted once and never reissued}}"
status: draft
status_since: "{{today}}"
last_verified: "{{today}}"
summary: "{{one sentence a reader scans, which is the scent and not the title}}"
spec_layer: functional_spec
provenance:
  warrant: accepted
  agency: "{{human | agent | mixed}}"
  accepted_by: "{{a human, always named}}"
  evidence_basis: "{{evidenced | reconstructed | unevidenced}}"
relations:
  regulated_by:
    - "{{the identifier of each standard that binds this component, or delete this block. The standard names this document back under `regulates`}}"
  realized_by:
    - "{{the identifier of the technical specification that realizes this one. Absent until one is written, and the expectation reports its absence after 90 days}}"
---

# {{the component, and what it does, as the one heading of the document}}

## Scope

{{The one component this document is about, and the boundary of it. Name what a caller reaches and name what is somebody else's document. A functional specification whose scope is a subsystem is a subsystem's worth of documents that nobody split.}}

## Behavior

{{What the component does, stated as it is now and never as what it will do. This is the canonical text: when the technical specification and this document disagree, this one is right and the other is stale. Write for a caller who has to decide what to rely on.}}

{{Say what the component does not do, where a reader would reasonably assume otherwise. An omission reads as an oversight and a stated exclusion reads as a decision.}}

## Acceptance

{{How a reader establishes that an implementation matches this document. Name the test, the measurement or the review. This heading is what separates a specification from a description, and it is where the standards that regulate this component are answered.}}

{{Cite each requirement of each regulating standard that this component answers, by the number the standard gives it. Nothing checks the citation. Identity is per document, so no edge reaches a requirement inside a standard, and the doctrine of this entry carries the finding.}}
