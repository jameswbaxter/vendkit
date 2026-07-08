"""Content adapters (DR-0009): identity copy by default; two named
transforms. Deterministic pure functions of (bytes, params, profile name) —
they never read the consumer tree (INV-2).
"""

from __future__ import annotations

import re

from .declaration import ExportDecl


def apply_adapters(
    decl: ExportDecl, path: str, data: bytes, profile: str | None
) -> bytes:
    """Transform file content for a consumer. Path renaming is separate
    (ExportDecl.consumer_path); this handles bytes only."""
    for adapter in decl.adapters_for(path):
        if adapter.kind == "glob-localise":
            data = _localise(data, adapter.fm_field, adapter.catalogue, profile)
    return data


def _localise(
    data: bytes, fm_field: str, catalogue: dict[str, list[str]], profile: str | None
) -> bytes:
    """Prune a front-matter glob union to the consumer's profile.

    Keep a glob iff it is owned by the consumer's profile or owned by no
    profile (universal); drop globs owned only by other profiles
    (export-declaration spec §3). An unbound consumer keeps the union
    verbatim. Non-UTF-8 content is returned untouched.
    """
    if profile is None:
        return data
    owners: dict[str, set[str]] = {}
    for pname, globs in catalogue.items():
        for g in globs:
            owners.setdefault(g, set()).add(pname)

    try:
        text = data.decode("utf-8")
    except UnicodeDecodeError:
        return data

    # The field is a single front-matter line: `field: "g1, g2, g3"`.
    pattern = re.compile(
        rf"^({re.escape(fm_field)}:\s*)([\"']?)(.*?)(\2)\s*$", re.MULTILINE
    )

    def repl(m: re.Match) -> str:
        globs = [g.strip() for g in m.group(3).split(",") if g.strip()]
        kept = [
            g for g in globs
            if profile in owners.get(g, {profile})  # unowned => universal
        ]
        joined = ", ".join(kept)
        return f"{m.group(1)}{m.group(2)}{joined}{m.group(4)}"

    return pattern.sub(repl, text, count=1).encode("utf-8")
