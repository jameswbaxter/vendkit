"""Release cutting (releases-and-versioning spec §3, DR-0005)."""

from __future__ import annotations

import subprocess
from dataclasses import dataclass
from pathlib import Path

from . import migrations, versions
from .declaration import ExportDecl
from .manifest import build_publisher_manifest, load_manifest, manifests_equal
from .util import Refusal, UsageError, VendkitError, run_git


@dataclass
class CutResult:
    version: str
    previous: str | None
    added: int
    removed: int


def _remote_release_tags(root: str) -> list[str]:
    proc = subprocess.run(
        ["git", "ls-remote", "--tags", "--refs", "origin"],
        cwd=root, capture_output=True, text=True,
    )
    if proc.returncode != 0:
        # Hard error: never compute a version from an unknown baseline.
        raise VendkitError(
            f"cannot list remote tags: {proc.stderr.strip()}"
        )
    return [
        line.rpartition("refs/tags/")[2]
        for line in proc.stdout.splitlines() if "refs/tags/" in line
    ]


def _tag_exists(root: str, tag: str) -> bool:
    local = subprocess.run(
        ["git", "rev-parse", "-q", "--verify", f"refs/tags/{tag}"],
        cwd=root, capture_output=True,
    ).returncode == 0
    return local or tag in _remote_release_tags(root)


def _surface_delta(root: str, decl: ExportDecl, previous: str) -> tuple[set, set]:
    """(added paths, removed non-seed paths) vs the previous release's
    manifest. Seed-entry removals are excluded: retiring a template leaves
    every consumer's file intact (DR-0013), so it never demands a MAJOR or a
    migration payload. Seed additions still count (they imply MINOR)."""
    proc = subprocess.run(
        ["git", "show", f"{previous}:{decl.manifest_name}"],
        cwd=root, capture_output=True, text=True,
    )
    if proc.returncode != 0:
        return set(), set()  # previous release predates a manifest: no gate
    import json
    prev_entries = json.loads(proc.stdout).get("entries", [])
    prev_paths = {e["path"] for e in prev_entries}
    prev_nonseed = {e["path"] for e in prev_entries if not e.get("seed")}
    curr_paths = {e["path"] for e in build_publisher_manifest(decl, root)["entries"]}
    return curr_paths - prev_paths, prev_nonseed - curr_paths


def cut(
    root: str,
    decl: ExportDecl,
    bump_kind: str | None = None,
    explicit_version: str | None = None,
    summary: str = "",
    no_migrations_needed: bool = False,
    dry_run: bool = False,
) -> CutResult:
    # 1. Freshness pre-gate: a release never ships a stale manifest.
    manifest_path = Path(root) / decl.manifest_name
    if not manifest_path.is_file():
        raise UsageError(f"missing manifest {decl.manifest_name} — run generate")
    if not manifests_equal(
        load_manifest(str(manifest_path)), build_publisher_manifest(decl, root)
    ):
        raise Refusal("stale-manifest",
                      "committed manifest is stale — run generate and commit")

    # 2. Baseline and target.
    latest = versions.latest(_remote_release_tags(root), channel="rc")
    if explicit_version:
        target = explicit_version
        versions.require(target)
    else:
        if not bump_kind:
            raise UsageError("either --bump or --version is required")
        target = versions.bump(latest or "v0.0.0", bump_kind)
    if latest and not versions.require(target) > versions.require(latest):
        raise Refusal("not-newer", f"{target} is not newer than latest {latest}")
    if _tag_exists(root, target):
        raise Refusal("tag-exists", f"tag {target} already exists (INV-5)")

    # 3. Surface-delta bump enforcement (releases spec §2).
    added: set = set()
    removed: set = set()
    if latest:
        added, removed = _surface_delta(root, decl, latest)
        implied = "major" if removed else ("minor" if added else "patch")
        order = {"patch": 0, "minor": 1, "major": 2}
        actual = versions.classify_bump(latest, target)
        if order[actual] < order[implied]:
            raise Refusal(
                "bump-too-small",
                f"surface delta (+{len(added)}/-{len(removed)}) implies at "
                f"least a {implied} bump; {latest} -> {target} is {actual}",
            )
        # 4. Migration pre-gate: removals demand a payload or an override.
        if removed and not no_migrations_needed:
            entries = migrations.load_entries(root)
            if not any(e["applies_from"] == target for e in entries):
                raise Refusal(
                    "migration-missing",
                    f"release removes {len(removed)} exported file(s) but "
                    f"ships no migrations/ entry with applies_from: {target} "
                    "(pass --no-migrations-needed to record an override)",
                )

    # 5. Annotated tag + push (remote ref rejection = serialisation point).
    if not dry_run:
        message = "\n".join(filter(None, [
            f"{decl.slice_name} {target}",
            summary,
            f"surface-delta: +{len(added)} -{len(removed)}",
            "no-migrations-needed" if no_migrations_needed else "",
        ]))
        run_git(["tag", "-a", target, "-m", message], root)
        run_git(["push", "origin", f"refs/tags/{target}"], root)
    return CutResult(version=target, previous=latest,
                     added=len(added), removed=len(removed))
