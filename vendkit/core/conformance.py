"""Conformance: rule spec evaluation with platform-keyed detector bindings
(conformance spec; DR-0007 §3 for the decidability split).
"""

from __future__ import annotations

import re
import subprocess
from dataclasses import dataclass, field
from pathlib import Path

from .manifest import VENDKIT_DIR, discover_manifests, load_manifest
from .sliceconfig import SliceConfig
from .util import UsageError, load_yaml, match_any

SEVERITIES = ("mandatory", "waivable", "advisory")
STATUSES = ("pass", "fail", "waived", "attested", "skipped", "error")

CORE_RULES = Path(__file__).resolve().parent.parent.parent / "conformance" / "core-rules.yml"


@dataclass
class RuleResult:
    rule_id: str
    title: str
    severity: str
    status: str
    detail: str = ""

    @property
    def is_gap(self) -> bool:
        return self.status in ("fail", "error")


@dataclass
class ConformanceReport:
    results: list[RuleResult] = field(default_factory=list)

    @property
    def gaps(self) -> list[RuleResult]:
        return [r for r in self.results if r.is_gap]


def load_rules(paths: list[str]) -> list[dict]:
    rules, seen = [], set()
    for path in paths:
        data = load_yaml(path)
        if data.get("schema_version") != 1:
            raise UsageError(f"{path}: schema_version must be 1")
        for rule in data.get("rules") or []:
            rid = rule.get("id", "")
            if not rid or rid in seen:
                raise UsageError(f"{path}: missing or duplicate rule id {rid!r}")
            if rule.get("severity") not in SEVERITIES:
                raise UsageError(f"{path}: rule {rid}: bad severity")
            if not (rule.get("detector") or {}).get("kind"):
                raise UsageError(f"{path}: rule {rid}: detector.kind required")
            seen.add(rid)
            rules.append(rule)
    return rules


# -- pipeline discovery/parsing (Layer 1 detector bindings) --------------------

def _pipeline_files(consumer_root: str, platform: str) -> list[Path]:
    root = Path(consumer_root)
    if platform == "github":
        return sorted((root / ".github" / "workflows").glob("*.yml")) + sorted(
            (root / ".github" / "workflows").glob("*.yaml"))
    if platform == "ado":
        found = sorted((root / "azure-pipelines").glob("*.yml"))
        top = root / "azure-pipelines.yml"
        return ([top] if top.is_file() else []) + found
    return []


@dataclass
class PipelineInfo:
    path: Path
    text: str
    data: dict


def _load_pipelines(consumer_root: str, platform: str) -> list[PipelineInfo]:
    import yaml
    infos = []
    for f in _pipeline_files(consumer_root, platform):
        try:
            data = yaml.safe_load(f.read_text(encoding="utf-8")) or {}
        except yaml.YAMLError:
            data = {}
        infos.append(PipelineInfo(f, f.read_text(encoding="utf-8"), data))
    return infos


# A component is "wired" when the pipeline invokes its CLI subcommand —
# true for every scaffold shape on both platforms (direct `python -m
# vendkit.cli`, composite action, or step template all bottom out in the
# invocation). "Pinned" when the file carries a release-tag reference
# (GHA checkout `ref: refs/tags/vX.Y.Z` / `uses: …@vX.Y.Z`; ADO
# resources.repositories `ref: refs/tags/vX.Y.Z`).
_INVOKE = {
    "gate": re.compile(r"vendkit(\.cli)?\s+gate\b"),
    "sync": re.compile(r"vendkit(\.cli)?\s+sync-pipeline\b"),
    "watch": re.compile(r"vendkit(\.cli)?\s+watch\b"),
    "conformance": re.compile(r"vendkit(\.cli)?\s+conformance\b"),
    "migration-verify": re.compile(r"vendkit(\.cli)?\s+migrations-verify\b"),
}
_PIN = re.compile(
    r"(refs/tags/v\d+\.\d+\.\d+(-rc\.\d+)?\b)"
    r"|(@v\d+\.\d+\.\d+(-rc\.\d+)?\b)"
    r"|(@[0-9a-f]{40}\b)"
)


def _wired(info: PipelineInfo, component: str, platform: str,
           publisher_repo: str) -> tuple[bool, bool]:
    """(references component, pinned-to-tag) for one pipeline file."""
    rx = _INVOKE.get(component)
    if rx is None or not rx.search(info.text):
        return False, False
    return True, bool(_PIN.search(info.text))


def _has_event(info: PipelineInfo, event: str, platform: str) -> bool | None:
    """True/False when tree-decidable; None when the platform cannot say
    (→ attest degradation, conformance spec §3)."""
    if platform == "github":
        on = info.data.get("on") or info.data.get(True) or {}
        if isinstance(on, str):
            on = {on: None}
        if isinstance(on, list):
            on = dict.fromkeys(on)
        return ("pull_request" if event == "pull_request" else "schedule") in on
    if event == "schedule":
        return "schedules" in info.data
    return None  # ADO PR gating is a branch policy: not tree-decidable


# -- detectors ----------------------------------------------------------------

