---
id: "{{ACME-TS-delivery, minted once and never reissued}}"
status: draft
status_since: "{{today}}"
last_verified: "{{today}}"
summary: "{{one sentence a reader scans, which is the scent and not the title}}"
spec_layer: technical_spec
provenance:
  warrant: accepted
  agency: "{{human | agent | mixed}}"
  accepted_by: "{{a human, always named}}"
  evidence_basis: "{{evidenced | reconstructed | unevidenced}}"
relations:
  realizes:
    - "{{the identifier of the functional specification this document realizes. Required, and the functional specification names this document back under `realized_by`}}"
  regulated_by:
    - "{{the identifier of each standard that binds this document directly, or delete this block}}"
---

# {{how the component is realized, as the one heading of the document}}

## Scope

{{The one functional specification this document realizes, named by its identifier, and the part of the realization this document covers. A technical specification with no functional specification above it is a design nobody asked for, and the reciprocity rule reports it.}}

## Design

{{How the component is built: the structure, the data, the protocols and the choices. Write for an engineer who has read the functional specification and now has to change the code.}}

{{Do not restate the behavior. When this document and the functional specification disagree about what the component does, the functional specification is right, and a restatement here is the drift that makes the disagreement possible. Cite the functional specification and describe the realization.}}

{{Record the choices a reader would otherwise reopen: what was considered, what was taken, and what the taken option costs. A decision that needs its own argument belongs in a decision record, and this heading cites it.}}

## Conformance

{{How a reader establishes that the code matches this document, and how the design meets each requirement of each standard that regulates the functional specification above. Name the test, the measurement or the review for each one.}}

{{Nothing in this engine reads a requirement, so no rule reports a requirement this document leaves unanswered. The obligation `OB-SS-1` states the invariant and declares that no control of this engine discharges it.}}
