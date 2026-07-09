"""Scaffolder (onboarding spec §2): vendor + scaffold + report.

Run from a checkout of the publisher at the release being pinned. Never
performs trust-bootstrap acts; defers judgment to `vendkit conformance`.

Two explicit axes (DR-0015): `ci` picks the template pack (github-actions |
azure-pipelines | none — 'none' scaffolds no pipelines at all: fully manual
orchestration, the manifest's provenance is the pin). `scm` drives ownership
handling (CODEOWNERS is GitHub-only; Azure Repos uses a required-reviewers
policy, which lands on the manual-steps checklist) and the default handler
commands written to the slice config.
"""

from __future__ import annotations

import re
from dataclasses import dataclass, field
from pathlib import Path

from .declaration import ExportDecl
from .manifest import VENDKIT_DIR
from .materialise import materialise, seed_empty_manifest
from .util import UsageError

SCAFFOLD_ROOT = Path(__file__).resolve().parent.parent.parent / "scaffold"
_PLACEHOLDER = re.compile(r"__[A-Z][A-Z0-9_]*__")

# (template, consumer-relative output, primary-mode-only)
OUTPUTS = {
    "github-actions": [
        ("gate.yml.tmpl", ".github/workflows/vendkit-gate.yml", True),
        ("watch.yml.tmpl", ".github/workflows/vendkit-watch.yml", True),
        ("conformance.yml.tmpl", ".github/workflows/vendkit-conformance.yml", True),
        ("sync.yml.tmpl", ".github/workflows/__SLICE__-sync.yml", False),
    ],
    "azure-pipelines": [
        ("gate.yml.tmpl", "azure-pipelines/vendkit-gate.yml", True),
        ("watch.yml.tmpl", "azure-pipelines/vendkit-watch.yml", True),
        ("conformance.yml.tmpl", "azure-pipelines/vendkit-conformance.yml", True),
        ("sync.yml.tmpl", "azure-pipelines/__SLICE__-sync.yml", False),
    ],
    "none": [],
}

# Consumer-scm → reference handler module (handler-protocol spec §6). Just a
# default string in a config file — swap for any protocol-honouring command.
HANDLER_MODULES = {
    "github": "vendkit.handlers.github",
    "azure-repos": "vendkit.handlers.ado",
}


@dataclass
class OnboardResult:
    written: list[str] = field(default_factory=list)
    vendored: int = 0
    manual_steps: str = ""


def render(template: str, subs: dict[str, str]) -> str:
    out = template
    for key, value in subs.items():
        out = out.replace(f"__{key}__", value)
    left = _PLACEHOLDER.search(out)
    if left:
        # Fail loudly on any unresolved placeholder (onboarding spec §2).
        raise UsageError(f"unresolved scaffold placeholder: {left.group(0)}")
    return out


def onboard(
    publisher_root: str,
    consumer_root: str,
    decl: ExportDecl,
    ci: str,
    scm: str,
    version: str,
    profile: str | None = None,
    mode: str = "primary",
    base_branch: str = "main",
    pr_token_secret: str = "VENDKIT_PR_TOKEN",
    codeowners: str | None = None,
) -> OnboardResult:
    if ci not in OUTPUTS:
        raise UsageError(f"unknown ci {ci!r}")
    if scm not in HANDLER_MODULES:
        raise UsageError(f"unknown scm {scm!r}")
    if mode not in ("primary", "additive"):
        raise UsageError("mode must be 'primary' or 'additive'")
    if profile and decl.profiles and profile not in decl.profiles:
        raise UsageError(f"profile {profile!r} not declared by the publisher")
    if codeowners and scm != "github":
        raise UsageError(
            "--codeowners is GitHub-only: Azure Repos does not honour "
            "CODEOWNERS — add a required-reviewers branch policy covering "
            ".vendkit/** instead (it is on the manual-steps checklist)")
    result = OnboardResult()
    croot = Path(consumer_root)

    subs = {
        "SLICE": decl.slice_name,
        "SLICE_TITLE": decl.slice_title,
        "PUBLISHER_REPO": decl.publisher_repo,
        "PUBLISHER_SCM": decl.publisher_scm,
        "VERSION": version,
        "BASE_BRANCH": base_branch,
        "PR_TOKEN_SECRET": pr_token_secret,
    }

    # 1. Slice config first (materialise reads the profile from it).
    cfg_path = croot / VENDKIT_DIR / f"{decl.slice_name}.yml"
    if cfg_path.exists():
        raise UsageError(f"slice already onboarded: {cfg_path}")
    outputs = OUTPUTS[ci]
    if ci == "none":
        pin_files: list[str] = []
    else:
        pin_file = outputs[3][1].replace("__SLICE__", decl.slice_name)
        # Every scaffolded workflow this slice pins gets advanced in lockstep
        # by the sync PR (sync spec §3 step 4). Additive slices own only
        # their sync pipeline.
        pin_files = [pin_file] + (
            [o[1] for o in outputs[:3]] if mode == "primary" else []
        )
    cfg_path.parent.mkdir(parents=True, exist_ok=True)
    cfg_path.write_text(
        _slice_config(decl, profile, ci, scm, pin_files), encoding="utf-8")
    result.written.append(str(cfg_path))

    # 2. Vendor: empty manifest + reconcile-scope expansion (spec §2 phase 1).
    seed_empty_manifest(consumer_root, decl)
    report = materialise(publisher_root, consumer_root, decl, target=version,
                         apply=True, reconcile_scope=True)
    result.vendored = len(report.added)
    result.written.append(str(croot / VENDKIT_DIR / decl.manifest_name))

    # 3. Scaffold pipelines (none under ci: none).
    for tmpl_name, out_rel, primary_only in outputs:
        if primary_only and mode == "additive":
            continue
        tmpl = (SCAFFOLD_ROOT / ci / tmpl_name).read_text(encoding="utf-8")
        out_path = croot / out_rel.replace("__SLICE__", decl.slice_name)
        if out_path.exists() and primary_only:
            continue  # additive repair: shared pipelines already exist
        out_path.parent.mkdir(parents=True, exist_ok=True)
        out_path.write_text(render(tmpl, subs), encoding="utf-8")
        result.written.append(str(out_path))

    # 4. Ownership — opt-in, SCM-axis (DR-0015).
    if codeowners:
        co_path = croot / "CODEOWNERS"
        stanza = f"/{VENDKIT_DIR}/ {codeowners}\n"
        existing = co_path.read_text(encoding="utf-8") if co_path.is_file() else ""
        if stanza not in existing:
            co_path.write_text(existing + stanza, encoding="utf-8")
            result.written.append(str(co_path))

    result.manual_steps = _manual_steps(decl, ci, scm, pr_token_secret)
    return result


