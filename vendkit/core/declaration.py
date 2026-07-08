"""Export declaration: the single source of slice identity (DR-0002).

Schema: export-declaration spec v1. YAML is imported lazily (via util);
gate-path code never imports this module.
"""

from __future__ import annotations

import re
from dataclasses import dataclass, field
from pathlib import Path

from .util import UsageError, load_yaml, match_any

SLUG = re.compile(r"^[a-z][a-z0-9-]{0,15}$")
DEFAULT_DECL = "vendkit-export.yml"
ADAPTER_KINDS = ("prefix-namespace", "glob-localise")


@dataclass
class Adapter:
    kind: str
    match: str
    prefix: str = ""
    fm_field: str = ""
    catalogue: dict[str, list[str]] = field(default_factory=dict)


@dataclass
class Profile:
    name: str
    export_include: list[str] = field(default_factory=lambda: ["*"])
    export_exclude: list[str] = field(default_factory=list)


@dataclass
class ExportDecl:
    slice_name: str
    slice_title: str
    publisher_platform: str
    publisher_repo: str
    include: list[str]
    exclude: list[str]
    adapters: list[Adapter]
    profiles: dict[str, Profile]
    retracted: list[str]
    manifest_name: str
    path: str = DEFAULT_DECL

    # -- loading -----------------------------------------------------------

    @classmethod
    def load(cls, path: str) -> "ExportDecl":
        data = load_yaml(path)
        errs: list[str] = []

        if data.get("schema_version") != 1:
            errs.append("schema_version must be 1")

        sl = data.get("slice") or {}
        name = sl.get("name", "")
        if not isinstance(name, str) or not SLUG.match(name):
            errs.append(f"slice.name {name!r} must match {SLUG.pattern}")
        title = sl.get("title") or name

        pub = data.get("publisher") or {}
        platform = pub.get("platform", "")
        if platform not in ("github", "ado"):
            errs.append("publisher.platform must be 'github' or 'ado'")
        repo = pub.get("repo", "")
        if not isinstance(repo, str) or repo.count("/") != 1:
            errs.append("publisher.repo must be '<owner-or-project>/<repo>'")

        include = data.get("include") or []
        if not include or not all(isinstance(p, str) for p in include):
            errs.append("include must be a non-empty list of glob strings")
        exclude = data.get("exclude") or []

        adapters: list[Adapter] = []
        for i, raw in enumerate(data.get("adapters") or []):
            kind = raw.get("kind")
            if kind not in ADAPTER_KINDS:
                # Hard error, never a silent skip (DR-0009).
                errs.append(f"adapters[{i}]: unknown kind {kind!r}")
                continue
            adp = Adapter(kind=kind, match=raw.get("match", ""))
            if not adp.match:
                errs.append(f"adapters[{i}]: match is required")
            if kind == "prefix-namespace":
                adp.prefix = raw.get("prefix", "")
                if not adp.prefix:
                    errs.append(f"adapters[{i}]: prefix is required")
            else:
                adp.fm_field = raw.get("field", "")
                adp.catalogue = raw.get("catalogue") or {}
                if not adp.fm_field:
                    errs.append(f"adapters[{i}]: field is required")
            adapters.append(adp)

        profiles: dict[str, Profile] = {}
        for pname, praw in (data.get("profiles") or {}).items():
            praw = praw or {}
            es = praw.get("export_slice") or {}
            profiles[pname] = Profile(
                name=pname,
                export_include=es.get("include") or ["*"],
                export_exclude=es.get("exclude") or [],
            )

        retracted = data.get("retracted") or []
        manifest_name = data.get("manifest_name") or f"{name}-manifest.json"

        known = {"schema_version", "slice", "publisher", "include", "exclude",
                 "adapters", "profiles", "retracted", "manifest_name"}
        for key in data:
            if key not in known:
                errs.append(f"unknown top-level key: {key!r}")

        if errs:
            raise UsageError(f"{path}: " + "; ".join(errs))
        return cls(
            slice_name=name, slice_title=title,
            publisher_platform=platform, publisher_repo=repo,
            include=include, exclude=exclude, adapters=adapters,
            profiles=profiles, retracted=retracted,
            manifest_name=manifest_name, path=path,
        )

    # -- export surface ----------------------------------------------------

    def exported_files(self, root: str) -> list[str]:
        """Sorted repo-relative posix paths: matched(include) − matched(exclude).

        Regular files only; symlinks are rejected (export-declaration spec §2).
        """
        rootp = Path(root)
        found: set[str] = set()
        for pattern in self.include:
            for hit in rootp.glob(pattern):
                rel = hit.relative_to(rootp).as_posix()
                if hit.is_symlink():
                    raise UsageError(f"symlink in export surface: {rel}")
                if hit.is_file():
                    found.add(rel)
        result = [p for p in sorted(found) if not match_any(p, self.exclude)]
        if not result:
            raise UsageError(f"{self.path}: export surface is empty")
        return result

    # -- adapters ----------------------------------------------------------

    def adapters_for(self, path: str) -> list[Adapter]:
        hits = [a for a in self.adapters if match_any(path, [a.match])]
        for kind in ADAPTER_KINDS:
            if sum(1 for a in hits if a.kind == kind) > 1:
                raise UsageError(
                    f"{path}: more than one {kind} adapter matches"
                )
        return hits

    def consumer_path(self, path: str) -> str:
        for a in self.adapters_for(path):
            if a.kind == "prefix-namespace":
                head, _, tail = path.rpartition("/")
                if not tail.startswith(a.prefix):
                    tail = a.prefix + tail
                return f"{head}/{tail}" if head else tail
        return path

    # -- profiles ----------------------------------------------------------

    def profile_in_scope(self, profile: str | None, path: str) -> bool:
        """Whether reconcile-scope may offer `path` to this profile."""
        if profile is None or profile not in self.profiles:
            return True  # unbound consumer takes the whole surface
        prof = self.profiles[profile]
        return match_any(path, prof.export_include) and not match_any(
            path, prof.export_exclude
        )
