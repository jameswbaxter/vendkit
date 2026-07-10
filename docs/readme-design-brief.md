# Design brief: the VendKit README

You're designing the README for **VendKit**, an open-source Go tool. This brief tells you what VendKit is, who's reading, what the page must accomplish, and where you have creative room. Everything in "Must-be-accurate facts" ships as fact — don't soften, embellish, or paraphrase those. Everything else is yours to make beautiful.

---

## Context

VendKit vendors **curated slices of files across repositories, with provenance, integrity gates, and governed upgrades**.

The shape of the problem: an organization has files that must exist *identically* in many repos — AI/agent instructions, CI workflow templates, lint configs, design tokens. Today those files are copy-pasted and drift silently, or they're locked away behind package managers and submodules that don't fit "plain files in my tree."

VendKit's answer: a **publisher** repo declares which of its files form a distributable **slice**. **Consumer** repos vendor a pinned copy and get machinery that:

- **watches** for new publisher releases and raises an upgrade prompt,
- **syncs** the vendored copy forward as a reviewed pull request (adds/updates/removals — never silent),
- **gates** every consumer PR so a hand-edit of a vendored file cannot merge (drift protection),
- **verifies** structural migrations deterministically when a release reshapes content,
- **checks conformance** of a consumer against the publisher's adoption rules.

The mental model is **two lanes**:

- The **sync lane** opens one reviewed PR per upgrade.
- The **gate lane** runs on every consumer PR and fails on hand-edits or deletes of vendored files.
- A **composition invariant** binds them: the sync lane's output always passes the gate lane.

This two-lane picture is the load-bearing idea of the whole README. If a reader retains one thing, it's this.

## Audience

A busy platform/infra engineer or staff engineer who landed here from a search, a Slack link, or a coworker's PR. They will give you **30 seconds of skimming** before deciding whether to read properly. They are allergic to enterprise-speak, suspicious of "yet another sync tool," and they *have been burned by drift before* — that scar tissue is your way in. They want to know, in order: what is this, does it solve my problem, how fast can I try it, and why isn't this just submodules/Copybara/Renovate.

## Goals

1. **A skimmer understands the two-lane model in 30 seconds** — from the hero line, the diagram, and section headings alone.
2. **A motivated reader reaches a working quickstart without scrolling past philosophy.** How-to beats why-not. Usage above comparisons, always.
3. **The AI-instructions use case lands as the hero.** It's the most current, most relatable pain: one canonical CLAUDE.md / agent-rules / prompt-library set, vendored into dozens of repos, with governed upgrades and zero silent drift.
4. **The reader trusts it.** Warm tone, but the badges, the checksum manifest, the "never silent" guarantees, and the self-hosting note do credibility work.

## Structure & sections

Suggested order — react to it, improve it, but keep usage above philosophy:

1. **Hero.** Name, one-line what ("Vendor curated slices of files across repos — with provenance, integrity gates, and governed upgrades"), one short paragraph on why it matters (the drift problem, stated as a feeling every reader recognizes). Badge row directly under the title. Optional: a small tagline riff on "owned like first-party code, faithful to upstream."
2. **The two-lane model.** One simple diagram: publisher releases → sync lane (reviewed PR into consumer) and gate lane (every consumer PR, blocks hand-edits), with the composition invariant called out as a caption or annotation, not buried in prose. Three short bullets max alongside it.
3. **Quickstart.** The narrative arc is: *publisher declares a slice → consumer onboards in one command → PRs start flowing.* Show the install line, a minimal publisher declaration, the consumer onboarding command, and what the reader will see afterward (an upgrade PR appears; a hand-edit gets blocked). Keep it honest to the real CLI — where you don't know exact flags, mark placeholders clearly rather than inventing plausible-looking ones.
4. **What people vendor with it.** Four use cases, led by AI instructions:
   - **AI instructions across a fleet** (hero): one canonical set of agent instructions — CLAUDE.md, agent rules, prompt libraries, coding-standard docs, MCP config — vendored into dozens of product repos. Pinned, integrity-gated, upgraded by reviewed PR. Give this one a couple of sentences and maybe a small visual; the rest get one or two lines each.
   - **Shared CI/pipeline templates** — golden GitHub Actions / Azure Pipelines workflows with governed upgrades.
   - **Org-wide config & policy** — linter configs, .editorconfig, security policies, license headers, kept faithful across a fleet.
   - **Design-system tokens & schemas** — source-of-truth files that must stay byte-identical downstream but reviewed like first-party code.
