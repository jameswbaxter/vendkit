# The Diátaxis taxonomy

Diátaxis sorts end-user documentation into four modes: tutorial, how-to guide, reference and explanation. This entry ships the four as one facet, `reader_mode`, over the kinds a corpus already has.

It is a **partial entry**. It declares no kind, no shelf, no purpose and no relation, so it satisfies none of the invariant core on its own and it stands on a base that already does. [The library index](../README.md#admission-criteria) names that shape and asks an entry to state it, and [The core, declared](#the-core-declared) states it.

This file states the tradition, the prior art, and the argument that a reader mode is a facet here rather than a shelf. It also states what an author does with the two cases the mapping does not cover, because a mapping that pretends to be total sends the author to invent a value.

## The prior art

Diátaxis is Daniele Procida's, published at [diataxis.fr](https://diataxis.fr/), first written up in 2017 under an earlier name and renamed since. The site is the canonical statement and this file does not restate it. What is here is the part a taxonomy has to decide, which the method leaves to whoever applies it.

The four modes are widely copied, and the method's own site lists the projects that took them. Django's documentation is organized along the same four lines and its author has worked on it. A reader who works in documentation recognizes the four names without a gloss. That is criterion 1 of the library index: the tradition is named, the prior art is citable, and it predates this entry by years.

**The citation licenses no claim that the method works.** [HW-EVAL-adjacent-work §M](https://github.com/headwater-ai/headwater/blob/main/docs/evaluations/adjacent-work.md#m--what-the-survey-shows-as-a-whole-convergence-is-not-evidence) rules that a survey of converging practice is not evidence of efficacy, and that ruling covers this entry. Many projects adopting a scheme is a fact about adoption. This entry ships the four modes because adopters ask for them by name, and it claims nothing about what a corpus gains from them.

**One convergence is worth recording, under the same caution.** [HW-EVAL-adjacent-work §S.7](https://github.com/headwater-ai/headwater/blob/main/docs/evaluations/adjacent-work.md#s7-four-documentation-modes-found-here-before-they-were-consulted) records that the [first-run walkthrough](https://github.com/headwater-ai/headwater/blob/main/docs/evaluations/default-taxonomy-first-run.md) reached one of the four splits against this repository before anybody read the method. Specification parts state a model and argue for it, and `docs/evaluations/` carries the argument. That is reference against explanation, found by measurement. It is one split of four, found once, and it is a coincidence worth stating rather than a result.

## What the tradition converges on

Two axes and four cells. Practical against theoretical, and the reader at work against the reader studying. A tutorial is practical study, a how-to guide is practical work, reference is theoretical work, and explanation is theoretical study.

The method's central claim is not the grid. It is that a page which serves two of the four serves neither, and that the remedy is to split the page. The grid is the diagnostic and the split is the treatment.

The second claim is that the four are exhaustive over end-user documentation. A page that fits none of them is a page nobody has separated yet, rather than evidence of a fifth mode. This entry takes both claims as the tradition's, which is why `values` is closed at four and why the guidance for a mixed page says split it.

## Why a reader mode is a facet here and not a shelf

This is the argument the entry exists to make, and it is made in this repository's terms rather than in the method's. [HW-EVAL-theoretical-foundations §H](https://github.com/headwater-ai/headwater/blob/main/docs/evaluations/theoretical-foundations.md#h-theory-considered-and-set-aside) already ruled the conclusion — "available as an optional facet for adopters who want it, but it is not the spine" — and it ruled it in one sentence. The reasoning below is what that sentence stands on.

**A kind carries eight things and a reader mode supplies at most one.** [Spec 1](https://github.com/headwater-ai/headwater/blob/main/docs/spec/01-conceptual-model.md#kind) lists what a kind carries: a purpose, a section contract, a facet schema, a voice regime, a lifecycle regime, an identifier scheme, the relations it may participate in, and a template. The four modes state a reader intent and they state nothing about the other seven. A tutorial and a reference page in one corpus are written, reviewed, superseded and cited by the same machinery. Nothing in the method says otherwise, because the method is about how a page reads and not about what authority it carries.

**The test that separates the two is whether the property co-varies with the contract.** [Spec 2](https://github.com/headwater-ai/headwater/blob/main/docs/spec/02-taxonomy-model.md#kinds-are-rigid-states-are-not) makes a kind what a document permanently is, and a facet a property of one. Two documents that differ only in a value are one kind with a facet. A `design_spec` written as reference and a `design_spec` written as explanation take the same purpose, the same lifecycle ladder, the same identifier scheme and the same edges. What differs is who arrives and what they came to do, which [spec 1](https://github.com/headwater-ai/headwater/blob/main/docs/spec/01-conceptual-model.md#facet) puts in the list a facet exists to carry: "lifecycle state, freshness, ownership, scope, audience, provenance, confidentiality".

**Placement is primary, and a corpus has one primary axis.** [Spec 2](https://github.com/headwater-ai/headwater/blob/main/docs/spec/02-taxonomy-model.md#placement-is-primary-metadata-fills-the-gap) makes the directory the loudest signal a document sends, and it makes a homogeneous shelf forbid the discriminator facet outright. To make reader mode a shelf is to lay the corpus out by reader mode: one directory per mode, and one mode per location. A governed corpus has already spent that axis on authority and lifecycle, with decisions in one place and specifications in another. It cannot spend it twice.

**Orthogonality is what decides which corpus gets which form, and it decides both ways.** [Spec 2](https://github.com/headwater-ai/headwater/blob/main/docs/spec/02-taxonomy-model.md#facet-acceptance-tests) names orthogonality over two facets that are near-perfectly correlated: the redundant one does no work and will eventually disagree with the other. The canon is corpus-measured and advisory, and spec 2 is explicit about what it does with a pair it finds. `taxonomy audit` reports the pair and does not reject it, because the right fix is a judgment: sometimes you delete a facet, and sometimes you discover that the shelf split was wrong. In a governed corpus the shelf states authority and the mode states reader intent, the two are independent, and the facet earns its place. In a corpus that is only a documentation site, the shelf and the mode would be one fact written twice, and the audit would report that pair on every run without ever deciding it. There the honest declaration is a kind set rather than a facet, which is the second entry that [#6](https://github.com/headwater-ai/headwater/issues/6) split out. **The judgment the audit hands back is the one this doctrine exists to make before an adopter meets it.**

**So the two forms are not rival readings of one question.** They are the same observation applied to two corpora. The owner's ruling on #6 is that both ship, and the argument above is why neither is the general case: the answer follows from what else claims the placement axis, and no entry can know that about an adopter.

**A purpose is where the two forms actually meet.** [Spec 2](https://github.com/headwater-ai/headwater/blob/main/docs/spec/02-taxonomy-model.md#purpose-is-declared-not-implied) makes a purpose the reader intent a kind exists to satisfy, declared rather than implied, and routing matches a task against it before it matches any text. The four modes are four reader intents. Where nothing else claims the purpose slot, they become kinds and each one declares its own purpose. Where the kinds already serve `rationale`, `behavior`, `evidence` and `obligation`, the modes cut across all four and there is no slot left to claim.

**This repository met that boundary and drew it the same way.** Its adopter overlay declares a `tutorial` kind with `purpose: procedure`, and the comment beside the declaration states that this is not a Diátaxis kind: "a reader mode is not a kind here even when its name matches one". The kind is there because nothing else in the corpus answered "how do I start". A document of that kind and a document carrying `reader_mode: tutorial` are different claims, and an adopter who holds both should read the next section before setting the second.

## The facet

One declaration, four values, and no residual.

`reader_mode` is the name. `mode` is a word a second entry would want, and criterion 6 of the library index gives an address to whichever entry claims it first, so a general word taken by a partial entry is an address taken from everybody. `diataxis_mode` names the method rather than the property, and a facet names what a document is. `reader_mode` is the term [HW-EVAL-theoretical-foundations](https://github.com/headwater-ai/headwater/blob/main/docs/evaluations/theoretical-foundations.md#h-theory-considered-and-set-aside) already uses for this idea.

`required: false`, and that is the load the declaration carries rather than a softening of it. The next section states two populations that take no value, and a required facet would force one onto both. A wrong value in a closed set is worse than no value, because a filter over the set then returns the document as though a person had judged it.

`volatility: stable` is a claim, and it is the tradition's rather than this entry's. A page has one mode, and a page that changes mode has been rewritten into a different page. So the value does not drift under a document that stays itself. The consequence in the engine is narrow and it is the useful one: [spec 2](https://github.com/headwater-ai/headwater/blob/main/docs/spec/02-taxonomy-model.md#facet-acceptance-tests) forbids a `mutable` value in an identifier, a path or a shelf pattern, so `mutable` would forbid the mode-partitioned site that is the most common reason to adopt the method. `stable` permits that layout and requires nothing.

No `role`. The role registry is closed at six, none of them is this, and a role is the only thing about a facet that an emitter may read by name. An adopter who wants the corpus partitioned by mode writes this facet into `shelves.<s>.group_by`, which is a **shelf** member rather than a projection member: the meta-schema puts `group_by` inside the shelf block, and the projection block has no such key. That is the argument above arriving as a declaration. To partition a corpus by reader mode is a placement decision, so it belongs to whoever owns the placement axis, and that is the adopter and never this entry.

**The facet attaches at `kinds.governed_document.facets.optional`, which is one line and the line that makes the entry admissible.** `governed_document` is the base's abstract kind, and every concrete kind of every entry inherits from it, so one `optional` line offers the facet to a whole corpus without this entry naming one kind of any other entry. That keeps the bundle add-only and its address set disjoint, which is criteria 3 and 6.

**What that line does, exactly, and what it does not do.** It satisfies the relevance canon, which `headwater taxonomy validate` refuses a facet without: a facet that no kind requires or lists as optional, that no shelf discriminates on, that no expectation reads and that carries no role, is refused as unread. It does not decide which documents the value check reads. `facet.value.not_permitted` generates one instance for every kind that does not `forbid` an enumerated facet, so a value outside the four is refused on any kind in the corpus, whether or not that kind lists the facet at all. Both halves were measured, and [the fixtures](https://github.com/headwater-ai/headwater/blob/main/docs/taxonomies/diataxis/fixtures/README.md#the-two-controls) record the two runs.

## The two cases the mapping does not cover

[HW-EVAL-theoretical-foundations](https://github.com/headwater-ai/headwater/blob/main/docs/evaluations/theoretical-foundations.md#h-theory-considered-and-set-aside) names both when it sets the method aside, and neither is a defect in the method. A scheme for end-user documentation meets documents that are not end-user documentation, and it meets pages that nobody has split yet.

### A document that mixes modes

**Split it.** That is the tradition's own remedy and this entry does not soften it. A page that explains and also lists is two pages, and the reader who came for one of them reads past the other.

**Until it is split, leave `reader_mode` unset.** The facet is optional for this case first. A mixed page given the value of its larger half is worse than a mixed page with no value, because a reader who filters on that value gets the page and a reader who filters on the other half does not.

**Nothing detects a mixed page, and no rule ever will.** To decide that a page holds two modes is to reason about what the prose asserts, and [spec 1](https://github.com/headwater-ai/headwater/blob/main/docs/spec/01-conceptual-model.md#two-layers-terminology-and-assertions) draws the boundary that puts that outside the engine. So the facet records a judgment and never discovers one, and the entry declares the invariant as `OB-DX-1` with an `unverifiable` disposition rather than pretending a check is owed. `docs/specifications/beacon-overview.md` in the fixture corpus is that page, and the run over the corpus reports nothing about its mixing. It does report `identifier.unusable` against the file, which is the base package's own defect and says nothing about reader modes.

### A kind that no mode fits

**Leave `reader_mode` unset, and do not split.** [HW-EVAL-theoretical-foundations](https://github.com/headwater-ai/headwater/blob/main/docs/evaluations/theoretical-foundations.md) names three populations: values statements, registers and decision records. None of them is end-user documentation, and a reader does not arrive at one in a reader mode. A reader arrives at a decision record because they need to know what was ruled and by what authority.

**Splitting is the wrong remedy here, and that is what separates this case from the first one.** A mixed page has two readers and one file, and a split gives each reader a file. A decision record has one reader and one file already. To split it by mode would separate the ruling from the reasoning that a reader of the ruling needs.

**This repository's own decision register is the live instance of the distortion that [HW-EVAL-theoretical-foundations](https://github.com/headwater-ai/headwater/blob/main/docs/evaluations/theoretical-foundations.md) predicted.** `docs/spec/09-decisions.md` resolves to the kind `decision_register`, on the heterogeneous `spec_series` shelf, serving the purpose `rationale`. As a page it is an index of the settled decisions of this project, which reads as reference. Each entry summarizes an argument and names the evidence that closed it, which reads as explanation. Neither value is right for the file, both are right for a part of it, and the split that the first case prescribes would separate a register from its own entries. So the register takes no value, and that is the correct outcome rather than a gap in it.

**A third case is worth naming because the canons catch it and this section could not.** Where a kind already means one mode, the facet on that kind separates nothing, and every document of the kind takes the same value. That is the differentiation canon, which `taxonomy audit` measures over a corpus rather than over a schema. The remedy is to leave the facet unset on that kind, or not to select this bundle at all. An adopter whose every kind is in that position has a corpus that the kind-set form fits and this one does not.

## The core, declared

Criterion 5 asks which facet carries each core role and which kind serves each core purpose. This entry answers that it supplies none of them, and the answer is the declaration rather than a hole in it.

| Core requirement | What supplies it | Where |
|---|---|---|
| `facet_role: state` | `status` | `headwater/standard` |
| `facet_role: freshness` | `last_verified` | `headwater/standard` |
| `facet_role: scent` | `summary` | `headwater/standard` |
| `purpose: rationale` | `kinds.decision` | `headwater/standard` |
| `purpose: behavior` | `kinds.specification` | `headwater/standard` |
| lifecycle-sensitive succession | `relations.supersedes`, with `on_target: {set_state: superseded}` | `headwater/standard` |

`reader_mode` carries no role, and a role is the only thing about a facet that the engine reads by name. So this entry adds a reading of a corpus and it changes nothing about how that corpus is governed. That is the whole claim of a facet-only entry, and it is why the entry composes with any base rather than with one.

## What this entry deliberately does not declare

**No kind.** The four modes are not kinds here, and the argument above is the reason. The kind-set form of the same tradition is a separate entry with a separate bar.

**No shelf.** A shelf is a placement rule and this entry makes no claim about where a document sits. An adopter who wants a mode-partitioned layout writes the shelves in their own overlay, which `volatility: stable` permits.

**No template.** The library index asks for one template per concrete kind that an entry adds, and this entry adds none, so `templates/` is absent. That a facet-only entry is admissible at all is settled by name rather than by a reading of the anatomy: [#2](https://github.com/headwater-ai/headwater/issues/2) writes criterion 5 around "a facet-only overlay like Diátaxis that composes onto a base rather than standing alone", and the index carries the same sentence. What nobody has written is what such an entry puts in the anatomy's `templates/` slot, which is the fourth finding below.

**No requirement on any kind.** No `facets.require` anywhere, on this entry's kinds or on anybody else's. The first is impossible because this entry has no kinds. The second is what [HW-OBL-0040](https://github.com/headwater-ai/headwater/blob/main/docs/obligations/0040-composition-between-two-library-entries-has-no-add-only-form.md) holds, and the second finding below states what this entry learned about it and what it did not settle.

**No composition demonstration.** The facet applied as an overlay onto another entry's instance corpus is the deliverable that waits on HW-OBL-0040, and it moved to the second issue with the kind set.

## The one invariant, declared as an obligation with no control

`OB-DX-1` states the method's central claim: a document is in one reader mode. Its disposition is `unverifiable` rather than `gap`, and the difference matters. A `gap` says a control is owed and names what would discharge it. `unverifiable` says no control of this engine can ever discharge it, and states why.

The reasoning is the one under the first hard case. A document that mixes modes is a document whose prose serves two readers, and to find that is to reason about what the prose asserts. The closed value set catches a document that names a fifth mode. It cannot catch a document that names one mode truthfully and then serves two.

## What this draft assumed

**That a hyphen is legal in a facet value.** `how-to` is what the tradition calls the mode, and the meta-schema types a value as a string with no grammar over it. The value is never written as an address, because the `guidance` keys sit under one `add` at `facets.reader_mode` rather than under one address each. A run of `taxonomy validate` and `taxonomy resolve` over the entry accepts it.

**That `optional` on an abstract kind reaches every kind below it.** [Spec 2](https://github.com/headwater-ai/headwater/blob/main/docs/spec/02-taxonomy-model.md#abstract-kinds) states that facet and section requirements union down the chain, and it states it about requirements. It does not state it about `optional`. The resolver reads `require` and `optional` together when it decides what a kind's facets are, so the reading holds against this engine, and no document states it.

**That a partial entry may leave the `templates/` slot empty.** See the fourth finding.

## Findings

Five, in the tiers the [library index](../README.md#a-finding-has-a-tier-and-the-tier-fixes-the-timing) fixes. None of them is tier 1, so none of them stops this entry, and each one goes to [13 — Open obligations](https://github.com/headwater-ai/headwater/blob/main/docs/spec/13-open-obligations.md) in a change of its own after this entry merges.

**1. Nothing compares `extends:` against the package it names, and both existing entries carry a stale value. (Tier 2.)** The meta-schema declares `extends` at the root of an overlay source and shape-validates it as a scalar. Nothing in the engine compares that scalar to the version of the package under it. `design-spec` and `decision-record` both declare `extends: headwater/standard@1.0.0`, and the package they extend is at 3.3.0. The example block in the [library index](../README.md#what-an-entry-ships) shows the same number, which is where both copies came from. Nothing reports the divergence, and a bundle written against a base that has since changed under it looks identical to one written yesterday. This entry writes `3.3.0`, which is true of it today and which nothing holds it to tomorrow. The remedy is either a comparison at resolve time or the removal of the field. [#295](https://github.com/headwater-ai/headwater/issues/295) answered this: [HW-DR-0040](https://github.com/headwater-ai/headwater/blob/main/docs/decisions/0040-q40-whether-extends-bundle-requires-and-an-overlay-s-taxonomy-key-are-a-mechanism-or-a-label.md) rules `extends` a label rather than a mechanism, the third remedy this entry did not name, and the specification and the meta-schema now state the gap plainly instead of closing it.

**2. The relevance canon accepts `optional`, and the wall that HW-OBL-0040 records is narrower than #6 stated. (Tier 2.)** [#6](https://github.com/headwater-ai/headwater/issues/6) records that "a facet that no kind requires and no shelf discriminates on fails the relevance canon", and it concludes that a facet-only entry "has to reach `kinds.<k>.facets.require` on a design-spec kind". The first half is right and the conclusion does not follow. The resolver's denominator for the canon reads `require` and `optional` together, so an `optional` listing satisfies relevance, and this entry ships on that route. [The fixtures](https://github.com/headwater-ai/headwater/blob/main/docs/taxonomies/diataxis/fixtures/README.md#the-two-controls) record the run that measured it.

This sharpens HW-OBL-0040 and it does not settle it. What that record holds is that vocabulary two traditions need cannot live in an entry, because the first entry to claim an address owns it. That is still true, and requiring a facet on a kind another entry declared is still the operation with no add-only form. What is now known is that a facet-only entry does not need that operation to be admissible. Whether a demonstration over another entry's corpus needs it is a question for the demonstration, and this entry did not run one.

**3. A facet-only entry has no kinds, so its worked corpus borrows them, and the base supplies two. (Tier 3.)** Criterion 4 asks every entry for a worked instance corpus. This entry has no kind to type a document as, so the fixture corpus takes `decision` and `specification` from the base. Every mode page in it is therefore a `specification`, and it carries the `Scope` and `Behavior` headings that the base's section contract requires. A tutorial page does not naturally take those two headings. The fixture corpus states this, and it is the argument for the kind-set entry arriving from the fixtures rather than from an argument.

**4. The anatomy has a `templates/` slot and nothing states what a partial entry puts in it. (Tier 3.)** That a facet-only entry is admissible is not in doubt and it never was: [#2](https://github.com/headwater-ai/headwater/issues/2) settles it by name under criterion 5, writing that clause around "a facet-only overlay like Diátaxis that composes onto a base rather than standing alone". What no document states is the anatomy's other half. Criterion 2 asks for the whole anatomy and the [what an entry ships](../README.md#what-an-entry-ships) table asks for one template per concrete kind, and an entry with no concrete kind has no kind for a template to shape. This entry leaves the directory absent, and that is a reading rather than a rule.

**5. Two documents of this corpus state the wrong second axis for the method. (Tier 2.)** [HW-EVAL-adjacent-work §S.7](https://github.com/headwater-ai/headwater/blob/main/docs/evaluations/adjacent-work.md#s7-four-documentation-modes-found-here-before-they-were-consulted) writes the two axes as "practical against theoretical and specific against general", and [the first-contact evaluation](https://github.com/headwater-ai/headwater/blob/main/docs/evaluations/first-contact.md) writes the same pair. The method's second axis is not generality. It is whether the reader is at work or at study, which is why a tutorial and a how-to guide are both practical and are still two different pages. Under "specific against general" a tutorial and a how-to guide land in one cell and the grid collapses to three. [What the tradition converges on](#what-the-tradition-converges-on) above states the axis this entry works from, and the divergence is recorded here rather than silently. The remedy is one sentence in each of the two documents, and an entry never edits a specification part, so this travels as a change of its own.

**One thing the fixture run reproduced that is not a new finding.** Six of the seven findings in the run are `identifier.unusable` against the base's `specification` kind, which mints under no identifier scheme. That is [HW-OBL-0107](https://github.com/headwater-ai/headwater/blob/main/docs/obligations/0107-the-base-package-ships-a-kind-that-the-scaffolder-refuses-to-write.md), already recorded, and it is reported here because a fresh corpus reproduces it on its first run.

## What an adopter takes

Select the bundle beside whatever base taxonomy already types the corpus. Nothing else changes: no document is reclassified, no shelf moves, and no check that ran before stops running.

Then set `reader_mode` on the documents a reader arrives at in a reader mode, and leave it unset everywhere else. The two sections above say which is which. A corpus where the answer is "unset" almost everywhere has learned something about itself, and the honest response is to deselect the bundle rather than to fill the field in.
