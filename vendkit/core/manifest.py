"""Manifest build/load/diff and the gate lane.

STDLIB ONLY (INV-9): this module is on the consumer PR path. It reads JSON
manifests and files — never YAML, never the export declaration. Publisher
manifest *building* takes an ExportDecl argument, but importing this module
pulls in no YAML.
"""

from __future__ import annotations

import json
import os
import stat
from dataclasses import dataclass, field
from pathlib import Path
from typing import TYPE_CHECKING

from .normalise import RECIPE, hash_as_recorded, normalise_hash
from .util import UsageError, VendkitError

if TYPE_CHECKING:  # no runtime import — keeps the gate path YAML-free
    from .declaration import ExportDecl

SCHEMA_VERSION = 1
VENDKIT_DIR = ".vendkit"


def _is_exec(path: Path) -> bool:
    return bool(path.stat().st_mode & stat.S_IXUSR)


def build_publisher_manifest(decl: "ExportDecl", root: str) -> dict:
    """Publisher-side manifest of the working tree (manifest spec §1).

    Hashes are of the file *as vendored for an unbound consumer* — i.e. the
    raw exported bytes (adapters that depend on a profile do not apply to the
    publisher manifest; consumer manifests re-hash post-adapter bytes)."""
    entries = []
    seen_consumer: dict[str, str] = {}
    for rel in decl.exported_files(root):
        p = Path(root) / rel
        digest, raw = normalise_hash(p.read_bytes())
        consumer_path = decl.consumer_path(rel)
        if consumer_path in seen_consumer:
            raise UsageError(
                f"consumer_path collision inside slice: {consumer_path} "
                f"({seen_consumer[consumer_path]} vs {rel})"
            )
        seen_consumer[consumer_path] = rel
        entries.append({
            "path": rel,
            "consumer_path": consumer_path,
            "sha256": digest,
            "exec": _is_exec(p),
            "raw": raw,
        })
    return {
        "schema_version": SCHEMA_VERSION,
        "slice": decl.slice_name,
        "profile": "*",
        "normalisation": RECIPE,
        "entries": entries,
    }


def load_manifest(path: str) -> dict:
    try:
        with open(path, encoding="utf-8") as fh:
            data = json.load(fh)
    except FileNotFoundError as exc:
        raise UsageError(f"manifest not found: {path}") from exc
    except json.JSONDecodeError as exc:
        raise VendkitError(f"manifest unreadable: {path}: {exc}") from exc
    if data.get("schema_version") != SCHEMA_VERSION:
        raise VendkitError(
            f"{path}: unsupported schema_version "
            f"{data.get('schema_version')!r} (engine supports {SCHEMA_VERSION})"
        )
    return data


def dump_manifest(manifest: dict, path: str) -> None:
    Path(path).parent.mkdir(parents=True, exist_ok=True)
    with open(path, "w", encoding="utf-8") as fh:
        json.dump(manifest, fh, indent=2, sort_keys=True)
        fh.write("\n")


def manifests_equal(a: dict, b: dict) -> bool:
    return json.dumps(a, sort_keys=True) == json.dumps(b, sort_keys=True)


# -- gate lane ---------------------------------------------------------------

@dataclass
class Finding:
    manifest: str
    slice_name: str
    consumer_path: str
    kind: str  # changed | removed | collision
    detail: str = ""


@dataclass
class GateReport:
    findings: list[Finding] = field(default_factory=list)
    checked: int = 0

    @property
    def clean(self) -> bool:
        return not self.findings


def discover_manifests(consumer_root: str) -> list[str]:
    """The fixed discovery convention (DR-0012): .vendkit/*-manifest.json."""
    d = Path(consumer_root) / VENDKIT_DIR
    return sorted(str(p) for p in d.glob("*-manifest.json")) if d.is_dir() else []


def gate_check(consumer_root: str, manifest_paths: list[str]) -> GateReport:
    """Verify vendored files against their manifests; enforce INV-7.

    Findings: `changed` (hash or exec differs), `removed` (missing),
    `collision` (a consumer_path claimed by two manifests). There is no
    `added` finding — consumers own everything outside the tracked slices.
    """
    report = GateReport()
    claimed: dict[str, str] = {}
    for mpath in manifest_paths:
        manifest = load_manifest(mpath)
        slice_name = manifest.get("slice", "?")
        for entry in manifest.get("entries", []):
            cpath = entry["consumer_path"]
            report.checked += 1
            if cpath in claimed:
                report.findings.append(Finding(
                    mpath, slice_name, cpath, "collision",
                    f"also tracked by {claimed[cpath]}",
                ))
                continue
            claimed[cpath] = mpath
            f = Path(consumer_root) / cpath
            if not f.is_file():
                report.findings.append(
                    Finding(mpath, slice_name, cpath, "removed")
                )
                continue
            digest = hash_as_recorded(f.read_bytes(), entry.get("raw", False))
            if digest != entry["sha256"]:
                report.findings.append(
                    Finding(mpath, slice_name, cpath, "changed", "content differs")
                )
            elif _is_exec(f) != entry.get("exec", False):
                report.findings.append(
                    Finding(mpath, slice_name, cpath, "changed", "exec bit differs")
                )
    return report
