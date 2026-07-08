"""Scaffolder (onboarding spec §2): vendor + scaffold + report.

Run from a checkout of the publisher at the release being pinned. Never
performs trust-bootstrap acts; defers judgment to `vendkit conformance`.
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
    "github": [
        ("gate.yml.tmpl", ".github/workflows/vendkit-gate.yml", True),
        ("watch.yml.tmpl", ".github/workflows/vendkit-watch.yml", True),
        ("conformance.yml.tmpl", ".github/workflows/vendkit-conformance.yml", True),
        ("sync.yml.tmpl", ".github/workflows/__SLICE__-sync.yml", False),
    ],
    "ado": [
        ("gate.yml.tmpl", "azure-pipelines/vendkit-gate.yml", True),
        ("watch.yml.tmpl", "azure-pipelines/vendkit-watch.yml", True),
        ("conformance.yml.tmpl", "azure-pipelines/vendkit-conformance.yml", True),
        ("sync.yml.tmpl", "azure-pipelines/__SLICE__-sync.yml", False),
    ],
}

MANUAL_STEPS = """\
Manual steps (reported, never performed — onboarding spec §4; each maps to a
mandatory conformance rule):
  1. Grant the CI identity read access on {repo} (checkout + component resolution).
  2. Provision the PR-capable sync credential as secret {secret}
     (GitHub: PAT/App token, NOT GITHUB_TOKEN; ADO: PR-create-capable identity).
  3. Protect the default branch and make the gate a required check
     (GitHub: ruleset/branch protection; ADO: Build Validation policy).
  4. Record attestations in .vendkit/{slice}.yml:
       branch_protection_enabled, sync_credential_provisioned,
       pull_request_enforcement (ADO), required_check_enforced.
Then run `vendkit conformance --strict` — fully onboarded == it passes.
"""


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
    platform: str,
    version: str,
    profile: str | None = None,
    mode: str = "primary",
    base_branch: str = "main",
    pr_token_secret: str = "VENDKIT_PR_TOKEN",
) -> OnboardResult:
    if platform not in OUTPUTS:
        raise UsageError(f"unknown platform {platform!r}")
    if mode not in ("primary", "additive"):
        raise UsageError("mode must be 'primary' or 'additive'")
    if profile and decl.profiles and profile not in decl.profiles:
        raise UsageError(f"profile {profile!r} not declared by the publisher")
    result = OnboardResult()
    croot = Path(consumer_root)

    subs = {
        "SLICE": decl.slice_name,
        "SLICE_TITLE": decl.slice_title,
        "PUBLISHER_REPO": decl.publisher_repo,
        "PUBLISHER_PLATFORM": decl.publisher_platform,
        "VERSION": version,
        "BASE_BRANCH": base_branch,
        "PR_TOKEN_SECRET": pr_token_secret,
    }

    # 1. Slice config first (materialise reads the profile from it).
    cfg_path = croot / VENDKIT_DIR / f"{decl.slice_name}.yml"
    if cfg_path.exists():
        raise UsageError(f"slice already onboarded: {cfg_path}")
    pin_file = OUTPUTS[platform][3][1].replace("__SLICE__", decl.slice_name)
    # Every scaffolded workflow this slice pins gets advanced in lockstep by
    # the sync PR (sync spec §3 step 4). Additive slices own only their sync.
    pin_files = [pin_file] + (
        [o[1] for o in OUTPUTS[platform][:3]] if mode == "primary" else []
    )
    cfg_path.parent.mkdir(parents=True, exist_ok=True)
    cfg_path.write_text(
        _slice_config(decl, profile, pin_file, pin_files), encoding="utf-8")
    result.written.append(str(cfg_path))

    # 2. Vendor: empty manifest + reconcile-scope expansion (spec §2 phase 1).
    seed_empty_manifest(consumer_root, decl)
    report = materialise(publisher_root, consumer_root, decl, target=version,
                         apply=True, reconcile_scope=True)
    result.vendored = len(report.added)
    result.written.append(str(croot / VENDKIT_DIR / decl.manifest_name))

    # 3. Scaffold pipelines.
    for tmpl_name, out_rel, primary_only in OUTPUTS[platform]:
        if primary_only and mode == "additive":
            continue
        tmpl = (SCAFFOLD_ROOT / platform / tmpl_name).read_text(encoding="utf-8")
        out_path = croot / out_rel.replace("__SLICE__", decl.slice_name)
        if out_path.exists() and primary_only:
            continue  # additive repair: shared pipelines already exist
        out_path.parent.mkdir(parents=True, exist_ok=True)
        out_path.write_text(render(tmpl, subs), encoding="utf-8")
        result.written.append(str(out_path))

    result.manual_steps = MANUAL_STEPS.format(
        repo=decl.publisher_repo, secret=pr_token_secret, slice=decl.slice_name)
    return result


def _slice_config(decl: ExportDecl, profile: str | None, pin_file: str,
                  pin_files: list[str]) -> str:
    profile_line = f"profile: {profile}" if profile else "# profile: <bind to a publisher profile>"
    files_yaml = "\n".join(f"    - {f}" for f in pin_files)
    return f"""\
schema_version: 1
slice: {decl.slice_name}
publisher:
  platform: {decl.publisher_platform}
  repo: {decl.publisher_repo}
{profile_line}
pin:
  file: {pin_file}
  pattern: "ref: refs/tags/v"
  files:
{files_yaml}
watch:
  channel: stable
  handoff:
    kind: {"workitem" if decl.publisher_platform == "ado" else "issue"}
    dedup_key: vendkit-watch-{decl.slice_name}
    routing: {{}}
attestations:
  branch_protection_enabled: false
  sync_credential_provisioned: false
waivers: []
"""
