# The brd-prd taxonomy

The requirements handoff. A business requirements document states the need, the requirements that follow from it and the measures of success. A product requirements document states the problem for one product, what a solution must do and what done means.

This entry declares two kinds, one purpose, one voice regime, one facet, one relation, one shelf, two identifier schemes and three obligations. It requires no other entry. This file states what the tradition is, why each declaration is here, and what this draft assumed where the specification is silent.

## The prior art

**IIBA's BABOK Guide, version 3, released April 2015.** Section 2.3 declares a requirements classification schema of four categories. Business requirements are "statements of goals, objectives, and outcomes that describe why a change has been initiated". Stakeholder requirements "describe the needs of stakeholders that must be met in order to achieve the business requirements". Solution requirements "describe the capabilities and qualities of a solution that meets the stakeholder requirements", and they divide into functional and non-functional. Transition requirements describe capabilities that carry the solution from the current state to the future state. Nothing needs them once the change is complete.

The four are categories of requirement rather than a list of documents. The business analysis tradition then files them: a business requirements document carries the business tier, and often the stakeholder tier with it. Section 2.3 spells the movement between tiers as a bridge: a stakeholder requirement "may serve as a bridge between business and solution requirements". The word `elaborate` is BABOK's elsewhere rather than section 2.3's, and the guide writes of progressive elaboration in its own perspectives. This entry takes its relation name from that wider vocabulary, and the [relation section](#the-relation-and-who-creates-the-edge) says so.

**The Pragmatic Marketing Framework, in continuous use since 1993.** The organization that publishes it began in 1993 and became the Pragmatic Institute in late 2018. The copyright line on the framework itself reads from 1993. The framework is a grid of activities across the product life cycle, and its grid carries one Requirements box rather than a box per document. The two documents come from Pragmatic's own article, *Writing the Market Requirements Document*, which is the source of every quotation in this paragraph. A Market Requirements Document identifies and prioritizes market problems and "describes the why behind the product". A Product Requirements Document "translates the MRD into specific product features" and describes the how. Pragmatic asks the product manager to write market requirements "in the words of the persona, in business or personal terms". The alternative it names is "offering a technical definition".

**Marty Cagan, Silicon Valley Product Group, "Revisiting the Product Spec", October 2006.** Cagan is the source for the slot and he disputes the form, and this entry states both. He puts the responsibility plainly. The product manager has "to make sure that you deliver to the engineering team a product spec that describes a product that will be successful". The audience of that spec is "engineering, QA, customer service, marketing, site operations, sales". He then argues against the document. Most specs "take too long to write, they are seldom read". They also fail to "provide the necessary detail, address the difficult questions, or contain the critical information they need to". His replacement is a high-fidelity prototype with annotated pages beside it.

**The third source is a dissent about the medium and not about the slot.** A high-fidelity prototype is still a product-level statement of what must be built. Product hands it to engineering, and it answers to a business need above it. An adopter who follows Cagan keeps the `prd` kind and writes a short document that names the prototype. Nothing in this entry requires the document to be long.

## What the three traditions converge on, and where they differ

**Two levels, and one handoff between them.** All three separate a statement of the need from a statement of what must be built. BABOK separates business requirements from solution requirements. Pragmatic separates the market document from the product document. Cagan separates the business case and opportunity assessment from the spec that reaches engineering. The convergence is the shape rather than the vocabulary.

**The upper level owns the why, and it is written before any solution.** BABOK's business requirements state why a change was initiated. Pragmatic's market document states the why and is written in the words of the persona. Cagan's opportunity assessment answers what problem this solves and for whom. All three treat a solution named at this level as a failure of the level.

**The lower level owns done.** A product requirements document is where a team agrees what it is building. Pragmatic's product document translates the market problems into features. Cagan asks for a spec that "describes a product that will be successful", which is a claim about the finished thing. BABOK's solution requirements are the tier a test can fail.

**Both documents are prospective, and no source treats that as a defect.** Every sentence of both is about a product that does not exist. That is not a stylistic property of the tradition. It is what a requirement is, and it is why this entry declares a voice regime of its own.

**Where they differ is the number of tiers and the name of the upper document.** BABOK names four categories and permits a document to carry more than one. Pragmatic names the upper document for the market. Cagan writes about one document and one prototype. This entry takes the two-document shape, because it is the one all three converge on as artifacts. It declares no kind for a tier that only one source names.

## Why two kinds and not three

**Not one.** A taxonomy that merges the two loses the handoff. The relation between them is what makes a product document stale when its business need is superseded. A self-edge on one kind says nothing.

**Not four.** BABOK names four categories of requirement, and a kind per category would be a structure taken from one source rather than from the convergence. The categories are also not documents. An adopter who keeps the stakeholder tier and the transition tier apart adds two values and two kinds in an overlay of their own.

### The MRD gets doctrine and no kind

`brd` carries two traditions' names for one slot. In IIBA's BABOK Guide it is the artifact that carries the business requirements tier, and often the stakeholder tier with it. In the Pragmatic Marketing Framework the same slot is the Market Requirements Document, which states the why at market level. The two are not the same document, and this entry does not claim they are. They answer one reader intent at one point in the handoff, and no relation runs between them, so one kind serves both. An adopter whose corpus keeps both writes two documents of the `brd` kind.

Two kinds serving one purpose is legal here, and the ground is narrow. [Spec 2](https://github.com/headwater-ai/headwater/blob/main/docs/spec/02-taxonomy-model.md#purpose-is-declared-not-implied) permits several kinds per purpose and refuses only a kind serving two unrelated purposes. The precedent that licenses a split is the design-spec entry's `review_prompt` and `review_record` under one `evidence` purpose. The standards-spec entry followed it with two specification kinds under `behavior`. In both cases the ground is that a relation runs between them. `elaborates` runs between `brd` and `prd`. Nothing runs between a BRD and an MRD, so that ground is absent and a third kind would rest on nothing.

**The honest limitation beside it: no facet distinguishes the two.** A `requirement_origin` facet with the values `business` and `market` reads well, and no check, projection, routing rule or expectation would read it. The relevance canon of [spec 2](https://github.com/headwater-ai/headwater/blob/main/docs/spec/02-taxonomy-model.md#facet-acceptance-tests) refuses a facet that nothing reads, and `taxonomy validate` enforces it. [Finding 7](#findings) records the omission.

### Why the names are acronyms

`brd` and `prd` are the first acronym kinds in this library. Every other kind here is a spelled-out lower_snake_case noun phrase, and that house style is the reason to look twice.

The deviation is correct. Criterion 1 asks the entry to model the tradition's own terms, and the tradition's own term is the acronym. Practitioners say "the PRD". Cagan writes PRD. Pragmatic writes MRD and PRD in the framework itself. Nobody in any of the three sources writes "product requirements document" twice in a row. A kind named `business_requirements_document` would be this entry renaming the tradition to suit a naming habit. That is the error the standards-spec entry refused when it declined to hedge `standard` into `governing_standard`.

`business_requirements` and `solution_requirements` were also considered, and refused. They are faithful to one of the three sources and unrecognizable to the other two. They also name categories of requirement rather than the two documents.

## The relation, and who creates the edge

`elaborates` runs from the product document to the business one, in the `derivation` family, with the business document as the nucleus.

The name is the tradition's own verb, and the [prior art](#the-prior-art) above states which source supplies it. BABOK writes of progressive elaboration and of elaborating requirements, though its classification schema calls the tier movement a bridge. Pragmatic translates the why into the how. `derives_from` was refused. [Spec 2](https://github.com/headwater-ai/headwater/blob/main/docs/spec/02-taxonomy-model.md#relation-families-and-nuclearity) uses that string as the generic example spelling of a derivation edge, in its own sample taxonomy. An entry that claims the most general name in a family closes that name to every later entry. `elaborates` is specific to this tradition, exactly as `realizes` was to the sibling entry.

The nuclearity is what this entry is for, and [spec 2](https://github.com/headwater-ai/headwater/blob/main/docs/spec/02-taxonomy-model.md#relation-families-and-nuclearity) lists four things it buys. Three land here.

- **Lifecycle inheritance.** A product document whose business document is superseded is stale the moment the succession lands, and the engine sees it structurally with no separate check. The `derivation` family is lifecycle-sensitive, which is what makes that true.
- **Context pruning.** An agent under a context budget drops the product document and keeps the business one. That is the right way round for a reader who asks why the work exists at all.
- **Orphan detection.** A business document that nothing elaborates is a real omission, and an unlinked product document is a generation defect. The fixture corpus reports the first and stays silent about the second, which is the measured form of this claim.

`composition` was refused. It would say the product document is part of the business one, and it is not. It is a second document about one initiative at a second level.

**One honest wrinkle.** A product requirements document written with no business document above it is common practice rather than a defect. Plenty of teams keep only a PRD. The nuclearity is still right, because it is a claim about this tradition's two-document shape. A corpus that keeps product documents alone is not running this tradition. The declaration reports the missing half only when a corpus opts into the shape. The expectation sits on `brd`, and no expectation sits on `prd`.

`created_by: scaffold`, on the argument the design-spec entry used for `applied_in`. The run that opens a product document from a business one writes both halves of the edge. No relation here is `created_by: author`, so this entry owes no author-edge sentence under the rule the [library index](../README.md#one-rule-of-the-base-is-not-a-criterion-here) states.

### Why the base's `traces_to` does not serve

The base already declares a relation that reaches from any governed document to any other. [Spec 2](https://github.com/headwater-ai/headwater/blob/main/docs/spec/02-taxonomy-model.md#the-decision-relation-vocabulary) glosses it as "Derives from a requirement, driver, or source". So `prd traces_to brd` is legal today with no declaration at all, and a reader is owed the reason this entry declares a relation anyway. Four reasons, and each one is mechanical rather than a matter of taste.

**The evidence family is not lifecycle-sensitive.** [Spec 2](https://github.com/headwater-ai/headwater/blob/main/docs/spec/02-taxonomy-model.md#relation-families-and-nuclearity)'s family table marks succession, derivation, governance and composition as lifecycle-sensitive and evidence as not. A product document whose business document is superseded would therefore not be stale. That is the single thing this relation exists to buy.

**`traces_to` declares no inverse and no reciprocity.** A reader who arrives at the business document learns nothing about which product documents answer it. No reciprocity rule fires on a half-written pair, and `headwater check --fix` has no back-link to write. The fixture corpus reports exactly that finding on a planted half-written `elaborates`, and it would report nothing on the same pair spelled `traces_to`.

**`traces_to` declares no nuclearity, and the evidence family's cell in the table is blank.** [Spec 2](https://github.com/headwater-ai/headwater/blob/main/docs/spec/02-taxonomy-model.md#reading-precedence-is-derived) says evidence carries no reading order, so context pruning has nothing to order by.

**`created_by: hook`.** A hook writes it, and the base means the hook that reads a change. Nothing mechanical turns a business need and a product document into a pair.

This entry also cannot repair `traces_to` in place. Adding an inverse to it is an `override`, and criterion 3 forbids an override outright.

## The voice regime, and why it is not `declarative`

The base declares exactly one voice regime. `regimes.voice.declarative` forbids `future_intent`, `change_narration` and `phased_rollout`. This entry declares `regimes.voice.prospective`, which forbids the second and the third and not the first, and it binds that regime on both kinds.

**A requirements document is prospective by construction.** A success measure states a value the business does not have. A release criterion states a condition the product does not meet. Both are statements about a product that does not exist, and `future_intent` is the category that forbids them. To bind `declarative` on these kinds is to report the tradition itself on every document of it.

**The precedent is exact and it is in this library.** The design-spec entry declares `regimes.voice.narrative` as an empty regime, which forbids nothing. It binds that regime on its evidence documents, on the ground that they are time-bound by nature. This entry takes the same move and stops short of an exemption. `change_narration` and `phased_rollout` still bite. A requirements document that narrates last quarter's rename is wrong here too. So is one that sequences its own delivery into phases. A phase belongs to the plan that owns it.

**This is not a declaration written around a finding.** [Finding 2](#findings) records the gap anyway, and the fixture corpus measures it rather than asserting it. A probe changed the two `voice:` lines to `declarative` and changed nothing else. The run reported four extra `future_intent` findings across four of the seven documents, and the [fixture README](https://github.com/headwater-ai/headwater/blob/main/docs/taxonomies/brd-prd/fixtures/README.md#what-the-bases-voice-regime-would-have-reported) holds the sentences the rule named. An adopter who selects only the base has no prospective voice available and has to declare one.

## The core, declared

This is a full entry rather than a partial one. It declares kinds, a shelf, a purpose, a relation and identifier schemes, so the partial-entry paragraph of criterion 5 does not apply to it. It satisfies the core **through the base, for every clause**, and it is not the first full entry to do so. The decision-record entry took the same position and stated it. Its one declared kind serves `obligation`, and its doctrine says that `rationale` is served by the base `decision` and that it "declares no behavior-serving kind".

| Core requirement | What satisfies it | Declared by |
|---|---|---|
| `facet_role: state` | `facets.status` | the base, inherited through `governed_document` |
| `facet_role: freshness` | `facets.last_verified` | the base, the same way |
| `facet_role: scent` | `facets.summary` | the base, the same way |
| `purpose: rationale` | `kinds.decision` | **the base.** This entry declares no rationale-serving kind |
| `purpose: behavior` | `kinds.specification` | **the base.** This entry declares no behavior-serving kind either |
| `relation_family: succession`, lifecycle-sensitive | `relations.supersedes` | the base, untouched |

`facets.status_since` is not a clause of the core, and the one participation expectation reads it, so it is listed here for a reader who checks. The base requires it on `governed_document`, which is what makes the expectation well-formed.

**This entry declares no facet that carries an engine role.** `requirement_tier` takes no `role`, for the reason the diataxis entry's `reader_mode` and the standards-spec entry's `spec_layer` take none. The role registry is closed at six and none of them is this.

**The purpose map, stated rather than implied.**

| Kind | Purpose | The reader intent |
|---|---|---|
| `brd` | `requirement`, new and declared by this entry | why are we doing this at all, and what would make it a success |
| `prd` | `requirement`, the same | what must we build, and what does done mean |

**Both base purposes were tested against these kinds and both fail on the base's own wording.** The base glosses `rationale` as "explain why a choice was made and what it forecloses", answering "why is it this way". That is retrospective and it is about a choice already taken. A business requirement states a prospective need. Routing settles it. Over a corpus that holds both, "why is it like this" reaches the decision and "why are we building this" reaches the business document. Two questions, and therefore two purposes. The base glosses `behavior` as "state what the system does, as it is now", answering "what may I rely on". A product requirement states what a product must do and does not yet do, and nothing in it may be relied on.

`purposes.constraint` is closed to this entry, because the standards-spec entry holds the address. It would be wrong here even if it were free. A standard is standing and corpus-wide, and a requirement is scoped to one initiative and retires when it is met.

### The reading criterion 5 does not supply

Criterion 5 asks an entry to name "which of its facets carry those roles, and which of its kinds serve those purposes". This entry names none of either. It declares no facet with a role and no kind serving `rationale` or `behavior`. The criterion supplies one escape, and it is written for a facet-only overlay such as Diátaxis. This entry is not that. It declares kinds, a shelf, a purpose, a relation and two identifier schemes. It composes onto a base that satisfies the core, and it adds a third reader intent the base does not carry.

**The reading this entry takes: a full entry satisfies criterion 5 by naming the declaration that satisfies each clause, whoever declared it.** The table above is that naming. An entry that serves no core purpose is admissible. The core is a property of the resolved taxonomy rather than of any one overlay. `taxonomy validate` checks core satisfiability on the merged result. The run in the pull request that carries this entry is the evidence. The resolved taxonomy satisfies the core with all five entries selected, and with this entry alone.

[Finding 1](#findings) asks for a ruling on the wording. This is the same class of hole the library index already records for the third entry. Criterion 2 had no reading for a partial entry, and that entry had to take one.

## What this entry deliberately does not declare

**A rationale-serving kind or a behavior-serving kind.** The base's `decision` serves the first and the base's `specification` serves the second. A kind invented to satisfy criterion 5 is a shape invented for the library, which criterion 1 refuses.

**A third kind for the market document.** [The MRD subsection](#the-mrd-gets-doctrine-and-no-kind) states the whole of it.

**A deviation edge.** The standards-spec entry cannot declare a registered deviation, because `does_not_comply_with` is reserved package content that `package.optional` does not hold. That is a real gap for a standards tradition and it is not one here. Neither cited tradition has a registered-deviation concept. A registered deviation is a durable, owned, expiring record that a component departs from a rule while the rule stays in force. BABOK has traceability, change control and approval. The product lineages have descoping and release trade-offs. In all of them the equivalent object is a status on one requirement, and never an edge between two documents. So this entry is not blocked by an empty `package.optional`, and [finding 5](#findings) states where the real gap is.

**A `requirement_origin` facet.** The relevance canon refuses a facet that nothing reads, and [finding 7](#findings) records the assumption.

**A shelf layout.** The shelf declares none, and [finding 7](#findings) records the reason.

**Any dependency on the standards-spec entry.** [Finding 3](#findings) states the whole argument.

## What this draft assumed

**That the discriminator value of a heterogeneous shelf is the kind name.** It is, and this is a measurement rather than a reading. `engine/crates/census/src/resolve.rs` matches the value against the shelf's `kinds` list by identity and stops when it finds no match. So `facets.requirement_tier` declares `brd` and `prd` as its values, and a facet named for a tier carries a value named for a kind. The design-spec entry's `doc_type` and the standards-spec entry's `spec_layer` have the same shape.

**That `volatility: stable` is right for `requirement_tier`.** The claim is that a document does not move from the business tier to the product tier. A document that reads as though it did has been rewritten into a second document. The design-spec entry recorded the same assumption for `doc_type` and nothing has ruled on it. The claim is what permits an adopter to put the value in a shelf path or a projection output.

**That an identifier scheme may carry no namespace.** The base declares none for `decision_id` and states why: a published package that named one would have every adopter mint under it. The decision-record and standards-spec entries follow the base, and the design-spec entry declares no scheme at all. So two live patterns exist, and no ruling separates them. This entry follows the base.

**That `minted-once` is right for both schemes.** A business document and a product document are cited from outside the corpus, in a roadmap, a board paper and a ticket. An identifier that a later reconciliation moved would break a citation that nobody inside the corpus can rewrite. The `{slug}` in each pattern is what makes this workable, because the name comes from the initiative rather than from a counter.

**That the expectation window is 90 days.** The number comes from no source. A business need that no product document answers after a quarter is a need nobody scoped. The severity is `warn`, so the finding is advisory, and an adopter who plans on a different cadence overrides it.

**That the shelf path is `docs/requirements/**`.** The tradition's own word is free here, and the contrast with the sibling entry is the point. The standards-spec entry records, as its third finding, that the base's `shelves.specifications` closed `docs/specifications/**` to the entry that models the specification tradition. That entry had to invent `docs/component-specs/**`. Nothing closes this one. The cost that finding names is about which words a minimal base spent, and not about entries in general.

## Findings

All seven below go to [13 — Open obligations](https://github.com/headwater-ai/headwater/blob/main/docs/spec/13-open-obligations.md) as a separate change after this entry merges. That is the rule the [library index](../README.md#where-a-finding-goes) states. None of them is tier 1, so none of them holds the entry.

**1. Tier 2 — criterion 5 has no reading for a full entry that serves no core purpose.** The criterion asks an entry to name which of its facets carry the core roles and which of its kinds serve `rationale` and `behavior`. It supplies an escape only for a facet-only overlay. This entry declares kinds, a shelf, a purpose, a relation and identifier schemes, and it serves a third reader intent instead of either core purpose. [The core, declared](#the-core-declared) takes a reading, and the wording of the criterion should either admit it or refuse it. **The reading is not new, and that is what makes the finding worth filing.** The [decision-record entry](../decision-record/doctrine.md#the-core-declared) reached the same position and read the criterion the same way, without naming the gap. So the criterion has now been read this way twice, by two entries that met it independently. This is the same class as the criterion-2 hole the third entry found, and it has one more occurrence behind it.

**2. Tier 2 — the base's only voice regime forbids the construction this tradition is made of.** `voice.declarative` forbids `future_intent`, and a requirements document states what a product must do before it exists. This entry declares `regimes.voice.prospective` on the design-spec entry's `narrative` precedent. The fixture corpus measures what `declarative` would have reported over the same prose: four extra findings across four of seven documents. An adopter who selects only the base has no prospective voice regime available. A corpus of requirements on the bare base therefore reports its own tradition. The base carries one voice regime and the language needs at least two.

**3. Tier 2 — the pipeline that both entries describe cannot be declared by either of them.** A corpus that keeps a business document, a product document, a functional specification and a technical specification wants an edge from `prd` to `functional_spec`. The natural spelling adds `prd` to `relations.realizes`'s endpoint list, which reaches into an address the standards-spec entry declared. That is not an `add`, and it is the second of the three blocked operations that [HW-OBL-0040](https://github.com/headwater-ai/headwater/blob/main/docs/obligations/0040-composition-between-two-library-entries-has-no-add-only-form.md) carries. The alternative is a new relation and `requires: [standards-spec]`. A dependency on an entry is a dependency on all of it. That is three kinds, a purpose, a facet, two relations, two shelves, three identifier schemes and three obligations. A product team that wants two requirements documents would inherit an internal-standard ladder it never asked for. The decision-record entry paid that cost once for one address, and this entry refuses it, so HW-OBL-0040 now has a record from each side.

**4. Tier 2 — sub-document identity, arriving again from another tradition.** Both cited traditions define traceability at the level of one requirement. BABOK's stakeholder tier bridges the business tier and the solution tier one requirement at a time, and Pragmatic asks which market problem each feature answers. Identity in this language is per document, and a requirement is a numbered line under a heading. So `OB-BP-1` and `OB-BP-2` state what the tradition asks for, and no declaration reaches it. Every citation in the fixture corpus is prose that no rule reads. This is the design-spec entry's third finding and the standards-spec entry's fifth, so `design-spec`, `standards-spec` and `brd-prd` each record it.

**5. Tier 2 — a descoped or unmet requirement has no form.** The product tradition's equivalent of a registered deviation is a status on one requirement, and never an edge between two documents. A requirement is descoped, deferred, out of scope or not met at release. Each of those is a state of a line inside a document. So the `does_not_comply_with` gap the standards-spec entry records is not the gap here. The gap is the same root seen from a different side. A requirement has no identity, so it has no state. `OB-BP-3` is unverifiable, and the descoping the tradition does every week is invisible to the corpus.

**6. Tier 3 — the bundle-set sketch has no name for this cluster.** [Spec 7](https://github.com/headwater-ai/headwater/blob/main/docs/spec/07-distribution-and-federation.md) lists `procedure`, `standards`, `evidence`, `proposals`, `operations` and `compliance` in its bundle-set example. `proposals` is the nearest and it is not this, because a requirement is accepted work rather than a proposal. Spec 13 says the bundle set waits on a first adopter, and the library index says every admitted entry is a revision of that guess. This entry is a revision the sketch has no box for, which is the same class as the standards-spec entry's seventh finding.

**7. Tier 3 — assumptions this draft took.** The shelf declares no `layout`, because a layout has no form for a per-initiative subdirectory and the tradition's files are `<initiative>/brd.md`. `volatility: stable` on `requirement_tier` is the assumption the design-spec entry recorded for `doc_type`. `brd` carries two traditions' names for one slot, and no facet distinguishes them, because a facet no check reads fails the relevance canon. Both kinds are named by acronym, which no other kind in this library is. The 90-day window is a judgment rather than a measurement. Whether an entry may declare an identifier scheme with no namespace still has two live patterns in this library and no ruling. The section on assumptions above states each one with its reasoning.
