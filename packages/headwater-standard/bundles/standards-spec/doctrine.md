# The standards-spec taxonomy

The internal-standard ladder. A standard binds many components. A functional specification states what one component does, and a technical specification states how that component is realized.

This entry declares three kinds, one purpose, one facet, two relations, two shelves, three identifier schemes and three obligations. It requires no other entry. This file states what the tradition is, why each declaration is here, and what this draft assumed where the specification is silent.

## The prior art

**MIL-STD-498**, issued by the United States Department of Defense in December 1994. It unified two lines of practice that had run apart. One was the development standard for weapon-system software. The other specified the content of the life-cycle documents of an automated information system. Its Data Item Descriptions name the rungs directly. The System/Subsystem Specification, the Software Requirements Specification and the Software Design Description are three of them. The standard was later withdrawn, and the ladder it wrote down outlived it.

**ISO/IEC/IEEE 29148**, second edition 2018. The family replaced IEEE 830-1998, which is the requirements-specification document most engineers of that period met. It names four information items rather than one document. They are a business requirements specification, a stakeholder requirements specification, a system requirements specification and a software requirements specification.

What runs between the four is traceability, and it is not governance. The standard defines requirements traceability as the "derivation path (upward) and allocation/flow-down path (downward) of requirements in the requirements set". It names the two ends parent and child. Nothing in it says that one item governs the reading of another. It also declines to fix the four as four documents. Clause 7 says the items "can contain similar information items that could be considered as different views for the same product". Clause 8.2 says a reader may combine the business item with the stakeholder one.

**Joel Spolsky, "Painless Functional Specifications", four parts on Joel on Software, October 2000.** This is the source of the two names this entry uses. A functional specification "describes how a product will work entirely from the user's perspective", and it says nothing about how the product is built. A technical specification "describes the internal implementation of the program". It talks about data structures, database models, algorithms and the choice of language.

His argument for the split is sequencing rather than audience. "The most important thing is to nail down the user experience." The choice of programming language is not worth arguing about until the product's behavior is settled. The functional document comes first because the cheapest decisions are the ones taken before any code exists.

