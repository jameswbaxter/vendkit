"""Release watch (release-watch spec). Pure detection — no delivery.

Watch compares each slice's pin against the publisher's latest qualifying
release over the git protocol (core/upstream.py) and returns findings.
Turning findings into tickets is the handoff handler's job (DR-0014); the
CLI invokes it, this module never does.
"""

from __future__ import annotations

from dataclasses import dataclass, field
from pathlib import Path

from . import upstream, versions
from .manifest import VENDKIT_DIR, load_manifest
from .sliceconfig import SliceConfig, discover_slice_configs, read_pin
from .util import UsageError


@dataclass
class WatchFinding:
    slice_name: str
    kind: str  # update-available | tag-moved | pin-unreadable | no-releases
    pinned: str = ""
    latest: str = ""
    bump: str = ""
    detail: str = ""


@dataclass
class WatchReport:
    findings: list[WatchFinding] = field(default_factory=list)
    slices: int = 0

    @property
    def actionable(self) -> list[WatchFinding]:
        return [f for f in self.findings if f.kind != "no-releases"]


def retracted_at_newest(url: str, newest_tag: str) -> list[str]:
    """Read the retraction list from the NEWEST release's declaration
    (releases spec §4 bootstrapping quirk). Best-effort: an unreadable
    declaration means no retractions, not a crash."""
    try:
        raw = upstream.read_file_at(url, newest_tag, "vendkit-export.yml")
        import yaml
        data = yaml.safe_load(raw) or {}
        return list(data.get("retracted") or [])
    except Exception:
        return []


def watch(
    consumer_root: str,
    slice_name: str | None = None,
    dry_run: bool = False,
) -> WatchReport:
    report = WatchReport()
    configs = discover_slice_configs(consumer_root)
    if slice_name:
        configs = [c for c in configs if c.slice_name == slice_name]
        if not configs:
            raise UsageError(f"no slice config for {slice_name!r}")
    for cfg in configs:
        report.slices += 1
        if dry_run:
            continue  # PR self-test: no network, empty successful report
        report.findings.extend(_watch_one(consumer_root, cfg))
    return report


def _watch_one(consumer_root: str, cfg: SliceConfig) -> list[WatchFinding]:
    findings: list[WatchFinding] = []
    # 1. PINNED — config rot must be loud (spec §2).
    try:
        pinned = read_pin(consumer_root, cfg)
    except UsageError as exc:
        return [WatchFinding(cfg.slice_name, "pin-unreadable", detail=str(exc))]

    # 2. LATEST over the git protocol (vendor-free — DR-0015).
    url = upstream.clone_url(cfg.publisher_scm, cfg.publisher_repo)
    tags = upstream.list_release_tags(url)

    names = [t.name for t in tags]
    newest = versions.latest(names, channel="rc")
    retracted = retracted_at_newest(url, newest) if newest else []
    latest = versions.latest(names, channel=cfg.channel, retracted=retracted)
    if latest is None:
        return [WatchFinding(cfg.slice_name, "no-releases")]

    # 3. Provenance: pinned tag must still resolve to the recorded commit
    # (security model §3).
    manifest_path = Path(consumer_root) / VENDKIT_DIR / f"{cfg.slice_name}-manifest.json"
    if manifest_path.is_file():
        source = load_manifest(str(manifest_path)).get("source") or {}
        recorded = source.get("commit")
        if recorded and source.get("release") == pinned:
            now = {t.name: t.commit for t in tags}.get(pinned)
            if now and now != recorded:
                findings.append(WatchFinding(
                    cfg.slice_name, "tag-moved", pinned=pinned,
                    detail=f"tag {pinned} resolved to {now[:12]}, "
                           f"manifest recorded {recorded[:12]}",
                ))

    # 4. Compare.
    if versions.require(latest) > versions.require(pinned):
        findings.append(WatchFinding(
            cfg.slice_name, "update-available", pinned=pinned, latest=latest,
            bump=versions.classify_bump(pinned, latest),
        ))
    return findings


def render_report(report: WatchReport) -> str:
    lines = [f"# vendkit watch — {report.slices} slice(s)", ""]
    if not report.findings:
        lines.append("No findings.")
    for f in report.findings:
        head = f"- **{f.slice_name}**: {f.kind}"
        if f.kind == "update-available":
            head += f" {f.pinned} → {f.latest} ({f.bump})"
        if f.detail:
            head += f" — {f.detail}"
        lines.append(head)
    return "\n".join(lines) + "\n"
