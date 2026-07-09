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
    publisher_scm: str
    publisher_repo: str
    include: list[str]
    exclude: list[str]
    seed: list[str]
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
        scm = pub.get("scm", "")
        if scm not in ("github", "azure-repos"):
            errs.append("publisher.scm must be 'github' or 'azure-repos'")
        repo = pub.get("repo", "")
        # A URL/path is used verbatim by git; shorthand is scm-expanded
        # (core/upstream.py). Core never branches on scm beyond that
        # expansion — it is provenance and link metadata (DR-0015).
        if not isinstance(repo, str) or not repo:
            errs.append("publisher.repo is required (git URL or shorthand)")

        include = data.get("include") or []
        seed = data.get("seed") or []
        if not all(isinstance(p, str) for p in include + seed):
            errs.append("include/seed must be lists of glob strings")
        if not include and not seed:
            errs.append("at least one of include/seed must be non-empty")
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
                 "seed", "adapters", "profiles", "retracted", "manifest_name"}
        for key in data:
            if key not in known:
                errs.append(f"unknown top-level key: {key!r}")

        if errs:
            raise UsageError(f"{path}: " + "; ".join(errs))
        return cls(
            slice_name=name, slice_title=title,
            publisher_scm=scm, publisher_repo=repo,
            include=include, exclude=exclude, seed=seed, adapters=adapters,
            profiles=profiles, retracted=retracted,
            manifest_name=manifest_name, path=path,
        )

    # -- export surface ----------------------------------------------------

    def _matched(self, root: str, patterns: list[str]) -> list[str]:
        """Sorted repo-relative posix paths: matched(patterns) − matched(exclude).

        Regular files only; symlinks are rejected (export-declaration spec §2).
        """
        rootp = Path(root)
        found: set[str] = set()
        for pattern in patterns:
            for hit in rootp.glob(pattern):
                rel = hit.relative_to(rootp).as_posix()
                if hit.is_symlink():
                    raise UsageError(f"symlink in export surface: {rel}")
                if hit.is_file():
                    found.add(rel)
        return [p for p in sorted(found) if not match_any(p, self.exclude)]

    def exported_files(self, root: str) -> list[str]:
        """The vendored (drift-gated) surface."""
        result = self._matched(root, self.include)
        if not result and not self.seed:
            raise UsageError(f"{self.path}: export surface is empty")
        overlap = set(result) & set(self._matched(root, self.seed))
        if overlap:
            # A path cannot be both drift-gated and free-to-diverge (DR-0013).
            raise UsageError(
                f"{self.path}: path(s) matched by both include and seed: "
                f"{', '.join(sorted(overlap)[:3])}"
            )
        return result

    def seeded_files(self, root: str) -> list[str]:
        """The scaffold-once surface (DR-0013): materialised only when the
        consumer path does not exist, then consumer-owned."""
        return self._matched(root, self.seed)

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