**This repository already wrote the same ladder down.** [Spec 2](https://github.com/headwater-ai/headwater/blob/main/docs/spec/02-taxonomy-model.md#contract-sidecars-the-specification-as-oracle) sketches a contract sidecar over a layout of `specifications/ingest/parser/functional.md` and `technical.md`. It glosses the first as "prose: what it does and why — canonical for meaning". It glosses the second as "prose: how it is realized". That is in-corpus prior art, and it arrived from the sidecar question rather than from any of the three sources above. Four independent arrivals at one shape is the strongest argument this entry has for criterion 1.

## What the tradition converges on

**A document about one component, at two levels.** This is the four-way convergence, and it is the strongest claim in this section. MIL-STD-498 separates the Software Requirements Specification from the Software Design Description. Spolsky separates the functional specification from the technical one. Spec 2 separates `functional.md` from `technical.md`. 29148 puts requirements in its four items and leaves the design description to ISO/IEC/IEEE 12207 beside it.

**A third level above both, which two of the four supply rather than describe.** MIL-STD-498 and 29148 are themselves standards that bind many specifications across many projects. That binding is the relation this entry declares. Neither one describes an in-corpus kind for it, and Spolsky never discusses standards at all. So the `standard` kind rests on two sources and not on four. The entry says so rather than claim a convergence it does not have.

**The functional level is canonical for meaning.** Spolsky puts the user experience first and the implementation second. Spec 2 glosses `functional.md` as "canonical for meaning" in as many words. In MIL-STD-498 and in 29148 the requirements document is the baseline a design answers to. The reader who asks what may be relied on is sent to the functional document in all four.

**The rate-of-change argument is this entry's own, and no source states it.** A realization changes when the implementation changes. Behavior changes when somebody decides to change it. To hold both in one document makes every implementation change look like a behavior change to every reader of the history. That reasoning is why `realizes` is a `derivation` edge with the functional specification as its nucleus. It is offered here as an argument rather than as a citation.

**Traceability runs both ways, and 29148 is the source that states it exactly.** Its definition names a derivation path upward and an allocation path downward, over one requirements set. MIL-STD-498 asks the same of its Data Item Descriptions. Neither direction is a link that a corpus derives from prose. That is why this entry declares edges for it, and why `OB-SS-2` records what the edges still cannot reach.

**What the tradition does not converge on is the file layout, or the number of rungs.** MIL-STD-498 numbers its deliverables. 29148 names four items by scope and then permits two of them to be one document, and Spolsky writes one document per feature. This entry takes the layout spec 2 already sketched, because it is the one the corpus that hosts this library wrote.

## Why three kinds and not two or four

**Not two.** A taxonomy that merges the standard into the specification loses the one edge that matters. A standard binds many components and a specification is about one. To merge them makes `regulates` a self-edge on one kind. The reading precedence of the governance family then says a document governs the reading of itself.

**Not four.** 29148 names four information items and this entry declares two specification kinds. The standard itself is the argument against copying its four. Clause 7 calls them possible "different views for the same product", and 8.2 lets a reader combine two of them. A taxonomy with four kinds would therefore refuse a document the standard permits. A kind set that one standards body's item list fixes is also a structure taken from one source rather than from the convergence. An adopter who needs the stakeholder item and the system item apart adds two values and two kinds in an overlay of their own.

**The names are the plain words.** `standard`, `functional_spec` and `technical_spec`. `governing_standard` was considered and refused. Spec 2's rigidity rule refuses a kind named with a bare phase adjective and says nothing against a plain noun. The hedge would be this entry disagreeing with the tradition it models. `srs`, `sdd` and `requirements_spec` were refused for naming one standards body.

**Two kinds serve one purpose, and that is legal.** [Spec 2](https://github.com/headwater-ai/headwater/blob/main/docs/spec/02-taxonomy-model.md#purpose-is-declared-not-implied) says several kinds may serve one purpose, and it refuses only a kind that serves two unrelated purposes. The precedent inside this library is exact. The design-spec entry gives `review_prompt` and `review_record` the one `evidence` purpose, and it splits them because a relation runs between them. Here `realizes` runs between the two specification kinds, and a relation endpoint names a kind.

## The relations, and who creates each edge

**Neither relation is `created_by: author`**, so this entry owes no author-edge sentence under the rule the [library index](../README.md#one-rule-of-the-base-is-not-a-criterion-here) states. Both proposers are named below anyway, because a reader who has to write an edge by hand should know that nobody meant them to.

### `regulates`, from the standard to the specification

The direction is forced rather than chosen, and it is the sharpest point of this entry. [Spec 2](https://github.com/headwater-ai/headwater/blob/main/docs/spec/02-taxonomy-model.md#reading-precedence-is-derived) derives reading precedence for the governance family in one clause: "On governance between two documents, the source governs." An edge spelled `conforms_to`, from the specification to the standard, would therefore derive the precedence backwards. The specification would govern the reading of the standard that binds it, and an agent that pruned by precedence would read the wrong document first.

The name avoids two addresses the base holds. `governs` runs from a document to a `code_path`, and `constrains` runs from a decision to a decision. Both declare their endpoints as lists, and an `add` cannot reach into a list that exists, so neither is reachable.

`created_by: agent`, which matches the base's `constrains`. Nothing mechanical knows which standard binds which component. A coherence sweep proposes the edge and a person accepts it.

### `realizes`, from the technical specification to the functional one

The family is `derivation` and the nucleus is the functional specification. This is where the entry earns its keep, and [spec 2](https://github.com/headwater-ai/headwater/blob/main/docs/spec/02-taxonomy-model.md#relation-families-and-nuclearity) lists four things nuclearity buys. Three of them land here exactly.

- **Lifecycle inheritance.** A technical specification whose functional specification is superseded is stale the moment the succession lands, and the engine sees it structurally with no separate check.
- **Context pruning.** An agent under a context budget drops the technical specification and keeps the functional one. That is the right way round, because the functional document is the one every source treats as canonical for meaning.
- **Orphan detection.** An unlinked functional specification is a real orphan, and an unlinked technical specification is a generation defect. The fixture corpus reports the first and stays silent about the second, which is the measured form of this claim.

`composition` was refused. It would say the technical specification is part of the functional one, and it is not. It is a second document about one subject at a second level.

`created_by: scaffold`, on the argument the design-spec entry used for `applied_in`. The run that opens a technical specification from a functional one writes both halves of the edge.

### The third relation this entry does not declare

The tradition's central negative edge is the registered deviation, and this entry cannot supply it. [Finding 1](#findings) states why.

## The core, declared

This is a full entry rather than a partial one. It declares kinds, shelves, a purpose, relations and identifier schemes, so the partial-entry paragraph of criterion 5 does not apply to it. It satisfies the core the way the design-spec entry does, which is **through the base** for every clause except one.

| Core requirement | What satisfies it | Declared by |
|---|---|---|
| `facet_role: state` | `facets.status` | the base, inherited through `governed_document` |
| `facet_role: freshness` | `facets.last_verified` | the base, the same way |
| `facet_role: scent` | `facets.summary` | the base, the same way |
| `purpose: rationale` | `kinds.decision` | **the base.** This entry declares no rationale-serving kind |
| `purpose: behavior` | `kinds.functional_spec` and `kinds.technical_spec` | **this entry** |
| `relation_family: succession`, lifecycle-sensitive | `relations.supersedes` | the base, untouched |

`facets.status_since` is not a clause of the core, and both participation expectations read it, so it is listed here for a reader who checks. The base requires it on `governed_document`, which is what makes the expectations well-formed.

**This entry declares no facet that carries an engine role.** `spec_layer` takes no `role`, for the reason the diataxis entry's `reader_mode` takes none. The role registry is closed at six and none of them is this.

**The purpose map, stated rather than implied.**

| Kind | Purpose | The reader intent |
|---|---|---|
| `standard` | `constraint`, new and declared by this entry | what must hold across everything this governs, and what it rules out |
| `functional_spec` | `behavior`, from the base | what does this component do, and what may I rely on |
| `technical_spec` | `behavior`, from the base | what does this component do, at the level of the realization |

`purposes.constraint` is the one new purpose. [Spec 2](https://github.com/headwater-ai/headwater/blob/main/docs/spec/02-taxonomy-model.md) already uses the word for it, in the product-suite column of its worked example. No entry and no package declares the address.

**One correction, because the issue that asked for this entry stated the core wrongly.** The framing said the immutable core requires `rationale`, `constraint`, `procedure` and `behavior`. It does not. `core.requires` in the base package names the three facet roles, the purposes `rationale` and `behavior`, and a lifecycle-sensitive succession family. `procedure` is declared only by this repository's own consumer overlay, which is not an entry. `constraint` was declared nowhere at all before this entry.

## What this entry deliberately does not declare

**A rationale-serving kind.** The base's `decision` serves `rationale` and this entry adds nothing there. A kind invented to tick criterion 5 is a shape invented for the library, which criterion 1 refuses.

**A deviation relation.** The address `does_not_comply_with` is reserved package content, and [finding 1](#findings) states the whole of it.

**A normative-keyword voice regime.** The worked example promises one and no declaration in this language holds one. [Finding 4](#findings) states it. All three kinds bind the base's `declarative` regime instead.

**A `standard_class` facet.** A facet that names a standard as internal, industry or regulatory reads well and no check, projection, routing rule or expectation would read it. The relevance canon of [spec 2](https://github.com/headwater-ai/headwater/blob/main/docs/spec/02-taxonomy-model.md#facet-acceptance-tests) refuses a facet that nothing reads, and `taxonomy validate` enforces it.

**A shelf layout.** Neither shelf declares one. [Finding 8](#findings) records the reason.

**`purposes.procedure` and `purposes.attestation`.** Both appear in spec 2's worked example and neither belongs to this tradition. An entry that models the guides column or the regulated column declares them.

## What this draft assumed

**That the discriminator value of a heterogeneous shelf is the kind name.** It is, and this is a measurement rather than a reading. `engine/crates/census/src/resolve.rs` matches the value against the shelf's `kinds` list by identity and stops when it finds no match. So `facets.spec_layer` declares `functional_spec` and `technical_spec` as its values, and a facet named for a layer carries a value named for a kind. The design-spec entry's `doc_type` has the same shape, and no rule and no document states the constraint anywhere a schema author would find it.

**That `volatility: stable` is right for `spec_layer`.** The claim is that a document does not move from one rung to the other. A document that reads as though it did has been rewritten into a second document. The design-spec entry recorded the same assumption for `doc_type` and nothing has ruled on it. The claim is what permits an adopter to put the value in a shelf path or a projection output.

**That an identifier scheme may carry no namespace.** The base declares none for `decision_id` and states why: a published package that named one would have every adopter mint under it. The decision-record entry followed the base and the design-spec entry declared no scheme at all, so two live patterns exist and no ruling separates them. This entry follows the base.

**That `minted-once` is right for all three schemes.** A standard and a specification are cited from outside the corpus, in a contract, an audit trail and a ticket. An identifier that a later reconciliation moved would break a citation that nobody inside the corpus can rewrite. The `{slug}` in each pattern is what makes this workable, because the name comes from the subject rather than from a counter.

**That the two expectation windows are 180 days and 90 days.** Neither number comes from a source. A standard that regulates nothing after half a year is a standard nobody applied. A functional specification that nothing realizes after a quarter is a requirement nobody built. Both are `severity: warn`, so both are advisory, and an adopter who measures a different cadence overrides them.

## Worked instances

Criterion 4 asks for at least one real or realistic corpus that the entry types. This entry carries two.

**[Beacon](https://github.com/headwater-ai/headwater/blob/main/docs/taxonomies/standards-spec/fixtures/README.md), a realistic corpus.** Two standards on the `standards` shelf and six component specifications on the `component_specs` shelf, with planted defects that the fixture README names one by one. Beacon is invented, and four entries of this library share it, so a reader can hold four traditions over one project.

**[n8n](https://github.com/headwater-ai/headwater/blob/main/docs/taxonomies/standards-spec/fixtures/n8n/README.md), a real one.** Seven documents from `n8n-io/n8n`, pinned at `b0550cb3cb4d1752546a69056c55eccfb9111a12` on `master`, copied with a front-matter block added and every body byte-identical: six of the rule files that n8n's AI code reviewer loads on every pull request, and the README that files them. These are documents that decide what a reviewer comments on, which is what `purposes.constraint` names. [HW-EVAL-n8n-worked-example](https://github.com/headwater-ai/headwater/blob/main/docs/evaluations/n8n-worked-example.md) is the report, and four of its results belong here.

**This entry as it ships types none of that corpus, and the strict run over it exits 0.** The shelf for `kinds.standard` is `standards` at `docs/standards/**`, and n8n keeps these documents at `.agents/review-rules/`. All seven go untyped with the reason `no shelf pattern claims this path`, no rule instantiates, and `headwater check --strict` exits 0. This is the design-spec entry's first result arriving a second time from a second kind, and it is now a property of the library rather than of one entry.

**The required sections are present in substance in the whole corpus and absent in form in the whole of it.** `kinds.standard` requires `Scope`, `Requirements` and `Conformance`. Not one heading at any level of any of the 23 upstream files carries one of those words, so a typed document reports three `section.required.missing` errors and seven of them report 21. The corpus is not missing the content. Every one of the 22 rule files opens with a line `Applies to: …`, which is the scope, written as a labeled paragraph rather than as a heading, and 16 of them use the imperative `Flag` somewhere to state what to flag, though only six write it as a literal `Flag:` block. The contract is lexical over headings and the tradition writes the same content as a labeled paragraph, so a rule that reads headings reports 100% absence over 100% presence. `Conformance` is the honest third: no rule file states how conformance is judged, because the reviewer judges it, and that section is genuinely absent rather than differently written.

**The homogeneous shelf resolves cleanly here, and it could not for `design_spec`.** The design-spec entry was forced into a heterogeneous shelf with one admitted kind, which its own fixture README calls a contradiction, because `design_spec` requires the facet `doc_type` and a homogeneous shelf refuses a document that restates its placement. `kinds.standard` requires no facet and forbids `spec_layer`, so `homogeneous: true, kind: standard` resolves with nothing to declare. Same library, second entry, and the contradiction is gone. The forced exclusion is gone with it: n8n's rule files are under `.agents/`, so the vendored package under `packages/` sits outside the corpus root and no exclusion is needed.

**`obligations.OB-SS-1` carries `class: coherence`, and this corpus is its first member anywhere.** [Spec 4](https://github.com/headwater-ai/headwater/blob/main/docs/spec/04-assurance-model.md) and [HW-OBL-0114](https://github.com/headwater-ai/headwater/blob/main/docs/obligations/0114-a-control-that-names-a-sweep-marks-its-obligation-verified-with-nothing-run.md) both record that this repository's own register declares no coherence-class obligation, so a sweep here discharges nothing. Selecting this entry over a real corpus gives that class its first member in the library. It still discharges nothing, because OB-SS-1's disposition is `unverifiable` and no control names a `sweep:` mechanism. The gap moves from hypothetical to visible, which is what a worked instance is for.

## Findings

All eight below go to [13 — Open obligations](https://github.com/headwater-ai/headwater/blob/main/docs/spec/13-open-obligations.md) as a separate change after this entry merges. That is the rule the [library index](../README.md#where-a-finding-goes) states. Finding 6 was found by running the engine rather than by reading the specification.

**1. Tier 2 — the tradition's registered-deviation check cannot be declared.** [Spec 2](https://github.com/headwater-ai/headwater/blob/main/docs/spec/02-taxonomy-model.md#the-decision-relation-vocabulary) names one check of four that the decision-relation vocabulary brings:

    a `does_not_comply_with` edge that points at a `current` standard, which is a registered deviation and must carry an owner and an expiry

The relation is defined-but-unenabled package content, reached by `$package.optional.does_not_comply_with`, and `package.optional` holds nothing. So this entry cannot enable it. It does not declare the name either, because the package would collide with the address the day `package.optional` holds the relation. The central negative edge of the tradition is unreachable, and `OB-SS-3` carries the gap.

**2. Tier 2 — the negative edge and the positive edge point opposite ways, and reading precedence cannot hold both.** Spec 2 spells `does_not_comply_with` from the deviating document to the standard, inside the governance family, whose derived precedence makes the source govern. That says the deviating document governs the reading of the standard it violates. This entry spells its positive edge from the standard, which is the only spelling the precedence clause permits. Either the family assignment of the negative edge or the precedence clause is wrong, and no reading makes both right.

**3. Tier 2 — a base shelf's path is closed to every entry that models the same territory.** The layout the tradition uses is spec 2's own contract-sidecar sketch, `specifications/<component>/functional.md`. The base holds `shelves.specifications` at `docs/specifications/**`, and shelf determinism refuses a second pattern over it. So the entry that models the specification tradition cannot use the word the tradition uses, and it declares `docs/component-specs/**` instead. This is the design-spec entry's second finding seen from the shelf side rather than the section side. An opinionated default in a minimal base costs every later entry over the same ground.

**4. Tier 2 — the normative-keyword voice regime has no declarable form.** Spec 2's worked example promises "normative keywords on standards" for the product-suite column and "mandatory normative keyword usage" for the regulated one. A voice regime carries a `forbid` list and nothing else. A requirement that every normative sentence uses one of a fixed keyword set is unexpressible. This entry binds `declarative` on all three kinds and states the gap here. A standard whose requirements avoid every normative keyword passes every rule.

**5. Tier 2 — a standard's requirements are sub-document objects and nothing can name one.** This is the design-spec entry's third finding arriving from a third tradition. A conformance claim wants to say that one technical specification meets requirement 3 of one standard, and identity in this language is per document. `OB-SS-2` carries the gap and names contract sidecars as the place the answer probably lives. Nothing declares how a criterion identifier reaches the graph, and every citation in the fixture corpus is prose that no rule reads.

**6. Tier 2 — an unknown discriminator value removes a document from every check, and no gate reports it.** This one was measured rather than reasoned. The fixture corpus plants `spec_layer: interface_spec` on one document. Kind resolution stops, the census carries the row, and no rule instantiates over the document. A run with that document alone reports 0 findings and `headwater check --strict` exits 0. `engine/crates/check/src/coverage.rs` states the ruling deliberately, so this is not an engine defect. It is a property of every heterogeneous shelf, and the design-spec entry's `spec_series` shelf has carried it since the library opened. A typo in one metadata value is indistinguishable from a clean document at the gate. The [fixture README](https://github.com/headwater-ai/headwater/blob/main/docs/taxonomies/standards-spec/fixtures/README.md#planted-defect-4-reports-nothing-and-that-is-the-measurement) holds the run.

**7. Tier 3 — the bundle-set sketch names `standards` and this entry is `standards-spec`.** [Spec 7](https://github.com/headwater-ai/headwater/blob/main/docs/spec/07-distribution-and-federation.md) lists `standards: {requires: []}` and `compliance: {requires: [standards, evidence]}` in its bundle-set example. Spec 13 says the bundle set waits on a first adopter, and the library index says every admitted entry is a revision of that guess. This entry is that revision for the standards cluster, and it is named for the whole ladder rather than for one shelf. The two must not drift.

**8. Tier 3 — assumptions this draft took.** Neither shelf declares a `layout`, because a layout has no form for a per-component subdirectory and the tradition's files are `<component>/functional.md`. `volatility: stable` on `spec_layer` is the same assumption the design-spec entry recorded for `doc_type`. Whether an entry may declare an identifier scheme with no namespace still has two live patterns in this library and no ruling. The two expectation windows are judgments rather than measurements. The section on assumptions above states each one with its reasoning.