def evaluate(
    consumer_root: str,
    cfg: SliceConfig,
    platform: str,
    rules: list[dict],
) -> ConformanceReport:
    report = ConformanceReport()
    waived = {w.get("rule"): w.get("reason", "") for w in cfg.waivers}
    manifest_path = Path(consumer_root) / VENDKIT_DIR / f"{cfg.slice_name}-manifest.json"
    manifest = load_manifest(str(manifest_path)) if manifest_path.is_file() else None
    pipelines = _load_pipelines(consumer_root, platform)

    for rule in rules:
        rid, severity = rule["id"], rule["severity"]
        det = rule["detector"]
        if rid in waived:
            if severity == "waivable":
                report.results.append(RuleResult(
                    rid, rule.get("title", ""), severity, "waived", waived[rid]))
                continue
            report.results.append(RuleResult(
                rid, rule.get("title", ""), severity, "fail",
                "rule is mandatory and cannot be waived"))
            continue
        try:
            status, detail = _detect(
                det, consumer_root, cfg, platform, manifest, pipelines)
        except Exception as exc:  # detector crash = error status, not a crash
            status, detail = "error", str(exc)
        report.results.append(
            RuleResult(rid, rule.get("title", ""), severity, status, detail))
    return report


def _detect(det, consumer_root, cfg, platform, manifest, pipelines):
    kind = det["kind"]
    root = Path(consumer_root)

    if kind == "file-exists":
        path = det["path"].replace("<slice>", cfg.slice_name)
        return ("pass", "") if (root / path).is_file() else ("fail", f"{path} missing")

    if kind == "manifest-tracked":
        if manifest is None:
            return "fail", "slice manifest missing"
        tracked = {e["consumer_path"] for e in manifest.get("entries", [])}
        return ("pass", "") if det["path"] in tracked else (
            "fail", f"{det['path']} not tracked")

    if kind == "profile-bound":
        return ("pass", cfg.profile or "") if cfg.profile else (
            "fail", "no profile declared in slice config")

    if kind == "codeowners-covers":
        return _codeowners(root, det.get("patterns") or [])

    if kind == "attest":
        name = det["attestation"]
        return ("attested", name) if cfg.attestations.get(name) else (
            "fail", f"attestation {name!r} not recorded")

    if kind == "tool":
        tool = root / det["path"]
        if not tool.is_file():
            return "skipped", f"tool absent: {det['path']}"
        proc = subprocess.run([str(tool), *det.get("args", [])],
                              cwd=consumer_root, capture_output=True)
        return ("pass", "") if proc.returncode == 0 else (
            "fail", f"tool exited {proc.returncode}")

    if kind == "pipeline-wired":
        return _pipeline_wired(det, cfg, platform, pipelines)

    if kind == "paths-lockstep":
        return _paths_lockstep(det, cfg, platform, pipelines, manifest)

    return "error", f"unknown detector kind {kind!r}"


def _codeowners(root: Path, patterns: list[str]):
    for loc in ("CODEOWNERS", ".github/CODEOWNERS", "docs/CODEOWNERS"):
        f = root / loc
        if f.is_file():
            owned = [ln.split()[0] for ln in f.read_text().splitlines()
                     if ln.strip() and not ln.startswith("#") and ln.split()]
            missing = [p for p in patterns
                       if not any(p.startswith(o.strip("/").rstrip("*").rstrip("/"))
                                  or match_any(p, [o.strip("/")]) for o in owned)]
            return ("pass", "") if not missing else (
                "fail", f"not covered: {', '.join(missing)}")
    return "fail", "no CODEOWNERS file"


def _pipeline_wired(det, cfg, platform, pipelines):
    component = det["component"]
    hits = [(info, pinned) for info in pipelines
            for wired, pinned in [_wired(info, component, platform, cfg.publisher_repo)]
            if wired]
    if not hits:
        return "fail", f"no pipeline references component {component!r}"
    info, pinned = hits[0]
    if det.get("pinned") and not pinned:
        return "fail", f"{info.path.name}: reference is not pinned to a release tag"
    for event in det.get("events") or []:
        decided = _has_event(info, event, platform)
        if decided is False:
            return "fail", f"{info.path.name}: not wired on {event}"
        if decided is None:
            # Not tree-decidable on this platform: degrade to attestation
            # (conformance spec §3).
            att = f"{event}_enforcement"
            if not cfg.attestations.get(att):
                return "fail", (f"{event} enforcement is not tree-decidable on "
                                f"{platform}; record attestation {att!r}")
            return "attested", att
    if det.get("required_check"):
        att = "required_check_enforced"
        if not cfg.attestations.get(att):
            return "fail", f"record attestation {att!r} (branch protection / policy)"
        return "attested", att
    return "pass", info.path.name


def _paths_lockstep(det, cfg, platform, pipelines, manifest):
    """If the gate pipeline path-filters, the filter must cover every
    consumer_path. No filter (the scaffolded default) is a pass."""
    if manifest is None:
        return "fail", "slice manifest missing"
    gate = [i for i in pipelines
            if _wired(i, det.get("component", "gate"), platform, cfg.publisher_repo)[0]]
    if not gate:
        return "fail", "gate pipeline not found"
    info = gate[0]
    if platform == "github":
        on = info.data.get("on") or info.data.get(True) or {}
        filters = ((on.get("pull_request") or {}).get("paths")
                   if isinstance(on, dict) and isinstance(on.get("pull_request"), dict)
                   else None)
    else:
        filters = (((info.data.get("pr") or {}).get("paths") or {}).get("include")
                   if isinstance(info.data.get("pr"), dict) else None)
    if not filters:
        return "pass", "gate runs unfiltered"
    uncovered = [e["consumer_path"] for e in manifest.get("entries", [])
                 if not match_any(e["consumer_path"], list(filters))]
    return ("pass", "") if not uncovered else (
        "fail", f"filter misses {len(uncovered)} path(s), e.g. {uncovered[0]}")