def _manual_steps(decl: ExportDecl, ci: str, scm: str, secret: str) -> str:
    """The irreducible checklist (onboarding spec §4). Each step maps to a
    conformance rule, so 'fully onboarded' == `conformance --strict` passes."""
    steps = [
        f"Grant the CI identity read access on {decl.publisher_repo} "
        "(checkout + engine resolution).",
    ]
    if ci != "none":
        steps.append(
            f"Provision the PR-capable sync credential as secret {secret} "
            "for the PR handler (GitHub: PAT/App token, NOT GITHUB_TOKEN; "
            "Azure DevOps: PR-create-capable identity).")
    steps.append(
        "Protect the default branch and make the gate a required check "
        "(GitHub: ruleset/branch protection; Azure Repos: Build Validation "
        "policy)." if ci != "none" else
        "Protect the default branch. NOTE: with ci 'none' there is no "
        "PR-time gate — you forfeit automated drift enforcement and must "
        "run `vendkit gate --strict` yourself.")
    if scm == "azure-repos":
        steps.append(
            "Add a required-reviewers branch policy covering .vendkit/** "
            "and the vendored namespaces (Azure Repos does not honour "
            "CODEOWNERS); record attestation required_reviewers_policy.")
    steps.append(
        f"Record attestations in .vendkit/{decl.slice_name}.yml: "
        "branch_protection_enabled, sync_credential_provisioned, "
        "pull_request_enforcement (azure-pipelines), required_check_enforced.")
    lines = [
        "Manual steps (reported, never performed — onboarding spec §4; each "
        "maps to a conformance rule):",
    ]
    lines += [f"  {i}. {s}" for i, s in enumerate(steps, 1)]
    lines.append(
        "Then run `vendkit conformance --strict` — fully onboarded == it passes.")
    return "\n".join(lines) + "\n"


def _slice_config(decl: ExportDecl, profile: str | None, ci: str, scm: str,
                  pin_files: list[str]) -> str:
    profile_line = f"profile: {profile}" if profile else "# profile: <bind to a publisher profile>"
    if pin_files:
        files_yaml = "\n".join(f"    - {f}" for f in pin_files)
        pin_block = f"""\
pin:
  # First entry is the authoritative read source; the sync PR advances the
  # matching reference line in every listed file, in lockstep.
  pattern: "ref: refs/tags/v"
  files:
{files_yaml}
"""
    else:
        pin_block = """\
# ci is 'none': no pin lines — the manifest's source.release is the pin.
"""
    handler_module = HANDLER_MODULES[scm]
    if ci == "none":
        handlers_block = """\
# handlers: deliberately unwired (fully manual). Wire any executable that
# honours the handler protocol, e.g.:
#   pr: {exec: [python3, -m, %s]}
""" % handler_module
    else:
        handlers_block = f"""\
handlers:
  # Delivery handlers (handler protocol, DR-0014): any executable honouring
  # the protocol can replace these reference commands.
  pr:
    exec: [python3, -m, {handler_module}]
  handoff:
    exec: [python3, -m, {handler_module}]
    dedup_key: vendkit-watch-{decl.slice_name}
  fact-verify:
    exec: [python3, -m, {handler_module}]
"""
    return f"""\
schema_version: 1
slice: {decl.slice_name}
publisher:
  scm: {decl.publisher_scm}
  repo: {decl.publisher_repo}
scm: {scm}
ci: {ci}
{profile_line}
{pin_block}watch:
  channel: stable
{handlers_block}seeds:
  # Seeded files are scaffolded once, then yours (DR-0013). 'informational'
  # adds a note to sync PRs when an upstream template later changes.
  notes: informational
attestations:
  branch_protection_enabled: false
  sync_credential_provisioned: false
waivers: []
"""
