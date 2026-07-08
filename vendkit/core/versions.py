"""Version grammar, ordering, channels, retraction awareness.

Stdlib only — ``is-newer`` runs on the consumer PR path (INV-9).
Grammar and semantics: releases-and-versioning spec §2.
"""

from __future__ import annotations

import re

from .util import Refusal, UsageError

_STABLE = re.compile(r"^v(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)$")
_RC = re.compile(r"^v(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)-rc\.([1-9]\d*)$")

# Ordering key: (major, minor, patch, is_stable, rc_n). A stable release
# sorts above every rc of the same triple (SemVer §11).
Key = tuple[int, int, int, int, int]


def parse(version: str, channel: str = "stable") -> Key | None:
    """Parse a tag name; None if it is not release-shaped for the channel."""
    m = _STABLE.match(version)
    if m:
        a, b, c = (int(g) for g in m.groups())
        return (a, b, c, 1, 0)
    if channel == "rc":
        m = _RC.match(version)
        if m:
            a, b, c, n = (int(g) for g in m.groups())
            return (a, b, c, 0, n)
    return None


def require(version: str, channel: str = "rc") -> Key:
    """Parse accepting the widest grammar; UsageError if malformed."""
    key = parse(version, channel)
    if key is None:
        raise UsageError(f"not a release version: {version!r}")
    return key


def is_newer(pinned: str, target: str, retracted: list[str] | None = None) -> bool:
    """True iff target > pinned. Refuses a retracted target (exit 3)."""
    if retracted and target in retracted:
        raise Refusal("retracted", f"target {target} is retracted by the publisher")
    return require(target) > require(pinned)


def latest(
    tags: list[str], channel: str = "stable", retracted: list[str] | None = None
) -> str | None:
    """Greatest qualifying version among tag names; None when none qualify."""
    best: tuple[Key, str] | None = None
    for name in tags:
        if retracted and name in retracted:
            continue
        key = parse(name, channel)
        if key is None:
            continue
        if best is None or key > best[0]:
            best = (key, name)
    return best[1] if best else None


def classify_bump(pinned: str, target: str) -> str:
    """'patch' | 'minor' | 'major' for the pinned→target jump."""
    p, t = require(pinned), require(target)
    if t[0] != p[0]:
        return "major"
    if t[1] != p[1]:
        return "minor"
    return "patch"


def bump(version: str, kind: str) -> str:
    a, b, c, stable, _ = require(version)
    if not stable:
        raise UsageError(f"cannot bump from a pre-release: {version}")
    if kind == "major":
        return f"v{a + 1}.0.0"
    if kind == "minor":
        return f"v{a}.{b + 1}.0"
    if kind == "patch":
        return f"v{a}.{b}.{c + 1}"
    raise UsageError(f"unknown bump kind: {kind!r}")


def in_window(pinned: str, applies_from: str, target: str) -> bool:
    """Migration window arithmetic: pinned < applies_from <= target."""
    return require(pinned) < require(applies_from) <= require(target)
