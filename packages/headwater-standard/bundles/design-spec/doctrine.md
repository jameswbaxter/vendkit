# The design-spec taxonomy

A numbered series of documents that state one design, read in order, with the argument split across the parts. Academic papers, IETF RFCs and software design documents all arrive at that shape, and they arrive at it separately. This entry models it.

The corpus of this repository is an unstated instance of the tradition. `docs/spec/` runs motivation, then vocabulary, then the core model, then process, assurance, interfaces, architecture, ecosystem, related work, and a decision register. Nothing in the repository declares that shape, so nothing can check it. That is the whole reason the tradition is worth typing first.

## The prior art

Four bodies of practice converge, and each one supplies a different part of the model.

**The IETF.** [RFC 7322](https://www.rfc-editor.org/info/rfc7322) fixes the anatomy of a published RFC. An abstract, a fixed section order, a security-considerations section that nobody may omit, and references split into normative and informative. [RFC 2026](https://www.rfc-editor.org/info/rfc2026) fixes the ladder that a document climbs, from Internet-Draft to Proposed Standard and onward. Two header fields of a published RFC do the work that this design calls succession. `Obsoletes:` names the document that this one replaces, and `Updates:` names the document whose effect this one changes without retiring it. Those are `supersedes` and `overrides` in the base package, under other names, and the mapping is exact.

**The research paper.** The IMRaD arc — introduction, method, results, discussion — is the most copied document structure in print. Its content is a claim, its evidence, and its position against prior work. The related-work section is the part that this tradition keeps and that a design document often drops.

**Software design practice.** Parnas and Clements, in *A Rational Design Process: How and Why to Fake It* (IEEE Transactions on Software Engineering, 1986), state the rule that the tradition rests on. The design process is never rational, and the document is written as though it were. That is on purpose: a reader needs the reconstruction rather than the history. Clements and colleagues, in *Documenting Software Architectures: Views and Beyond*, supply the same discipline for architecture. [ISO/IEC/IEEE 42010](https://www.iso.org/standard/74393.html) makes the viewpoint an object with a declared form.

**The decision register.** A design that records its decisions in one place, with the argument beside each, is a long-standing convention of standards work. It is not the architecture-decision-record tradition, which gives each decision a document of its own. The split matters here, and the [decision-record entry](../README.md) models the other side of it.

## What the tradition converges on

Four properties recur, and the schema below encodes each one.

**The series is ordered, and the number is part of the address.** A reader cites "spec 7" and reaches one document. An RFC number is never reissued and never renumbered. So the sequence is stable data on the document, and a renumber is a move that leaves the old address answering.

**One argument, several stages.** A part of the series is not a chapter that stands alone. It is a stage: motivation before model, model before process, process before assurance. A reader who starts in the middle is told what to read first.

**Decisions and debts are collected, not scattered.** The tradition puts settled decisions in a register and open work in another, rather than leaving both in the prose of whichever document raised them.

**Evidence is a document, and the instrument is committed with it.** A measurement that no reader can repeat is an assertion. So the evaluation that closed a question is filed, and a review's prompt sits beside its findings.

## The central question: kind, facet value, or template guidance

The canonical roles run from motivation and vocabulary through the core model, process, assurance, interfaces, architecture and related work, to the decisions. Each one could be a kind, or a value of one facet, or nothing at all in the schema. [Issue #3](https://github.com/headwater-ai/headwater/issues/3) asked which, and it said to decide by what the checks need to reference rather than by symmetry. That test answers cleanly, and it answers differently for different roles.

**A role becomes a kind when something must name it.** A relation endpoint, a participation expectation, or a purpose is such a thing. `review_prompt` and `review_record` are two kinds rather than one kind with a role facet. A relation runs between them, and an endpoint names a kind. `decision_register` is its own kind because its reader intent is `rationale` while the rest of the series serves `behavior`, and purpose is declared per kind.

**A role becomes a facet value when a check reads it and nothing links to it.** The three kinds that share `docs/spec/` need a discriminator, because placement cannot separate them and [spec 2](https://github.com/headwater-ai/headwater/blob/main/docs/spec/02-taxonomy-model.md#kind-resolution) resolves a heterogeneous shelf through one facet. So `doc_type` exists, and its values are kind names.

**Everything else is template guidance.** Nothing in the model references "the related-work section". No endpoint names it, no expectation windows it, and no purpose divides it from the section above it. Ten kinds for ten sections would multiply every obligation the model carries by ten and buy no check. The canonical arc therefore lives in `templates/design_spec.md`, where an author meets it, and nowhere in the schema.

The general rule that falls out: **purpose splits kinds, placement splits shelves, and prose structure splits neither.** A role that changes what a reader wants is a kind. A role that only changes what the page looks like is a template.

## The kinds

Six concrete kinds, each one under the base's `governed_document` abstract kind. Each inherits the four core-bearing facets, and each joins `supersedes`, `governs` and `traces_to` with no endpoint edit.

Each of the five that stayed here also requires `title`, which headwater/standard declares in the `name` role from 4.0.0. A numbered series is the tradition that needs a name most and states one least: the number orders the parts and names none of them, so a generated index and a site navigation both rendered `HW-SPEC-vision-and-scope` where "Vision and scope" belongs ([#123](https://github.com/headwater-ai/headwater/pull/123), [#427](https://github.com/headwater-ai/headwater/issues/427)). The facet stood in the decision-record entry until 4.0.0 and no `add` could reach these kinds from there, which is the finding that entry's doctrine carries as its own finding 1.

**The title is not the first heading, and on this shelf the two differ on purpose.** A part opens `# 0 — Vision and scope`, because a reader who opens the file wants the number. Its title is "Vision and scope", because `sequence` already carries the number and [spec 2](https://github.com/headwater-ai/headwater/blob/main/docs/spec/02-taxonomy-model.md#the-meta-schema) refuses a second copy of a value the front matter holds. A rendered label that read the heading would put the number in every sidebar entry and would take it from prose that no rule holds, which is the reading the owner refused when the alternative was put.

| Kind | Purpose | Shelf | Why it is not the kind above it |
|---|---|---|---|
| `design_spec` | `behavior` | `docs/spec/**` | It states what the system is |
| `decision_register` | `rationale` | `docs/spec/**` | It states why, and what each choice forecloses |
| `obligation_register` | `obligation` | `docs/spec/**` | It states what is owed, which is neither of the above |
| `evaluation` | `evidence` | `docs/evaluations/**` | It reports one measurement, and it is time-bound |
| `review_prompt` | `evidence` | `docs/reviews/**` | It is the instrument, and a relation names it |
| `review_record` | `evidence` | `docs/reviews/**` | It is one application of that instrument |

Two purposes are new, and the base has no kind that serves either. `evidence` answers "what closed this question". `obligation` answers "what is still owed". A corpus that keeps its debts in prose has neither, which is the state this repository was in before [13 — Open obligations](https://github.com/headwater-ai/headwater/blob/main/docs/spec/13-open-obligations.md) existed.

`evaluation` and `review_record` take the narrative voice regime, because both are time-bound by nature and [spec 3](https://github.com/headwater-ai/headwater/blob/main/docs/spec/03-authoring-and-lifecycle.md#voice) exempts that class. Everything else takes the declarative regime.

## The lifecycle ladder maps onto the base

The tradition has a status ladder, and the RFC form of it is the clearest. An Internet-Draft becomes a Proposed Standard, and a later document obsoletes it. The base declares four states, and the ladder maps onto them without a new value.

| The tradition | The base state | What it means here |
|---|---|---|
| Draft, under argument | `draft` | Written, and not yet settled |
| Settled, current | `current` | The reader may rely on it |
| Obsoleted by a successor | `superseded` | Retained, and the successor is named |
| Withdrawn, no successor | `deprecated` | Retained, and nothing replaced it |

**This entry adds no lifecycle state, and that is a constraint rather than a preference.** The base holds its states in one list under `vocabularies.lifecycle_state`, and a list cannot be extended by an `add` at a new key. An entry that wanted a fifth state would need `add_to`, whose standing inside the add-only rule nothing settles. The finding is recorded below, and the mapping above costs nothing in the meantime.

**A tombstone needs no declaration either.** [Issue #3](https://github.com/headwater-ai/headwater/issues/3) asked whether a redirect document is a kind of its own or a lifecycle state on the document it replaces. It is neither. `09-open-questions.md` in this repository is the worked case. The reciprocal half of `supersedes` is what makes it name its successors. What the tradition adds is an authoring convention: the retained body is rewritten as a map from each old anchor to its new home. A convention that the base already checks needs no new kind. A new kind would also collide with [rigidity](https://github.com/headwater-ai/headwater/blob/main/docs/spec/02-taxonomy-model.md#kinds-are-rigid-states-are-not): a document may not change kind while it stays the same document.

**Two mechanisms hold that file in place, and neither is `retain_terminal`.** Three live documents declare an edge to `HW-REG-open-questions`. Two registers declare `supersedes` and one review record declares `assesses`. Delete the file and each of those three becomes a `relation.target.unresolved` error, so `headwater check --strict` refuses the change and the commit gate refuses it with the same run. The file is also a `shelf_sections` projection, so `headwater generate --check` refuses a corpus that does not commit it. The reciprocal half of `supersedes` therefore keeps the file as well as makes it name its successors, which is the whole of what this entry needed from the base.

**The member is not what keeps it, for two reasons that are each enough.** No component read `retain_terminal` until `lifecycle.deletion.not_permitted`, which arrived in `headwater/standard` 3.2.0. And the file declares no value for the state facet, so it stands at no state and the rule that reads the member does not reach it either. This corpus reports that absence on every run, in the skip class that names `HW-REG-open-questions`.

The sentence that credited the member stood here from the commit that authored this entry, and nothing could have caught it: `.headwater/taxonomy.yml` excludes `docs/taxonomies/**` from the corpus, so no check of this engine reads a word of this file.

The sentence that credited the member stood here for two milestones, and nothing could have caught it: `.headwater/taxonomy.yml` excludes `docs/taxonomies/**` from the corpus, so no check of this engine reads a word of this file.

## Relations, and who creates each edge

Three relations, all in the `evidence` family, and none of them `created_by: author`.

- **`cites_evidence`**, from any of the three register-shelf kinds to an `evaluation`, with `cited_by` as its inverse and reciprocity required. A commit that adds an evaluation and edits a register in the same change is what proposes it, so the creator is a hook.
- **`applied_in`**, from a `review_prompt` to a `review_record`, reciprocity required. The scaffold that opens a review record from a prompt writes both halves, so the creator is a scaffold. This is the checkable form of a rule that this repository states in prose. A review whose instrument is unrecorded cannot be repeated against a later draft.
- **`assesses`**, from a `review_record` to whatever it reviewed. The same scaffold knows the targets, because a review names them before it runs.

Two participation expectations catch the absences that matter, and both are detective and windowed, as [spec 2](https://github.com/headwater-ai/headwater/blob/main/docs/spec/02-taxonomy-model.md#participation-expectations) requires. A `review_prompt` that acquires no record in ninety days is a plan rather than a review. An `evaluation` that no register cites in thirty days closed nothing.

**Sequence is not a relation, and the folding of `sequences` into participation does not say it is.** Spec 2 folded a declared chain into a windowed expectation on a kind. What it modeled was a genre system: proposal, then decision, then specification, then evidence. The order of a numbered series is a different thing. It is a static attribute of each document, and it never has a window. An author who reordered the series by editing edges would leave the paths disagreeing with the graph. So the order is the `sequence` facet, and the genre system of this tradition is the two expectations above.

## The core, declared

Criterion 5 of the [admission criteria](../README.md#admission-criteria) asks each entry to state its relationship to the invariant core. This entry is a full entry rather than a partial one, and it satisfies the core through the base.

- **`state`, `state_entered`, `freshness` and `scent`** are carried by the base facets `status`, `status_since`, `last_verified` and `summary`. Every kind here inherits all four from `governed_document`, and the entry declares no facet with an engine-significant role of its own.
- **`rationale`** is served by `decision_register`, beside the base's `decision`.
- **`behavior`** is served by `design_spec`, beside the base's `specification`.
- **Succession** stays lifecycle-sensitive and untouched. The entry adds no succession relation, because `supersedes` already carries the `Obsoletes:` semantics of the tradition.

Nothing here removes a satisfier, and an add-only overlay carries no operation that could.

## What this entry deliberately does not declare

**The provenance block, including the warrant.** [Issue #3](https://github.com/headwater-ai/headwater/issues/3) asks the schema to express the warrant, and the schema may not. [Spec 3](https://github.com/headwater-ai/headwater/blob/main/docs/spec/03-authoring-and-lifecycle.md#front-matter-is-the-contract) holds the provenance block outside the taxonomy's choices. Four engine rules turn on the value, and a taxonomy that could vary it would break them. So the warrant arrives with the engine, already required and already closed. The obligation that issue #3 attached to it survives in full, and it lands on [#4](https://github.com/headwater-ai/headwater/issues/4). The documents of this corpus have to carry the right value, and at least three of the four values appear in it.

**An identifier scheme.** [Spec 3](https://github.com/headwater-ai/headwater/blob/main/docs/spec/03-authoring-and-lifecycle.md#identifiers) reserves identifiers for decisions, requirements, acceptance criteria, controls and obligations. A numbered document in a series is none of those, and its number already lives in its path. The things in this tradition that do need identifiers are the entries inside the two registers, and the model cannot reach them. That finding is below.

**A language regime.** The controlled profile of this repository is a choice of this repository, not of the tradition. RFCs are not written in Simplified Technical English. The base declares `controlled: none`, and an adopter that wants a profile binds it in its own overlay. That is where [the first-run walkthrough](https://github.com/headwater-ai/headwater/blob/main/docs/evaluations/default-taxonomy-first-run.md#the-starter-kit-is-a-selection-not-a-third-option) put the same decision for the starter kit.

**A projection.** The series index that `sequence` deserves cannot be added, for the reason the lifecycle ladder could not be extended: `projections` is a list. The finding below covers both.

## What this draft assumed

Three places where the specification is silent and the draft had to pick. Each one is an assumption rather than a proposal, and each is listed in the findings below.

- **`volatility` has no declared value set.** [Spec 2](https://github.com/headwater-ai/headwater/blob/main/docs/spec/02-taxonomy-model.md#facet-acceptance-tests) requires the facet to declare one, and it names only the `mutable` case. The draft writes `stable` for both new facets, meaning that a change is a rename rather than an edit.
- **Value guidance has no declared syntax.** The ascertainability canon requires guidance on every enum value. The draft writes a `guidance` map under the facet, keyed by value.
- **A bundle file has no declared shape.** [Spec 7](https://github.com/headwater-ai/headwater/blob/main/docs/spec/07-distribution-and-federation.md#publishing) declares bundles in the package manifest and fixes nothing about the file that holds one. The draft uses the header that the [library index](../README.md#what-an-entry-ships) settled: `bundle`, `extends`, `requires`, and one `add` block.

**No `$`-reference appears in this draft, and that was a small datum for the grammar that was open when it was written.** The sublanguage has three uses: a vocabulary reference, a reference to optional package content, and an overlay address. A bundle that declares its own vocabulary inline and adds at fresh keys needs none of the three. So the first pressure on that grammar comes from an entry that reuses a base vocabulary, not from this one. [Spec 2](https://github.com/headwater-ai/headwater/blob/main/docs/spec/02-taxonomy-model.md#the--reference-sublanguage) has since defined the sublanguage, and every `add` address in this draft parses under it.

## Worked instances

Criterion 4 asks for at least one real or realistic corpus that the entry types. This entry carries two.

**[Beacon](https://github.com/headwater-ai/headwater/blob/main/docs/taxonomies/design-spec/fixtures/README.md), a realistic corpus.** Documents on the `spec_series` and `reviews` shelves, with planted defects that the fixture README names one by one. Beacon is invented, and four entries of this library share it, so a reader can hold four traditions over one project.

**[n8n](https://github.com/headwater-ai/headwater/blob/main/docs/taxonomies/design-spec/fixtures/n8n/README.md), a real one.** Four architecture documents from `n8n-io/n8n`, pinned at `b0550cb3cb4d1752546a69056c55eccfb9111a12` on `master`, copied with a front-matter block added and every body byte-identical. It is the first corpus this library has met that keeps its governed prose beside the code it governs, one document per package, with no collected documentation root. [HW-EVAL-n8n-worked-example](https://github.com/headwater-ai/headwater/blob/main/docs/evaluations/n8n-worked-example.md) is the report, and three of its results belong here.

**This entry as it ships types none of that corpus, and the strict run over it exits 0.** The one shelf is `spec_series` at `docs/spec/**`, and no document of a monorepo is under `docs/`. Four real governing documents go untyped, no rule instantiates, and nothing but the census says so. The shelf that reaches them has to be rooted at `packages`, has to fix one path segment out of 27,688 files, and has to be heterogeneous with a single admitted kind. That last one is forced. `design_spec` requires `doc_type`, and a homogeneous shelf refuses a document that restates the kind its placement already states, so the kind cannot sit cleanly on a homogeneous shelf at all.

**`sequence` has no value in a corpus that files one design specification per package.** The facet is required by three kinds of this entry and it means a position in a numbered series. n8n's architecture documents are one per package, and a package is not a position. Every one of the four reports `facet.required.missing`, and no number was invented to stop it. The assumption the entry made is that a design specification belongs to a series, and the first real corpus it met does not.

**`title` has no value in a corpus that nobody here may edit, and that is the price of requiring it.** headwater/standard 4.0.0 requires the facet on all five kinds of this entry, for the reason the facet section above states. Each of the four n8n documents opens with a first heading, and a heading is body text that no facet reads. So each one reports `facet.required.missing` a second time, and the remedy would be to write four titles into somebody else's documents. The fixture refuses that, because a worked instance that this repository edits until it passes is evidence about this repository.

## Findings

These go to [13 — Open obligations](https://github.com/headwater-ai/headwater/blob/main/docs/spec/13-open-obligations.md), on the route that the [library index](../README.md#where-a-finding-goes) fixes. Nothing here reopens a closed decision.

**1. A bundle cannot extend a list, so two ordinary needs have no add-only form.** The base holds `vocabularies.lifecycle_state`, `projections` and `core.requires` as lists. An `add` introduces a key that does not exist. Every element of a list sits inside an address that the overlay language reaches only as a whole. [Spec 2](https://github.com/headwater-ai/headwater/blob/main/docs/spec/02-taxonomy-model.md#customization-by-composition) supplies `add_to` for exactly this, and the add-only rule for bundles is stated over `add` alone. Two readings are open. Either a bundle may use `add_to`, and the confluence check then treats repeated `add_to` at one address as commuting under set semantics. Or it may not, and no tradition ever ships its own lifecycle ladder or its own index projection. This entry hit the limit twice while staying admissible, which suggests the first reading with the check stated explicitly.

**2. The base's `specification` kind carries a lexical section contract, and a tradition with other headings cannot reuse it.** `sections: {require: [Scope, Behavior]}` is a commitment to two heading names in a package whose core is [semantic and never lexical](https://github.com/headwater-ai/headwater/blob/main/docs/spec/07-distribution-and-federation.md#the-invariant-core). This entry therefore adds `design_spec` beside it rather than reusing it. An adopter of both is then left with a base kind and a base shelf that nothing fills. [The first-run walkthrough](https://github.com/headwater-ai/headwater/blob/main/docs/evaluations/default-taxonomy-first-run.md#the-starter-kit-is-a-selection-not-a-third-option) already made the same finding about the controlled-language profile. An opinionated default belongs to the starter kit, and the base holds what the core requires.

**3. Identity is per document, and this tradition identifies things inside documents.** A decision in the register was cited as Q13 and an obligation was cited by its bullet. Neither was a document, so neither reached the identifier index, and no edge could name one. Both halves have since converted, and the [decision-record entry](../decision-record/doctrine.md#what-a-migration-inherits) records what each pass cost. That is why `cites_evidence` runs at document grain and cannot say which entry an evaluation closed. [13 — Open obligations](https://github.com/headwater-ai/headwater/blob/main/docs/spec/13-open-obligations.md#a-human-maintains-this-list-by-hand) already records the same defect from the inside, and it prescribes ordinary corpus work as the remedy. This entry adds the outside view. The remedy converts each entry into a document, which is the [decision-record tradition](https://github.com/headwater-ai/headwater/issues/5). So the two entries are a migration of each other rather than neighbors.

**4. A participation expectation names one target kind.** `evidence-cited` expects a citation from a `decision_register`, and an evaluation cited only by an `obligation_register` still reports as absent. The tradition wants "cited by any register kind". An abstract kind over the two registers would express it, at the cost of a kind that exists to be a target.

**5. Two meta-schema surfaces have a required declaration and no stated form.** `volatility` requires a value that no value set defines. Value guidance is required by the ascertainability canon with no syntax. Both are cheap to fix and both are guesses in this draft today.

**6. The `narrative` voice regime is a known collision with the bundle set.** [Spec 7](https://github.com/headwater-ai/headwater/blob/main/docs/spec/07-distribution-and-federation.md#bundles-are-publisher-overlays-in-the-other-direction) sketches a `proposals` bundle that adds the same regime at the same address. Criterion 6 of the admission criteria makes that a resolution error for an adopter who selects both. The remedy is a declared dependency, once the bundle set is authored. The collision is also a first piece of data for [the item that waits on an adopter](https://github.com/headwater-ai/headwater/blob/main/docs/spec/13-open-obligations.md#what-waits-on-a-first-adopter).

## What HW-DR-0044 moved out, and why the rest could not follow

[HW-DR-0044](https://github.com/headwater-ai/headwater/blob/main/docs/decisions/0044-q44-whether-bundles-decompose-into-capabilities-and-assemblies-compose-practices.md) found finding 1 below a real cost rather than a tolerable one: a team that keeps ADRs and nothing else still resolved six kinds and three shelves of this tradition, for one purpose and one kind it ever touched. The remedy was not the four-way split its own Context section asked the investigation to test. Tracing what `decision-record` actually reaches — `purposes.obligation` and `kinds.evaluation`, and nothing else here — found that only `evaluation`, its two purposes and the `narrative` regime could leave without creating the cycle this section explains.

**`evaluation` left, because nothing else in this entry names it as an endpoint.** `cites_evidence.to` is its only reference from the kinds that stayed, and a relation may name a kind it does not own as long as its bundle requires the kind's owner. This entry now requires [evidence-and-obligation](../evidence-and-obligation/doctrine.md).

**`obligation_register`, `review_prompt` and `review_record` stayed, and each for the same reason: a required facet.** All three declare `facets.require` reaching into `doc_type` or `sequence`, and both facets stayed here, discriminating `spec_series` and `reviews`. Moving any of the three would have needed evidence-and-obligation to require this entry back for that facet reference, which cycles against the edge this entry now carries onto it: no bundle may sit on both ends of `requires`, and the resolver has no way to break the tie. `purposes.obligation` moved without the kind that most visibly serves it, because a purpose is a reference by name and `obligation_register` reaches it the same way `cites_evidence` reaches `evaluation`: through the `requires` edge, not through co-location.

**The `evidence-cited` participation expectation did not survive the move, and finding 4 already said why it was the first casualty.** The expectation named `decision_register` as its `to_kind`, a kind that stayed here. Declaring it on evidence-and-obligation's copy of `evaluation` would have needed the same reverse edge that `obligation_register` would have. Finding 4 already recorded that the expectation could only ever be satisfied by a `decision_register`, so an evaluation cited only by an `obligation_register` reported as absent under the very declaration this section retires. The [decision-record fixture](https://github.com/headwater-ai/headwater/blob/main/docs/taxonomies/decision-record/fixtures/README.md#what-a-run-reports) measured the same narrowness as unfixable for an adopter who keeps no register at all. The split does not close finding 4. It removes the one declaration finding 4 had already found too narrow to keep, and the general remedy — an abstract kind over the two registers — is still open.

## What #4 inherits

The dogfooding issue types this repository against this entry. Three things are ready for it, and one is not.

Ready: the three shelves match the tree as it stands, and the `doc_type` values cover every document under `docs/spec/`. The lifecycle mapping above says which state each document is in. Not ready: `docs/reviews/` holds prompts and findings in one directory, with no naming convention that a shelf can read. So #4 either supplies the `doc_type` facet on each file, or it renames them. The entry takes the first route, because a rename is corpus churn that a draft taxonomy has not yet earned.
