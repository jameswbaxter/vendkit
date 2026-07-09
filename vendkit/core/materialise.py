"""Materialisation — the sync lane's core (sync spec §2).

A pure function of (publisher tree, declaration, consumer profile, current
consumer manifest) — INV-2. `check` predicts `apply` exactly — INV-3.
Never deletes consumer files — INV-4/DR-0010.
"""

from __future__ import annotations

import os
import stat
from dataclasses import dataclass, field
from pathlib import Path

from .adapters import apply_adapters
from .declaration import ExportDecl
from .manifest import (
    SCHEMA_VERSION, VENDKIT_DIR, dump_manifest, load_manifest,
)
from .normalise import RECIPE, normalise_hash
from .sliceconfig import find_slice_config
from .util import UsageError, run_git


@dataclass
class SyncReport:
    updated: list[str] = field(default_factory=list)
    removed_upstream: list[str] = field(default_factory=list)
    added: list[str] = field(default_factory=list)
    seeded: list[str] = field(default_factory=list)          # DR-0013
    seed_retired: list[str] = field(default_factory=list)    # template gone upstream
    template_updated: list[str] = field(default_factory=list)  # informational only

    @property
    def changed(self) -> bool:
        # template_updated is deliberately excluded: an upstream template
        # change never forces a PR for a diverged, consumer-owned copy —
        # the note rides along with the next real sync (sync spec §4).
        return bool(self.updated or self.removed_upstream or self.added
                    or self.seeded or self.seed_retired)


def _render(decl: ExportDecl, publisher_root: str, rel: str,
            profile: str | None) -> tuple[bytes, str, bool]:
    """(post-adapter bytes, consumer_path, exec) for one exported file."""
    src = Path(publisher_root) / rel
    data = apply_adapters(decl, rel, src.read_bytes(), profile)
    return data, decl.consumer_path(rel), bool(src.stat().st_mode & stat.S_IXUSR)


def _tree_matches(consumer_root: str, cpath: str, data: bytes, execbit: bool) -> bool:
    f = Path(consumer_root) / cpath
    if not f.is_file():
        return False
    if f.read_bytes() != data:
        return False
    return bool(f.stat().st_mode & stat.S_IXUSR) == execbit


def _write(consumer_root: str, cpath: str, data: bytes, execbit: bool) -> None:
    f = Path(consumer_root) / cpath
    f.parent.mkdir(parents=True, exist_ok=True)
    f.write_bytes(data)
    mode = f.stat().st_mode
    if execbit:
        f.chmod(mode | stat.S_IXUSR | stat.S_IXGRP | stat.S_IXOTH)
    else:
        f.chmod(mode & ~(stat.S_IXUSR | stat.S_IXGRP | stat.S_IXOTH))


def materialise(
    publisher_root: str,
    consumer_root: str,
    decl: ExportDecl,
    target: str,
    apply: bool = False,
    reconcile_scope: bool = False,
) -> SyncReport:
    """Refresh the tracked slice from the publisher tree (the engine's own
    checkout at the target release — INV-6).

    In check mode nothing is written; classification is identical (INV-3):
    a file counts `updated`/`added` iff the working tree differs from what
    apply would write, so locally drifted or missing files self-heal.
    """
    report = SyncReport()
    manifest_path = str(Path(consumer_root) / VENDKIT_DIR / decl.manifest_name)
    current = load_manifest(manifest_path)
    cfg = find_slice_config(consumer_root, decl.slice_name)
    profile = cfg.profile if cfg else None
    if profile and profile not in decl.profiles and decl.profiles:
        raise UsageError(
            f"profile {profile!r} not declared by slice {decl.slice_name!r}"
        )

    exported = set(decl.exported_files(publisher_root))
    seeds = set(decl.seeded_files(publisher_root))
    tracked_entries = {e["path"]: e for e in current.get("entries", [])}
    tracked = list(tracked_entries)
    new_entries: list[dict] = []

    def _emit_entry(rel: str, bucket: list[str]) -> None:
        data, cpath, execbit = _render(decl, publisher_root, rel, profile)
        digest, raw = normalise_hash(data)
        new_entries.append({
            "path": rel, "consumer_path": cpath,
            "sha256": digest, "exec": execbit, "raw": raw,
        })
        if not _tree_matches(consumer_root, cpath, data, execbit):
            bucket.append(cpath)
            if apply:
                _write(consumer_root, cpath, data, execbit)

    def _emit_seed(rel: str) -> None:
        """Scaffold-once (DR-0013): a tracked seed is NEVER written again —
        the entry is the 'seeding happened' record, so deletion is respected.
        The entry's sha tracks the TEMPLATE (divergence-note comparator)."""
        data, cpath, execbit = _render(decl, publisher_root, rel, profile)
        digest, raw = normalise_hash(data)
        new_entries.append({
            "path": rel, "consumer_path": cpath, "sha256": digest,
            "exec": execbit, "raw": raw, "seed": True,
        })
        prior = tracked_entries.get(rel)
        if prior is None:
            # Untracked: seed if absent; adopt (entry only, never clobber)
            # if the consumer already has a file at the target path.
            report.seeded.append(cpath)
            if apply and not (Path(consumer_root) / cpath).is_file():
                _write(consumer_root, cpath, data, execbit)
        elif (prior.get("sha256") != digest
              and (Path(consumer_root) / cpath).is_file()):
            report.template_updated.append(cpath)

    # 1. Tracked refresh + 2. removals (report-only; files stay on disk).
    for rel in tracked:
        if rel in seeds:
            _emit_seed(rel)
        elif rel in exported:
            # Includes a publisher reclassifying a seed as vendored: the
            # refresh overwrites — a deliberate, PR-visible class change.
            _emit_entry(rel, report.updated)
        elif tracked_entries[rel].get("seed"):
            # Template retired upstream: the consumer's copy is theirs now;
            # nothing to delete, just stop tracking.
            report.seed_retired.append(rel)
        else:
            report.removed_upstream.append(rel)

    # 3. Additions — opt-in, bounded by the profile's export slice (DR-0010).
    if reconcile_scope:
        for rel in sorted((exported | seeds) - set(tracked)):
            if not decl.profile_in_scope(profile, rel):
                continue
            if rel in seeds:
                _emit_seed(rel)
            else:
                _emit_entry(rel, report.added)

    # 4. Manifest rewrite with provenance (manifest spec §1).
    if apply:
        new_manifest = {
            "schema_version": SCHEMA_VERSION,
            "slice": decl.slice_name,
            "profile": profile or "*",
            "normalisation": RECIPE,
            "source": {
                "scm": decl.publisher_scm,
                "repo": decl.publisher_repo,
                "release": target,
                "commit": run_git(["rev-parse", "HEAD"], publisher_root),
            },
            "entries": sorted(new_entries, key=lambda e: e["path"]),
        }
        dump_manifest(new_manifest, manifest_path)
    return report


def seed_empty_manifest(consumer_root: str, decl: ExportDecl) -> str:
    """Onboarding seed: an empty tracked slice for reconcile to expand."""
    path = Path(consumer_root) / VENDKIT_DIR / decl.manifest_name
    if path.exists():
        raise UsageError(f"manifest already exists: {path}")
    dump_manifest({
        "schema_version": SCHEMA_VERSION,
        "slice": decl.slice_name,
        "profile": "*",
        "normalisation": RECIPE,
        "entries": [],
    }, str(path))
    return str(path)
