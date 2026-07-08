"""Migrations: declarative payloads, window resolve, deterministic verify
(migrations spec, DR-0008).

`resolve` needs YAML (publisher payloads); `verify` is STDLIB ONLY — it runs
as an always-on required check on the consumer PR path (INV-9), consuming an
obligations JSON document.
"""

from __future__ import annotations

import json
import re
import subprocess
from dataclasses import dataclass, field
from pathlib import Path

from . import versions
from .util import UsageError, VendkitError, match_any

MIGRATIONS_DIR = "migrations"
KINDS = ("mechanical", "additive", "removal", "structural", "convention")
JUDGMENT_KINDS = ("structural", "convention")
CHECK_KINDS = ("file-absent", "file-present", "tool")
_ID = re.compile(r"^[a-z0-9][a-z0-9-]*$")


# -- resolve (publisher payloads; YAML) --------------------------------------

def load_entries(publisher_root: str) -> list[dict]:
    """Top-level migrations/*.yml only (examples/ etc. are not loaded)."""
    from .util import load_yaml  # lazy YAML path

    entries = []
    d = Path(publisher_root) / MIGRATIONS_DIR
    for f in sorted(d.glob("*.yml")) if d.is_dir() else []:
        data = load_yaml(str(f))
        errs = []
        if data.get("schema_version") != 1:
            errs.append("schema_version must be 1")
        if not _ID.match(data.get("id", "")):
            errs.append("id must be kebab-case")
        if versions.parse(data.get("applies_from", ""), "rc") is None:
            errs.append("applies_from must be release-shaped")
        if data.get("kind") not in KINDS:
            errs.append(f"kind must be one of {KINDS}")
        ver = data.get("verification") or {}
        obligations = (
            ver.get("must_be_absent") or ver.get("must_be_present")
            or ver.get("checks")
        )
        if not obligations:
            errs.append("verification must declare at least one obligation")
        for chk in ver.get("checks") or []:
            if chk.get("kind") not in CHECK_KINDS:
                errs.append(f"unknown check kind {chk.get('kind')!r}")
        if errs:
            raise UsageError(f"{f}: " + "; ".join(errs))
        data["_file"] = f.name
        entries.append(data)
    return entries


def resolve(
    entries: list[dict],
    pinned: str,
    target: str,
    profile: str | None,
    kinds: tuple[str, ...] = JUDGMENT_KINDS,
) -> tuple[list[dict], dict]:
    """Applicable entries in (pinned, target] for the profile, plus the
    aggregated obligations document handed to the verifier."""
    applicable = []
    for e in entries:
        if e["kind"] not in kinds:
            continue
        if not versions.in_window(pinned, e["applies_from"], target):
            continue
        profs = e.get("profiles") or ["*"]
        if "*" not in profs and (profile is None or profile not in profs):
            continue
        applicable.append(e)
    applicable.sort(key=lambda e: versions.require(e["applies_from"]))

    agg: dict = {"must_be_absent": [], "must_be_present": [], "checks": []}
    for e in applicable:
        ver = e.get("verification") or {}
        agg["must_be_absent"] += ver.get("must_be_absent") or []
        agg["must_be_present"] += ver.get("must_be_present") or []
        agg["checks"] += ver.get("checks") or []
    return applicable, agg


# -- verify (consumer PR path; stdlib only) -----------------------------------

@dataclass
class VerifyReport:
    failures: list[str] = field(default_factory=list)
    checked: int = 0

    @property
    def clean(self) -> bool:
        return not self.failures


def _tracked_files(consumer_root: str) -> list[str]:
    proc = subprocess.run(
        ["git", "ls-files"], cwd=consumer_root, capture_output=True, text=True
    )
    if proc.returncode != 0:
        raise VendkitError(f"git ls-files failed: {proc.stderr.strip()}")
    return proc.stdout.splitlines()


def verify(consumer_root: str, obligations: dict) -> VerifyReport:
    """Deterministic obligation check over tracked files. Zero obligations
    is a green no-op — safe as an always-on required check (migrations §4).

    Uses the same glob matcher as resolve/gate (util.path_match)."""
    report = VerifyReport()
    files = None  # lazily listed: zero obligations must not need git

    def tracked() -> list[str]:
        nonlocal files
        if files is None:
            files = _tracked_files(consumer_root)
        return files

    for glob in obligations.get("must_be_absent") or []:
        report.checked += 1
        hits = [f for f in tracked() if match_any(f, [glob])]
        if hits:
            report.failures.append(
                f"must_be_absent {glob!r} matches {len(hits)} file(s), e.g. {hits[0]}"
            )
    for glob in obligations.get("must_be_present") or []:
        report.checked += 1
        if not any(match_any(f, [glob]) for f in tracked()):
            report.failures.append(f"must_be_present {glob!r} matches nothing")
    for chk in obligations.get("checks") or []:
        report.checked += 1
        kind = chk.get("kind")
        if kind == "file-absent":
            if (Path(consumer_root) / chk["path"]).exists():
                report.failures.append(f"file-absent: {chk['path']} exists")
        elif kind == "file-present":
            if not (Path(consumer_root) / chk["path"]).exists():
                report.failures.append(f"file-present: {chk['path']} missing")
        elif kind == "tool":
            # Executes a manifest-tracked, gate-verified vendored tool —
            # never inline shell from upstream (migrations spec §5).
            tool = Path(consumer_root) / chk["path"]
            if not tool.is_file():
                report.failures.append(f"tool missing: {chk['path']}")
                continue
            proc = subprocess.run(
                [str(tool), *chk.get("args", [])],
                cwd=consumer_root, capture_output=True, text=True,
            )
            if proc.returncode != 0:
                report.failures.append(
                    f"tool {chk['path']} exited {proc.returncode}"
                )
        else:
            report.failures.append(f"unknown check kind {kind!r}")
    return report


def load_obligations(source: str) -> dict:
    """Obligations from an inline JSON string or an @file reference."""
    text = Path(source[1:]).read_text(encoding="utf-8") if source.startswith("@") else source
    try:
        data = json.loads(text)
    except json.JSONDecodeError as exc:
        raise UsageError(f"obligations are not valid JSON: {exc}") from exc
    if not isinstance(data, dict):
        raise UsageError("obligations must be a JSON object")
    return data
