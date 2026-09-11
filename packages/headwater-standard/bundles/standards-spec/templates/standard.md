---
id: "{{ACME-STD-logging, minted once and never reissued}}"
status: draft
status_since: "{{today}}"
last_verified: "{{today}}"
summary: "{{one sentence a reader scans, which is the scent and not the title}}"
provenance:
  warrant: accepted
  agency: "{{human | agent | mixed}}"
  accepted_by: "{{a human, always named}}"
  evidence_basis: "{{evidenced | reconstructed | unevidenced}}"
relations:
  regulates:
    - "{{the identifier of a functional or technical specification this standard binds. One line per specification, and the specification names this standard back under `regulated_by`}}"
---

# {{the name of the standard, as the one heading of the document}}

## Scope

{{Which components this standard binds and which it does not. A standard whose scope is everything is a standard nobody can apply, and a reader who cannot tell whether their component is in scope reads the whole document to find out. Name the class of component, and name the exclusions.}}

## Requirements

{{The requirements themselves, one per numbered subheading, each one stated so that a reader can tell whether a specification meets it. Use the normative keywords of your house style and use them consistently.}}

{{Nothing in this engine reads a requirement under this heading. Identity is per document, so `ACME-STD-logging` names the standard and no declaration names one requirement inside it. Number your requirements anyway. The numbers are what a specification cites, and the doctrine of this entry carries the finding.}}

### {{1. The first requirement, as a sentence a reader can test}}

{{What must hold, and what a component that does not hold it looks like.}}

## Conformance

{{How a reader establishes that a component conforms. Name the evidence: a test, a review, an audit, a measurement. A standard that states requirements and no way to establish them asks every reader to invent one.}}

{{State what a deviation costs and who registers it. Nothing in this engine records a deviation, because the relation that would carry it is package content that no package supplies, so the register you name here is a register a person keeps.}}
