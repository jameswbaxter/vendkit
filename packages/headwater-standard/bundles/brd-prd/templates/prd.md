---
id: "{{ACME-PRD-self-serve-onboarding, minted once and never reissued}}"
status: draft
status_since: "{{today}}"
last_verified: "{{today}}"
summary: "{{one sentence a reader scans, which is the scent and not the title}}"
requirement_tier: prd
provenance:
  warrant: accepted
  agency: "{{human | agent | mixed}}"
  accepted_by: "{{a human, always named}}"
  evidence_basis: "{{evidenced | reconstructed | unevidenced}}"
relations:
  elaborates:
    - "{{the identifier of the business requirements document this one answers, or delete this block. The business document names this one back under `elaborated_by`, and the reciprocity rule reports a half-written pair}}"
---

# {{the product change, named for what a user gets}}

## Problem

{{The problem this product change solves, stated for the team that builds it. Name the user, the task they are trying to finish, and the point at which the product stops them. A problem statement that names a feature has skipped the problem.}}

{{Name what is out of scope, where a reader would reasonably assume otherwise. An omission reads as an oversight and a stated exclusion reads as a decision.}}

## Solution requirements

{{What the product must do, numbered, one requirement per line. This is the canonical text for the build: when a ticket and this document disagree, this one is right. Write each line so that an engineer can build it and a tester can fail it.}}

{{Cite the business requirement each line answers, by the number the business requirements document gives it. Nothing checks the citation. Identity is per document, so no edge reaches a requirement inside another document, and the doctrine of this entry carries the finding. A requirement that answers no business need is scope, and `OB-BP-2` records that nothing here holds the stated reason for it.}}

## Release criteria

{{What must be true before this ships. Name the test, the measurement or the review for each one. Write the criteria as a reader checks them at the moment of release, and never as a plan that sequences the work into phases.}}

{{This heading is what done means. A product requirements document with no release criteria hands the decision to whoever is in the room on the day.}}
