# The canonical taxonomy library

This directory holds the canonical taxonomy library: a curated set of taxonomies, each one modeling a named documentation tradition. Every adopter would otherwise rediscover and re-encode their own tradition from nothing. The library is the alternative to that, published on the terms that [spec 7](https://github.com/headwater-ai/headwater/blob/main/docs/spec/07-distribution-and-federation.md) already fixed.

This file settles five things: what an entry is, what admits one, what each entry ships, what admits an assembly, and where each draft lives. It also collects the rulings that a draft works under, so that authoring an entry reopens none of them.

## An entry is a bundle

[Q3](https://github.com/headwater-ai/headwater/blob/main/docs/spec/09-decisions.md#q3--how-much-of-the-default-taxonomy-ships-in-the-box) settled the packaging mechanism before the library needed it. The base package `headwater/standard` is minimal and derived from the core. Optional content ships as a **bundle**, which is a publisher overlay that holds `add` operations and nothing else ([spec 7](https://github.com/headwater-ai/headwater/blob/main/docs/spec/07-distribution-and-federation.md#bundles-are-publisher-overlays-in-the-other-direction)). A library entry is one such overlay, plus the doctrine prose, the templates, and the fixtures that go with it.

The add-only rule comes from the resolver rather than from taste. Add-only overlays over disjoint addresses commute, so the static confluence check proves that every subset of bundles resolves ([spec 2](https://github.com/headwater-ai/headwater/blob/main/docs/spec/02-taxonomy-model.md#customization-by-composition)). The publisher runs that check once per release. No adopter can then select a combination of entries that fails.

## Admission criteria

Seven criteria. The first five confirm the sketch that the [epic](https://github.com/headwater-ai/headwater/issues/1) proposed, with one correction inside criterion 5. The last two follow from the confluence rule that criterion 3 rests on. A reviewer checks both mechanically once the engine exists.

**1. It models a named tradition with citable prior art.** A book, a published method, or a convention that many organizations copied. The doctrine prose carries the citations, and it states what the tradition converges on rather than asserting a shape. A structure invented for the library fails here, however neat it is. The test a reviewer applies: somebody who works in the tradition recognizes it, and the citation predates the entry.

**2. It ships the whole anatomy.** Schema, doctrine, templates, and fixtures, in the layout that [the next section](#what-an-entry-ships) fixes. A schema with no doctrine is a shape with no reason. [Spec 2](https://github.com/headwater-ai/headwater/blob/main/docs/spec/02-taxonomy-model.md#purpose-is-declared-not-implied) refuses that inside a taxonomy, and this file refuses it around one.

**3. It is add-only over the base.** No `override` and no `remove`. An entry that needs either one does not fail admission on its own account. It is evidence that the base declared something that it should not have, and that finding goes to [13 — Open obligations](https://github.com/headwater-ai/headwater/blob/main/docs/spec/13-open-obligations.md). The entry then waits for the base change rather than working around it.

**4. It carries a worked instance corpus.** At least one real or realistic corpus, typed by the entry. A taxonomy that never met a document is a guess about a tradition. The failure mode is symmetry: kinds that balance on the page and that nobody files. The corpus is also what the engine inherits as a fixture the day that it exists.

**5. It declares its relationship to the invariant core.** The core requires the `state`, `freshness`, and `scent` facet roles, the `rationale` and `behavior` purposes, and a lifecycle-sensitive succession family ([spec 2](https://github.com/headwater-ai/headwater/blob/main/docs/spec/02-taxonomy-model.md#the-immutable-core)). An entry names which of its facets carry those roles, and which of its kinds serve those purposes.

A **partial entry** states that instead. A facet-only overlay such as Diátaxis composes onto a base that already satisfies the core, and it satisfies nothing by itself. The declaration is what separates a partial entry from an entry with a hole in it.

This criterion was first drafted around `rationale` alone. That inherited the omission that [Q3 corrected in its own leaning](https://github.com/headwater-ai/headwater/blob/main/docs/spec/09-decisions.md#q3--how-much-of-the-default-taxonomy-ships-in-the-box). A taxonomy with no behavior-serving kind has nowhere to state what the system does, so a code path resolves to nothing.

**6. Its address set is disjoint from every admitted entry.** Confluence holds for add-only overlays over disjoint addresses. Two entries that both add `kinds.guide` collide, and an adopter who selects both gets a kind assembled out of two traditions. The remedy is a rename in the later entry, or a declared dependency on the earlier one where both mean the same thing.

**The resolver's own check is narrower than this criterion, and a reviewer runs both.** `headwater-resolve` compares the *leaves* that two operations write rather than the paths they address, because two overlays reaching into one declaration without meeting do commute and this repository committed such a pair on the day it typed itself ([#50](https://github.com/headwater-ai/headwater/issues/50)). So it refuses two entries that state a purpose for `kinds.guide`, and it admits two entries that give `kinds.guide` different members. The second pair resolves, and it is still two traditions sharing a name. Admission refuses the name, and the release check refuses the contradiction.

**7. It resolves, and it declares its dependency closure.** Every reference in the entry resolves against the base plus the entries that it names in `requires`. The [meta-schema](https://github.com/headwater-ai/headwater/blob/main/docs/spec/02-taxonomy-model.md#the-meta-schema) lists what that means in full, and three of its rules catch most of it. Every concrete kind is reachable from a shelf. Every declared purpose is served by a concrete kind. Every relation endpoint names a declared kind or anchor kind.

The [first-run walkthrough](https://github.com/headwater-ai/headwater/blob/main/docs/evaluations/default-taxonomy-first-run.md#bundles-and-why-one-line-per-relation-does-not-generalize) measured why this is not free. A new relation is one line. A new kind drags a shelf, a purpose, an identifier scheme, and its edges behind it.

### Two consumption forms share one authored entry shape

The library serves two consumers without maintaining two copies of a taxonomy. A composer selects bundles and writes any local connection between them. A batteries-included consumer takes a flattened package that a publisher derives from a named assembly. [Spec 7](https://github.com/headwater-ai/headwater/blob/main/docs/spec/07-distribution-and-federation.md#an-assembly-has-two-consumption-forms) defines both forms.

An assembly is not a second library entry. The admitted bundles remain the authored sources for their schema, doctrine, templates, and fixtures. The assembly adds one recipe, an optional assembly overlay, assembly doctrine, and an interaction fixture. Its flattened package is generated release output.

The package boundary keeps the two forms apart. A composer takes `headwater/standard` and its bundles. A batteries-included consumer takes a package such as `headwater/starter` and selects no bundles. The flattened package can repeat addresses from the source bundles because the two packages never enter one resolution.

Assembly glue does not repair arbitrary bundle composition. [HW-OBL-0040](https://github.com/headwater-ai/headwater/blob/main/docs/obligations/0040-composition-between-two-library-entries-has-no-add-only-form.md) remains open because the source bundles still cannot declare the same connection together. The assembly owns only the connection for its named combination.

#### Assembly admission

An assembly is a distribution choice rather than a documentation tradition. Criterion 1 therefore does not apply to it. Eight criteria keep the assembly set finite and keep the flattened artifact derived.

**1. It names an outside adopter or a documented adopter population.** The evidence states why explicit bundle selection does not meet that adopter's need.

**2. It selects only admitted bundles.** The recipe pins one source package version and lists its complete bundle selection. It relies on no implicit reading of `requires`, which [Q40](https://github.com/headwater-ai/headwater/blob/main/docs/spec/09-decisions.md#q40--whether-extends-bundle-requires-and-an-overlays-taxonomy-key-are-a-mechanism-or-a-label) rules a label today.

**3. Its overlay contains assembly glue only.** Each operation connects declarations from two or more selected bundles. It restates no declaration that the base or a selected bundle owns.

**4. It carries a worked interaction corpus.** The corpus exercises the connections between the selected traditions. The selected bundles retain their own fixture corpora.

**5. Its doctrine explains the combination.** It links to each selected entry's doctrine and states only the choices that belong to the assembly.

**6. Its flattened package is generated.** No person maintains a second taxonomy source, a second bundle doctrine, or a second bundle template.

**7. Publication proves equivalence.** A fresh recipe resolution and the flattened source have the same canonical declarations after package identity and derivation metadata are excluded.

**8. A reader can identify the form from the manifest.** `distribution.form` is `flattened`, and `distribution.derived_from` names the recipe inputs. The assembly doctrine states that its publisher coordinates upgrades.

These criteria replace the refusal recorded for [#391](https://github.com/headwater-ai/headwater/issues/391). The refusal treated a flattened package as an independently authored entry. The assembly model admits the consumer experience and refuses the duplicated source.

### One rule of the base is not a criterion here

No relation in the base is `created_by: author`, because the claim under test is that unassisted human capture decays. A tradition that an entry models may genuinely carry an author-declared edge, so the rule does not transfer. The meta-schema already requires a `created_by` on every relation from a closed set. What an entry owes beyond that is one sentence of doctrine for each author-created edge, stating why nothing mechanical can propose it. `taxonomy audit` reports edge counts and staleness by creator, so the bet stays measurable rather than hidden.

## What an entry ships

One directory per entry, named for the bundle that it will publish as. Four parts, and three of them have a destination in the published package, so promotion is a move rather than a rewrite. The fourth stays here, and the row below says why.

| Part | Path in the draft | Where it lands when the entry is published |
|---|---|---|
| Schema | `bundle.yml` | `bundles/<name>/bundle.yml` ([spec 7](https://github.com/headwater-ai/headwater/blob/main/docs/spec/07-distribution-and-federation.md#publishing)) |
| Doctrine | `doctrine.md` | the `doctrine/` path, vendored to consumers |
| Templates | `templates/` | `bundles/<name>/templates/`, which a publish reads and carries |
| Fixtures | `fixtures/` | nowhere. A publish reads this directory and leaves it out of the artifact, because no consumer verb opens one. It stays here, as the corpus this publisher measures the bundle against ([spec 7](https://github.com/headwater-ai/headwater/blob/main/docs/spec/07-distribution-and-federation.md#publishing)) |

**`bundle.yml` is the overlay.** It names the bundle, the base version that it was written against, its `requires` closure, and its `add` operations. The per-file shape of a bundle is not specified anywhere yet, so a draft adopts this one and says so:

```yaml
bundle: design-spec
extends: headwater/standard@1.0.0
requires: []

add:
  purposes.<name>: {...}
  kinds.<name>: {...}
  shelves.<name>: {...}
```

**`doctrine.md` is the prose that a consumer vendors.** It states the tradition and cites the prior art of criterion 1. It explains the selection rather than restating the schema. It carries the core declaration of criterion 5, the author-edge sentences above, and every reading that the draft assumed where the specification is silent.

**`templates/` holds one template per concrete kind that the entry adds.** [Spec 3](https://github.com/headwater-ai/headwater/blob/main/docs/spec/03-authoring-and-lifecycle.md) owns what a template contains. An entry with a kind that has no template asks an author to derive a document shape from a schema.

**`fixtures/` holds the worked corpus of criterion 4**, and a `fixtures/README.md` that states what each document exercises and which findings it should raise. No runner reads these files yet. A fixture manifest in a format that nothing executes is a guess about a runner. The statement stays prose until an engine gives it a shape.

## What an assembly ships

An assembly draft lives at `taxonomy-source/headwater-standard/assemblies/<name>/`. It holds `assembly.yml`, an optional `overlay.yml`, `doctrine.md`, and `fixtures/`. The source package manifest names the directory under `contents.assemblies`.

`assembly.yml` names the flattened package, its version, the source package version, the complete bundle selection, and the optional overlay. `doctrine.md` explains the combination and the coordinated upgrade. `fixtures/` measures behavior that crosses the selected entries.

The flattened taxonomy, copied entry doctrine, copied templates, package manifest, and release record are publisher output. They do not live in the assembly draft. A publish reads the draft and the selected entries, proves equivalence, and writes the complete package into its output directory.

## Where drafts live, and why here

**Drafts live at `docs/taxonomies/<name>/`, and this file is the index.** The decision is recorded here because this file is the one that every entry author reads first.

The alternative was `docs/evaluations/`, on the argument that a draft is evidence. That argument is spent. Q2, Q3, and Q13 each closed on an evaluation of its own, so no draft is needed to settle a decision now.

An evaluation is a point-in-time record of how a question closed, and it is finished when the question is. A draft taxonomy is neither. It is a deliverable that later releases revise, and it is the engine's fixture corpus the day that there is an engine. A deliverable filed as evidence reads as spent the moment that its decision closes. That is wrong for an artifact which has to stay current.

`docs/spec/` is not a candidate. The specification states what Headwater is, and a taxonomy is content that runs on it.

## The rulings a draft works under

These are settled elsewhere. An entry works under them and reopens none of them.

**The format is Headwater's own dialect, in YAML.** [Q2](https://github.com/headwater-ai/headwater/blob/main/docs/spec/09-decisions.md#q2--schema-format) closed it. YAML 1.2 core schema is the concrete syntax, and the schema language, the reference sublanguage, the overlay language, and the meta-schema are Headwater's. JSON Schema is an emitted export and never the validator, because [spec 2](https://github.com/headwater-ai/headwater/blob/main/docs/spec/02-taxonomy-model.md) already carries references that no JSON Schema keyword resolves.

**The loader rules bind a draft now.** `no` stays the string `no`. Duplicate keys are an error. Anchors, aliases, and merge keys are forbidden in taxonomy sources. The `$`-reference is the sanctioned reuse mechanism, and an alias is a second one that no overlay can address. Scalar types come from the meta-schema and never from the YAML resolver.

**No second projection is required.** [Q13](https://github.com/headwater-ai/headwater/blob/main/docs/spec/09-decisions.md#q13--linkml-and-shacl-as-substrate) put LinkML last of six emitters, shipping only when a named external consumer asks, and it ruled that emitters never chain. Its evidence is already collected in the [LinkML worked example](https://github.com/headwater-ai/headwater/blob/main/docs/evaluations/linkml-worked-example.md). A hand-written LinkML or SHACL projection of an entry buys nothing that those two documents do not already hold.

**A draft that uses `$`-references works under a settled grammar.** [Spec 2](https://github.com/headwater-ai/headwater/blob/main/docs/spec/02-taxonomy-model.md#the--reference-sublanguage) defines the sublanguage, and `engine/crates/ref` parses it. An entry writes an address as a dotted path of segments, and it writes a reference with a `$` and a root from the closed set. One question stays open, and 13 carries it: what `optional` holds under the `package` root. An entry that reads optional package content therefore still records the reading that it assumed, in `doctrine.md`.

## Where a finding goes

[9 — The decision register](https://github.com/headwater-ai/headwater/blob/main/docs/spec/09-decisions.md) is a register of settled decisions, and it accepts no new questions. A finding from library work goes to [13 — Open obligations](https://github.com/headwater-ai/headwater/blob/main/docs/spec/13-open-obligations.md), or it reopens a closed decision explicitly and argues the change.

**An entry never edits the specification, and the finding is a separate change.** The two travel in opposite directions. An entry is content that runs on the specification, and a finding is a claim about the specification. To carry both in one pull request lets a library change rewrite the rules that admitted it.

### A finding has a tier, and the tier fixes the timing

**1. It contradicts a closed decision.** Stop, and reopen that decision explicitly before the entry merges. An entry built on a ruling that its own author believes is wrong carries the error into every entry that follows it.

**2. It sharpens a decision, or it names a gap that blocks nothing.** It goes to 13, in one change per entry, opened as soon as the entry merges.

**3. It records an assumption that the specification does not cover.** The same change carries it, grouped under one item. Each one is a place where the meta-schema has to speak eventually.

**One change per entry, and never one per epic.** A finding held to the end of a program of work is written months after the argument that produced it. Whoever writes it then has to reconstruct that argument first. The long form stays in the entry's doctrine, where the reasoning already sits. 13 carries the pointer, which is the form that its own entries already take.

**The epic holds the ledger.** Each entry posts its findings to the epic as a comment when it merges, with the tier of each one. That is what makes a review at the end of the epic possible. The review checks that every finding landed, and it is not the mechanism that lands them.

One item there is this library seen from the specification's side. **The bundle set** waits on a first adopter, because it is a guess about how adopters cluster and it is data in a package. Every entry admitted here is a revision of that guess, and the two must not drift.

## Admission, and what is admitted

An entry arrives as a pull request that adds one directory under this one. The reviewer checks the seven criteria above, and the entry is admitted when all seven hold. Criteria 3, 6, and 7 become mechanical the day that the resolver exists. Until then a reviewer reads the address list in `bundle.yml` against the entries already here.

Each admitted entry adds a line below, with its bundle name and the tradition that it models.

| Entry | Bundle | The tradition |
|---|---|---|
| [`design-spec`](design-spec/) | `design-spec` | The numbered specification series of IETF RFCs, academic papers, and software design documents |
| [`decision-record`](decision-record/) | `decision-record` | The architecture-decision-record tradition of Nygard, MADR and the tooling around them |
| [`diataxis`](diataxis/) | `diataxis` | Procida's four documentation modes, as one facet over the kinds a corpus already has |
| [`standards-spec`](standards-spec/) | `standards-spec` | The internal-standard ladder of MIL-STD-498, ISO/IEC/IEEE 29148 and the functional and technical specification convention |
| [`brd-prd`](brd-prd/) | `brd-prd` | The business-analysis and product-management requirements handoff of IIBA's BABOK Guide, the Pragmatic Marketing Framework and the PRD convention |

**One bundle of this library is not an admitted entry, and a composer still has to select it.** [`evidence-and-obligation`](evidence-and-obligation/) lives under this directory, publishes inside `headwater/standard`, and declares `kinds.evaluation`, `shelves.evaluations`, `purposes.evidence`, `purposes.obligation` and `regimes.voice.narrative`. [HW-DR-0044](https://github.com/headwater-ai/headwater/blob/main/docs/decisions/0044-q44-whether-bundles-decompose-into-capabilities-and-assemblies-compose-practices.md) moved those addresses out of `design-spec`, and `tools/library-index-fixtures.sh` holds this list against the bundle rather than against a reader. Both [`design-spec`](design-spec/) and [`decision-record`](decision-record/) declare `requires: [evidence-and-obligation]`, and the `headwater/starter` assembly selects it, so a reader who takes either entry takes this bundle with it. The table above has no row for it, and this paragraph is where a composer finds the name.

**Two criteria refuse the row, and neither refusal is a matter of wording.** Criterion 1 asks for a named tradition with citable prior art. This bundle models no tradition, and its [doctrine](evidence-and-obligation/doctrine.md) cites none. It is a capability split of `design-spec` rather than a practice that somebody else wrote down. The third column of the table above is headed "The tradition", and there is nothing true to put in that cell. Criterion 4 asks for a worked instance corpus, and the bundle's [finding 2](evidence-and-obligation/doctrine.md#findings) records that it carries none of its own. The `design-spec` and `decision-record` fixture corpora exercise `evaluation` from the two entries that require it.

**Criterion 5 has no reading for this bundle either, and the ruling that supplies one is open.** Its [doctrine](evidence-and-obligation/doctrine.md#the-core-declared) states that it serves neither `rationale` nor `behavior` and claims neither. `decision-record` and `brd-prd` stand in the same position, and [#520](https://github.com/headwater-ai/headwater/issues/520) carries the wording that all three wait on. HW-DR-0044 authorized the split and settled nothing about admission. No ruling covers a row for this bundle, and a reviewer who wrote one would waive criterion 1 rather than read it. Two questions belong to the owner. The first is whether a capability split may be admitted under a criterion it cannot meet. The second is whether an admitted entry may require a bundle that admission refuses ([#510](https://github.com/headwater-ai/headwater/issues/510)).

The design-spec entry carries six findings against the specification, which its [doctrine](design-spec/doctrine.md#findings) states in full. Two of them are worth reading before another entry is authored. A bundle cannot extend a list, so no entry may ship its own lifecycle ladder or its own projection. And the base's `specification` kind carries a section contract that a tradition with other headings cannot reuse.

**The `decision-record` entry is what tested criterion 6, and criterion 6 is the one that cost.** It serves the `obligation` purpose and declares nothing at that address, because no entry may declare an address that another entry already holds. So it declares `requires: [evidence-and-obligation]` for the `obligation` purpose alone, and it takes every address that bundle declares along with it. [HW-DR-0044](https://github.com/headwater-ai/headwater/blob/main/docs/decisions/0044-q44-whether-bundles-decompose-into-capabilities-and-assemblies-compose-practices.md) prices the alternative. A dependency on the whole of `design-spec` takes every kind and every shelf that bundle declares, and an ADR-keeping team need use none of them. Two more addresses were closed to the entry in the same way: an endpoint list of concrete kinds, and a kind's list of required facets. Its [doctrine](decision-record/doctrine.md#findings) states the three as one finding. Shared vocabulary belongs to the base, because an entry is the one place where nothing composes.

**The `decision-record` entry also stopped adding what the base already had.** The base declares `kinds.decision` with Nygard's three sections, so the entry adds no decision kind and writes into the base kind at the keys it leaves absent. That is the operation an `add` was specified for, and it is the remedy that design-spec's second finding asks for, applied from the other side.

**The `diataxis` entry is a partial entry, and it is what criterion 5's partial-entry paragraph was written for.** It declares one facet and attaches it to the base's abstract kind, and it declares no kind, no shelf, no purpose and no relation. So it satisfies no clause of the invariant core, it stands on a base that does, and its [doctrine](diataxis/doctrine.md#the-core-declared) states which declaration of the base supplies each one.

**Criterion 2 has no reading for a partial entry, and `diataxis` had to take one.** The anatomy asks for one template per concrete kind, and an entry that adds no kind has no kind for a template to shape. The `diataxis` entry ships no `templates/` directory and states the derivation in its doctrine. A later ruling should either write the reading into the anatomy above or refuse it.

**The `standards-spec` entry declares a whole tradition and requires nothing.** It adds three kinds, a purpose, a facet, two relations, two shelves and three identifier schemes, and it declares `requires: []`. Criterion 6 cost `decision-record` a dependency on a sibling entry for one address. `standards-spec` met no address that `design-spec`, `decision-record` or `diataxis` holds, because the purpose its standard kind serves is one nobody had claimed. A full entry is therefore not condemned to a dependency. Shared vocabulary in the base is what decides which of the two an entry gets.

**A rule of the specification also forced `standards-spec` into a direction.** Reading precedence for the governance family makes the source govern. A `conforms_to` edge from a specification to the standard that binds it would therefore derive the precedence backwards. The entry spells the edge `regulates`, from the standard, and its [doctrine](standards-spec/doctrine.md#the-relations-and-who-creates-each-edge) states the derivation. Its second finding is that the specification's own negative edge, `does_not_comply_with`, is spelled the other way inside the same family.

**The `standards-spec` entry measured something no reading of the schema shows.** Its fixture corpus plants a discriminator value that its heterogeneous shelf does not admit. Kind resolution stops, the census carries the row, and no rule instantiates over the document. A run holding that document alone reports 0 findings and a strict run exits 0. That is a property of every heterogeneous shelf in this library, and the [fixture README](https://github.com/headwater-ai/headwater/blob/main/docs/taxonomies/standards-spec/fixtures/README.md#planted-defect-4-reports-nothing-and-that-is-the-measurement) holds the run.

**The `diataxis` entry also measured a wall that was recorded as wider than it is.** [#6](https://github.com/headwater-ai/headwater/issues/6) concluded that a facet-only entry has to reach `kinds.<k>.facets.require` on another entry's kinds, and therefore that it waits on [HW-OBL-0040](https://github.com/headwater-ai/headwater/blob/main/docs/obligations/0040-composition-between-two-library-entries-has-no-add-only-form.md). The relevance canon that the conclusion rests on reads `require` and `optional` together, so an `optional` listing on the base's abstract kind satisfies it, and the entry ships without the blocked operation. HW-OBL-0040 stands: requiring a facet on a kind that another entry declared still has no add-only form, and nothing here settles that.

**The `brd-prd` entry is a full entry that serves neither core purpose.** It is also the entry that says criterion 5 has no reading for that. It adds two kinds, a purpose, a voice regime, a facet, a relation, a shelf and two identifier schemes. Every clause of the core is satisfied by the base. Criterion 5 asks an entry to name which of its kinds serve `rationale` and `behavior`, and none of this entry's kinds serves either. `decision-record` stands in the same position and says so plainly, without naming a gap. Its [doctrine](decision-record/doctrine.md#the-core-declared) records that `rationale` comes from the base `decision`, and that it declares no behavior-serving kind. So `decision-record` and `brd-prd` read the criterion the same way. The `brd-prd` [doctrine](brd-prd/doctrine.md#the-reading-criterion-5-does-not-supply) states the reading and asks for a ruling on the wording, which is what `diataxis` did to criterion 2. The refusal is the other half. The issue that asked for the entry offered a dependency on `standards-spec`. That would let a corpus hold the whole pipeline from a business need to a technical specification. The edge that pipeline needs reaches into another entry's endpoint list, which [HW-OBL-0040](https://github.com/headwater-ai/headwater/blob/main/docs/obligations/0040-composition-between-two-library-entries-has-no-add-only-form.md) records as having no add-only form. The alternative buys one edge for the whole of a sibling entry, which is the cost `decision-record` paid for one address. So `decision-record` records what a dependency costs and `brd-prd` records what refusing one costs, and the obligation holds a report from each side.

**The `brd-prd` entry declares a voice regime that the base refuses to supply.** The base carries `voice.declarative`, which forbids `future_intent`. A requirements document states what a product must do before it exists, so the base's regime reports the tradition itself. The entry declares `regimes.voice.prospective`, which keeps the other two categories, on the precedent that `regimes.voice.narrative` set. A probe measured the difference rather than asserting it. With `declarative` bound instead, the same seven fixture documents raise four more findings, in four of them, and the [fixture README](https://github.com/headwater-ai/headwater/blob/main/docs/taxonomies/brd-prd/fixtures/README.md#what-the-bases-voice-regime-would-have-reported) holds the run.

**No entry of this library serves the `procedure` purpose, and which tradition should supply one is a ruling rather than an omission.** Read across the base package and every bundle under this directory, no concrete kind declares that purpose and no bundle declares the purpose at all. The word is not a coinage waiting on a name. [Spec 7](https://github.com/headwater-ai/headwater/blob/main/docs/spec/07-distribution-and-federation.md#bundles-are-publisher-overlays-in-the-other-direction) names a `procedure` bundle in its illustration of a publisher's bundle set. [Spec 2](https://github.com/headwater-ai/headwater/blob/main/docs/spec/02-taxonomy-model.md#abstract-kinds) writes `kinds.playbook` with that purpose into an adopter overlay. So the purpose is settled and the entry is absent, and [#509](https://github.com/headwater-ai/headwater/issues/509) is the first outside corpus to ask for one.

**Criterion 1 is what holds the answer, and it admits three traditions that do not agree.** Procida's four modes as a kind set is the second live proposal of [#6](https://github.com/headwater-ai/headwater/issues/6). It contradicts the reading that the admitted [`diataxis`](diataxis/) entry argues, which is that a reader mode is a property of a document rather than a species of one. The operational runbook of the site-reliability tradition is citable and old enough to pass criterion 1 on its own. A corpus of agent skills or of setup guides is not made of runbooks. The work-instruction rung of the ISO 9001 document hierarchy would be an addition to [`standards-spec`](standards-spec/) rather than an entry, and that entry cites neither ISO 9001 nor a procedure of any kind. Criterion 4 refuses the second and the third on one ground. A corpus typed against a tradition it does not belong to files every document and describes none of them.

**An adopter waits for none of this, and that is the part worth stating plainly.** The seven criteria above bind an entry of this library. They bind no adopter overlay, which answers to no confluence check against a bundle that the consumer declaration does not select. This repository declared `purposes.procedure`, `kinds.tutorial` and `shelves.tutorials` in its own overlay for that reason, and the same route carried `probe`, `interface_contract`, `requirement` and `acceptance_criterion` before an entry existed for any of them. The [n8n review-rules fixture](https://github.com/headwater-ai/headwater/blob/main/docs/taxonomies/standards-spec/fixtures/n8n/README.md) is the worked instance of it. Two operations in an adopter overlay, a shelf at the path that corpus already uses, and seven real documents typed with no change to any entry. So a corpus of procedures is typable today by an overlay that declares the purpose and the kind. What the library does not supply is the tradition, and criterion 4 keeps that gap on this record until somebody rules on it.
