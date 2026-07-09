"""Consumer slice config: .vendkit/<slice>.yml (DR-0012, onboarding spec §1).

Two axis fields record the consumer's environment explicitly (DR-0015):
`scm` (where the repo is hosted — github | azure-repos) and `ci` (what runs
the pipelines — github-actions | azure-pipelines | none). `ci: none` is the
fully-manual mode: no scaffolded pipelines, the human orchestrates, and the
manifest's provenance is the sole pin authority.
"""

from __future__ import annotations

from dataclasses import dataclass, field
from pathlib import Path

from .manifest import VENDKIT_DIR
from .util import UsageError, load_yaml

SCM_VALUES = ("github", "azure-repos")
CI_VALUES = ("github-actions", "azure-pipelines", "none")
HANDLER_KINDS = ("pr", "handoff", "fact-verify")


@dataclass
class SliceConfig:
    slice_name: str
    publisher_scm: str
    publisher_repo: str
    scm: str
    ci: str
    profile: str | None
    pin_pattern: str
    pin_files: list[str]
    channel: str
    handlers: dict[str, dict]
    handoff_dedup_key: str
    seed_notes: str
    attestations: dict[str, bool]
    waivers: list[dict]
    path: str

    @classmethod
    def load(cls, path: str) -> "SliceConfig":
        data = load_yaml(path)
        errs = []
        if data.get("schema_version") != 1:
            errs.append("schema_version must be 1")
        name = data.get("slice", "")
        pub = data.get("publisher") or {}
        pin = data.get("pin") or {}
        watch = data.get("watch") or {}
        if not name:
            errs.append("slice is required")
        if pub.get("scm") not in SCM_VALUES:
            errs.append(f"publisher.scm must be one of {SCM_VALUES}")
        scm = data.get("scm", "")
        if scm not in SCM_VALUES:
            errs.append(f"scm must be one of {SCM_VALUES}")
        ci = data.get("ci", "")
        if ci not in CI_VALUES:
            errs.append(f"ci must be one of {CI_VALUES}")
        pin_files = pin.get("files") or []
        pin_pattern = pin.get("pattern", "")
        if ci == "none":
            # Manual mode: the manifest's source.release IS the pin; a pin
            # block is meaningless without pipeline files to advance.
            if pin_files or pin_pattern:
                errs.append("pin block must be empty when ci is 'none'")
        elif not pin_files or not pin_pattern:
            errs.append("pin.pattern and pin.files (first entry is the "
                        "authoritative read source) are required")
        channel = watch.get("channel", "stable")
        if channel not in ("stable", "rc"):
            errs.append("watch.channel must be 'stable' or 'rc'")
        handlers = data.get("handlers") or {}
        for kind, spec in handlers.items():
            if kind not in HANDLER_KINDS:
                errs.append(f"handlers.{kind}: unknown kind "
                            f"(expected one of {HANDLER_KINDS})")
                continue
            command = (spec or {}).get("exec")
            if not (isinstance(command, list) and command
                    and all(isinstance(c, str) for c in command)):
                errs.append(f"handlers.{kind}.exec must be a non-empty "
                            "list of strings")
        seeds = data.get("seeds") or {}
        seed_notes = seeds.get("notes", "informational")
        if seed_notes not in ("informational", "silent"):
            errs.append("seeds.notes must be 'informational' or 'silent'")
        if errs:
            # A half-configured slice must be loud for every command
            # (DR-0012) — this is also what makes the .vendkit/ namespace
            # strict: any *.yml there MUST parse as a slice config.
            raise UsageError(f"{path}: " + "; ".join(errs))
        return cls(
            slice_name=name,
            publisher_scm=pub["scm"],
            publisher_repo=pub.get("repo", ""),
            scm=scm,
            ci=ci,
            profile=data.get("profile"),
            pin_pattern=pin_pattern,
            pin_files=pin_files,
            channel=channel,
            handlers=handlers,
            handoff_dedup_key=(handlers.get("handoff") or {}).get(
                "dedup_key", f"vendkit-watch-{name}"),
            seed_notes=seed_notes,
            attestations=data.get("attestations") or {},
            waivers=data.get("waivers") or [],
            path=path,
        )


def discover_slice_configs(consumer_root: str) -> list[SliceConfig]:
    """Fixed discovery: every .vendkit/*.yml is a slice config (DR-0012).
    A stray YAML file there is a usage error, never a silent skip."""
    d = Path(consumer_root) / VENDKIT_DIR
    if not d.is_dir():
        return []
    return [SliceConfig.load(str(p)) for p in sorted(d.glob("*.yml"))]


def find_slice_config(consumer_root: str, slice_name: str) -> SliceConfig | None:
    for cfg in discover_slice_configs(consumer_root):
        if cfg.slice_name == slice_name:
            return cfg
    return None


def read_pin(consumer_root: str, cfg: SliceConfig) -> str:
    """The consumer's pinned-release intent.

    `ci: none` has no pin lines: the slice manifest's source.release is the
    only authority. Otherwise scan pin.files[0] for pin.pattern immediately
    followed by a version — pattern present but no parsable version is a
    loud error, never a skip (release-watch spec §2)."""
    import re

    from . import versions

    if cfg.ci == "none":
        from .manifest import load_manifest
        mpath = Path(consumer_root) / VENDKIT_DIR / f"{cfg.slice_name}-manifest.json"
        release = (load_manifest(str(mpath)).get("source") or {}).get("release")
        if release:
            return release
        raise UsageError(
            f"{cfg.path}: ci is 'none' and the manifest records no "
            "source.release — vendor the slice first")

    pin_file = cfg.pin_files[0]
    path = Path(consumer_root) / pin_file
    if not path.is_file():
        raise UsageError(f"{cfg.path}: pin file not found: {pin_file}")
    seen_pattern = False
    for line in path.read_text(encoding="utf-8").splitlines():
        idx = line.find(cfg.pin_pattern)
        if idx < 0:
            continue
        seen_pattern = True
        # pin.pattern conventionally ends at (and includes) the leading 'v';
        # the tail is the numeric remainder (e.g. "1.4.2").
        tail = line[idx + len(cfg.pin_pattern):]
        m = re.match(r"([0-9][0-9A-Za-z.\-]*)", tail)
        if m:
            candidate = "v" + m.group(1)
            if versions.parse(candidate, "rc"):
                return candidate
    reason = "pattern found but no parsable version" if seen_pattern else "pattern not found"
    raise UsageError(f"{cfg.path}: pin unreadable in {pin_file}: {reason}")
