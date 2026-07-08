"""Shared helpers. Stdlib only — imported by gate-path code (INV-9)."""

from __future__ import annotations

import fnmatch
import subprocess


class VendkitError(Exception):
    """Infrastructure failure. CLI maps to exit >= 4."""

    exit_code = 4


class UsageError(VendkitError):
    """Bad arguments or config. CLI maps to exit 2."""

    exit_code = 2


class Refusal(VendkitError):
    """Deliberate refusal (retracted target, tag moved…). CLI maps to exit 3.

    ``reason`` is a stable machine token emitted as ``refused=<reason>``.
    """

    exit_code = 3

    def __init__(self, reason: str, message: str):
        super().__init__(message)
        self.reason = reason


def path_match(path: str, pattern: str) -> bool:
    """THE glob matcher. Resolver, migration verifier and gate must all use
    this one implementation (single matching semantics — see migrations spec
    §4 and the conformance-kit contract).

    fnmatch over posix relative paths, with ``**`` additionally matching any
    number of path segments (fnmatch's ``*`` already crosses ``/``; this
    wrapper exists so the semantics are pinned by tests, not by accident).
    """
    return fnmatch.fnmatchcase(path, pattern)


def match_any(path: str, patterns: list[str]) -> bool:
    return any(path_match(path, p) for p in patterns)


def run_git(args: list[str], cwd: str) -> str:
    """Run git, return stdout stripped. Raises VendkitError on failure."""
    proc = subprocess.run(
        ["git", *args], cwd=cwd, capture_output=True, text=True
    )
    if proc.returncode != 0:
        raise VendkitError(
            f"git {' '.join(args)} failed in {cwd}: {proc.stderr.strip()}"
        )
    return proc.stdout.strip()


def load_yaml(path: str):
    """Lazy YAML load. Only publisher/sync-path code may call this; the
    gate path reads JSON exclusively (INV-9, DR-0011)."""
    try:
        import yaml  # noqa: PLC0415 — deliberate lazy import
    except ImportError as exc:  # pragma: no cover
        raise UsageError(
            f"reading {path} requires PyYAML (pip install vendkit[publisher])"
        ) from exc
    with open(path, encoding="utf-8") as fh:
        data = yaml.safe_load(fh)
    if data is None:
        data = {}
    if not isinstance(data, dict):
        raise UsageError(f"{path}: top level must be a mapping")
    return data
