"""Consumer slice config: .vendkit/<slice>.yml (DR-0012, onboarding spec §1)."""

from __future__ import annotations

from dataclasses import dataclass, field
from pathlib import Path

from .manifest import VENDKIT_DIR
from .util import UsageError, load_yaml


@dataclass
class SliceConfig:
    slice_name: str
    publisher_platform: str
    publisher_repo: str
    profile: str | None
    pin_file: str
    pin_pattern: str
    pin_files: list[str]
    channel: str
    handoff_kind: str
    handoff_dedup_key: str
    handoff_routing: dict
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
        handoff = watch.get("handoff") or {}
        if not name:
            errs.append("slice is required")
        if pub.get("platform") not in ("github", "ado"):
            errs.append("publisher.platform must be 'github' or 'ado'")
        if not pin.get("file") or not pin.get("pattern"):
            errs.append("pin.file and pin.pattern are required")
        channel = watch.get("channel", "stable")
        if channel not in ("stable", "rc"):
            errs.append("watch.channel must be 'stable' or 'rc'")
        if errs:
            # A half-configured slice must be loud for every command (DR-0012).
            raise UsageError(f"{path}: " + "; ".join(errs))
        return cls(
            slice_name=name,
            publisher_platform=pub["platform"],
            publisher_repo=pub.get("repo", ""),
            profile=data.get("profile"),
            pin_file=pin["file"],
            pin_pattern=pin["pattern"],
            pin_files=pin.get("files") or [pin["file"]],
            channel=channel,
            handoff_kind=handoff.get("kind", "issue"),
            handoff_dedup_key=handoff.get("dedup_key", f"vendkit-watch-{name}"),
            handoff_routing=handoff.get("routing") or {},
            attestations=data.get("attestations") or {},
            waivers=data.get("waivers") or [],
            path=path,
        )


def discover_slice_configs(consumer_root: str) -> list[SliceConfig]:
    """Fixed discovery: every .vendkit/*.yml is a slice config (DR-0012)."""
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
    """Scan pin.file for pin.pattern immediately followed by a version.

    Pattern present but no parsable version => loud error, never a skip
    (release-watch spec §2)."""
    import re

    from . import versions

    path = Path(consumer_root) / cfg.pin_file
    if not path.is_file():
        raise UsageError(f"{cfg.path}: pin.file not found: {cfg.pin_file}")
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
    raise UsageError(f"{cfg.path}: pin unreadable in {cfg.pin_file}: {reason}")