5. **Why not X.** A tight table or four short rows — one line of "what it actually is" and one line of "why it's not this job" each:
   - *Package manager*: distributes artifacts into opaque stores; VendKit puts source files in your tree — reviewed, greppable, owned like first-party code, with a checksum manifest keeping them faithful to upstream.
   - *git submodule/subtree*: pins whole repos, not curated slices; no drift gate, no per-file provenance, no migration lifecycle.
   - *Copybara*: continuous code-motion; VendKit is release-oriented (immutable SemVer tags, migration payloads) and puts the consumer in control via PR.
   - *Renovate/Dependabot*: update dependency manifests; they don't vendor file trees or carry migrations.
6. **How it holds together.** A short trust section: checksum manifest, deterministic migration verification, conformance checks, the composition invariant, and the fact that VendKit is self-hosting — it vendors its own slice. Two or three sentences plus bullets; this is reassurance, not an architecture doc.
7. **Doc map / where next.** Links to deeper docs (publisher guide, consumer guide, migration authoring, CI setup for each backend), contributing, license.

## Tone & voice

- **Friendly, warm, human.** This is a helpful library, not an enterprise monolith. Write like a good engineer explaining something they're proud of, not like a vendor.
- **Concrete over abstract.** "A hand-edit of a vendored file cannot merge" beats "enforces integrity policies." Prefer verbs the reader can picture: watches, syncs, gates, blocks, opens a PR.
- **Confident, not breathless.** No "blazingly," no "revolutionary," no exclamation points doing the work adjectives should. Let the guarantees ("never silent," "always passes the gate") carry the weight.
- **One allowed moment of personality per section**, not per sentence. A dry aside about drift or copy-paste is on-brand; a pun-parade is not.
- Sentence-case headings. Short paragraphs. Skimmable bullets.

## Visual direction

- **Color:** friendly but trustworthy — think a warm, saturated accent (amber/teal/coral family) against calm neutrals, rather than corporate blue or hacker-neon. It should feel like a tool a small team loves, that a large org can also trust.
- **The two-lane diagram** is the centerpiece visual. Keep it simple enough to survive as SVG/ASCII in a README: two parallel horizontal lanes (sync above, gate below), publisher on the left, consumer PRs on the right, the invariant as a bridge or caption between lanes. If you do only one custom visual, it's this.
- **A second, smaller flow visual** for the upgrade lifecycle is nice-to-have: release tagged → watch fires → sync PR opens → review → merge → gate keeps it honest.
- **Badges**, in one row under the title: CI status, latest release, license (Apache-2.0), Go reference/`go install`. Standard shields.io style; don't invent exotic ones.
- **Code blocks** are part of the design: the install line and quickstart should look inviting — short commands, meaningful comments, no wall-of-flags.
- Design for GitHub's README constraints: works in light and dark mode, no external hosting dependencies you can avoid, degrades gracefully if images don't load (alt text that actually explains the diagram).

## Must-be-accurate facts

These ship as fact. Do not alter, hedge, or improvise around them.

- Name is **VendKit** — final, not provisional.
- Single **Go** implementation; one static binary with embedded scaffolds.
- Consumer-side integrity path is **dependency-free** (Go stdlib only).
- Core engine is **vendor-service-free**: git + filesystem only.
- CI backends: **GitHub Actions and Azure DevOps Pipelines as peer, first-class backends**, plus a fully manual mode (`ci: none`) with no CI at all. Don't present GitHub as primary and ADO as an afterthought.
- Releases are **immutable SemVer tags**; releases can carry **migration payloads**; migrations are **verified deterministically**.
- Sync is **never silent** — adds, updates, and removals arrive as a reviewed PR.
- The gate fails consumer PRs that hand-edit or delete vendored files.
- **Composition invariant:** the sync lane's output always passes the gate lane.
- A **checksum manifest** keeps vendored files faithful to upstream.
- License: **Apache-2.0**.
- Install: `go install github.com/jameswbaxter/vendkit/cmd/vendkit@latest`
- It is **implemented and self-hosting** — VendKit vendors its own slice. Not vaporware; don't use future tense for shipped behavior.

## What to cut

- **No philosophy before usage.** The "why not X" comparison comes after the quickstart and use cases, and it stays tight — four rows, no essays.
- **No feature-matrix sprawl.** If a section starts growing checkmark tables against every tool in the category, cut it back to the four comparisons above.
- **No architecture deep-dive.** Internals belong in linked docs; the README gets the trust bullets only.
- **No enterprise vocabulary.** Cut "governance framework," "compliance posture," "enterprise-grade," "solution." ("Governed upgrades" as a phrase is fine — it's the product's own term.)
- **No invented CLI output or fake flags presented as real.** Placeholder clearly or omit.
- **No roadmap section, no "coming soon."** The README describes what exists.
- **No mascot, no ASCII-art logo block** eating the first screenful. The hero is words, badges, and the diagram.

---

**Deliverable:** the complete README.md, plus the two-lane diagram (SVG preferred, with a fallback plan), ready to drop into the repo root.
