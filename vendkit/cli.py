"""The vendkit CLI (cli spec). One entrypoint; uniform conventions.

Exit codes: 0 ok · 1 strict findings · 2 usage/config · 3 refusal ·
>=4 infrastructure. Handlers lazy-import their modules so the gate path
stays stdlib-only (INV-9, DR-0011).
"""

from __future__ import annotations

import argparse
import json
import subprocess
import sys
from pathlib import Path

from .core.util import Refusal, UsageError, VendkitError

DEFAULT_DECL = "vendkit-export.yml"


def _port(args):
    from .ports.base import get_port
    return get_port(getattr(args, "platform", None))


def _decl(args):
    from .core.declaration import ExportDecl
    return ExportDecl.load(args.export_decl)


# -- publisher ---------------------------------------------------------------

def cmd_generate(args) -> int:
    from .core.manifest import (
        build_publisher_manifest, dump_manifest, load_manifest, manifests_equal,
    )
    decl = _decl(args)
    fresh = build_publisher_manifest(decl, args.root)
    port = _port(args)
    if args.check:
        try:
            committed = load_manifest(str(Path(args.root) / decl.manifest_name))
        except UsageError:
            port.emit_error(f"{decl.manifest_name} missing — run generate")
            return 1
        if not manifests_equal(fresh, committed):
            port.emit_error(f"{decl.manifest_name} is stale — run generate")
            port.emit_output("fresh", "false")
            return 1
        port.emit_output("fresh", "true")
        return 0
    dump_manifest(fresh, str(Path(args.root) / decl.manifest_name))
    port.emit_output("entries", str(len(fresh["entries"])))
    return 0


def cmd_release(args) -> int:
    from .core.release import cut
    decl = _decl(args)
    result = cut(
        args.root, decl, bump_kind=args.bump, explicit_version=args.version,
        summary=args.summary, no_migrations_needed=args.no_migrations_needed,
        dry_run=args.dry_run,
    )
    port = _port(args)
    port.emit_output("version", result.version)
    port.emit_output("previous", result.previous or "")
    port.emit_output("surface-delta", f"+{result.added}/-{result.removed}")
    return 0


# -- consumer PR path (stdlib only) --------------------------------------------

def cmd_gate(args) -> int:
    from .core.manifest import discover_manifests, gate_check
    paths = [args.manifest] if args.manifest else discover_manifests(args.consumer_root)
    if not paths:
        raise UsageError("no manifests found under .vendkit/")
    report = gate_check(args.consumer_root, paths)
    port = _port(args)
    for f in report.findings:
        print(f"{f.kind}: {f.consumer_path} [{f.slice_name}] {f.detail}".rstrip())
    port.emit_output("findings", str(len(report.findings)))
    port.emit_output("checked", str(report.checked))
    if args.json:
        print(json.dumps([vars(f) for f in report.findings]))
    if report.findings and args.strict:
        port.emit_error(
            f"gate: {len(report.findings)} finding(s) across "
            f"{len(paths)} manifest(s) — vendored files may only change via sync PRs"
        )
        return 1
    return 0


def cmd_is_newer(args) -> int:
    from .core import versions
    newer = versions.is_newer(args.pinned, args.target,
                              retracted=args.retracted or [])
    _port(args).emit_output("newer", "true" if newer else "false")
    return 0


def cmd_migrations_verify(args) -> int:
    from .core.migrations import load_obligations, verify
    report = verify(args.consumer_root, load_obligations(args.obligations))
    port = _port(args)
    for failure in report.failures:
        print(f"unmet: {failure}")
    port.emit_output("obligations", str(report.checked))
    port.emit_output("unmet", str(len(report.failures)))
    if report.failures:
        port.emit_error(f"{len(report.failures)} migration obligation(s) unmet")
        return 1
    return 0


# -- sync lane ------------------------------------------------------------------

def cmd_sync(args) -> int:
    from .core.materialise import materialise
    decl = _decl(args)
    report = materialise(
        args.publisher_root, args.consumer_root, decl, target=args.target,
        apply=args.apply, reconcile_scope=args.reconcile_scope,
    )
    port = _port(args)
    if not args.porcelain:
        for c in report.updated:
            print(f"updated: {c}")
        for c in report.added:
            print(f"added: {c}")
        for c in report.removed_upstream:
            print(f"removed-upstream: {c} (left on disk — delete in this PR)")
    port.emit_output("changed", "true" if report.changed else "false")
    return 0


def _publisher_tag(publisher_root: str) -> str:
    proc = subprocess.run(
        ["git", "describe", "--tags", "--exact-match", "HEAD"],
        cwd=publisher_root, capture_output=True, text=True,
    )
    if proc.returncode != 0:
        raise VendkitError(
            "publisher checkout is not at a release tag — the sync pipeline "
            "must pin the publisher to refs/tags/vX.Y.Z (INV-6)"
        )
    return proc.stdout.strip()


def cmd_sync_pipeline(args) -> int:
    """Full sync-lane orchestration (sync spec §3): resolve versions, probe,
    apply, advance pins, branch, push, open/update ONE reviewed PR."""
    from .core import migrations, versions
    from .core.manifest import VENDKIT_DIR, load_manifest
    from .core.materialise import materialise
    from .core.sliceconfig import find_slice_config, read_pin
    from .ports.base import RepoRef
    import os

    port = _port(args)
    consumer_root = args.consumer_root
    decl = ExportDecl_load(args)
    cfg = find_slice_config(consumer_root, args.slice)
    if cfg is None:
        raise UsageError(f"no slice config for {args.slice!r} under .vendkit/")

    # PINNED: the manifest's provenance is authoritative for what is vendored;
    # read_pin is the pre-first-sync bootstrap fallback.
    manifest_path = Path(consumer_root) / VENDKIT_DIR / decl.manifest_name
    source = load_manifest(str(manifest_path)).get("source") or {}
    pinned = source.get("release") or read_pin(consumer_root, cfg)
    # TARGET: the release this pipeline's publisher checkout is pinned to.
    target = _publisher_tag(args.publisher_root)

    # Retractions live at the NEWEST release's declaration (releases spec §4):
    # a target's own checkout cannot know it was retracted afterwards. Union
    # the target declaration's list with a best-effort read of the newest one.
    retracted = list(decl.retracted)
    try:
        from .core.watch import _retracted_at_newest
        upstream = port if port.name == "neutral" else __import__(
            "vendkit.ports.base", fromlist=["get_port"]
        ).get_port(cfg.publisher_platform)
        tags = [t.name for t in upstream.list_release_tags(
            RepoRef(cfg.publisher_platform, cfg.publisher_repo))]
        newest = versions.latest(tags, channel="rc")
        if newest:
            retracted += _retracted_at_newest(
                upstream, RepoRef(cfg.publisher_platform, cfg.publisher_repo),
                newest)
    except Exception:
        pass  # advisory steering, not integrity: proceed on target's own list

    if not versions.is_newer(pinned, target, retracted=retracted):
        port.emit_output("update-available", "false")
        port.emit_output("changed", "false")
        return 0
    port.emit_output("update-available", "true")

    # Probe (INV-3: a crash can never masquerade as staleness).
    probe = materialise(args.publisher_root, consumer_root, decl, target=target)
    if not probe.changed:
        port.emit_output("changed", "false")
        return 0

    report = materialise(args.publisher_root, consumer_root, decl,
                         target=target, apply=True,
                         reconcile_scope=args.reconcile_scope)
    port.emit_output("changed", "true")

    # Advance every pin line in lockstep (sync spec §3 step 4).
    for rel in cfg.pin_files:
        f = Path(consumer_root) / rel
        if f.is_file():
            f.write_text(
                f.read_text(encoding="utf-8").replace(
                    f"refs/tags/{pinned}", f"refs/tags/{target}"),
                encoding="utf-8")

    branch = f"vendkit/{cfg.slice_name}/sync-{pinned}-to-{target}"
    applicable, _ = migrations.resolve(
        migrations.load_entries(args.publisher_root), pinned, target, cfg.profile)
    body = _pr_body(cfg.slice_name, pinned, target, report, applicable,
                    load_manifest(str(manifest_path)).get("source") or {})

    def git(*a):
        subprocess.run(["git", "-c", "user.name=vendkit-sync",
                        "-c", "user.email=vendkit-sync@invalid", *a],
                       cwd=consumer_root, check=True)

    git("checkout", "-B", branch)
    git("add", "-A")
    git("commit", "-m", f"sync({cfg.slice_name}): {pinned} -> {target}")
    git("push", "--force", "origin", branch)

    repo = args.consumer_repo or os.environ.get("GITHUB_REPOSITORY") or (
        f"{os.environ.get('SYSTEM_TEAMPROJECT', '')}/"
        f"{os.environ.get('BUILD_REPOSITORY_NAME', '')}"
    )
    pr = port.open_or_update_pr(
        RepoRef(port.name, repo), branch, args.base_branch,
        f"sync({cfg.slice_name}): {pinned} → {target}", body)
    port.emit_output("pr-url", pr.url)
    return 0


def ExportDecl_load(args):
    from .core.declaration import ExportDecl
    decl_path = Path(args.publisher_root) / DEFAULT_DECL
    if getattr(args, "export_decl", None) and args.export_decl != DEFAULT_DECL:
        decl_path = Path(args.export_decl)
    return ExportDecl.load(str(decl_path))


def _pr_body(slice_name, pinned, target, report, applicable, source) -> str:
    lines = [
        f"Sync of slice `{slice_name}`: **{pinned} → {target}**.",
        "",
        f"- updated: {len(report.updated)}",
        f"- added (scope): {len(report.added)}",
        f"- removed upstream (left on disk — delete here): "
        f"{len(report.removed_upstream)}",
        f"- source commit: `{source.get('commit', '')[:12]}`",
    ]
    for c in report.removed_upstream:
        lines.append(f"  - `{c}`")
    if applicable:
        lines += ["", "**Applicable migrations in this window** "
                      "(judgment-bearing; see work items):"]
        lines += [f"- `{m['id']}` ({m['kind']}): {m.get('summary', '')}"
                  for m in applicable]
    lines += ["", "Review per your normal rules; the gate lane re-verifies "
                  "this PR (INV-1). Never auto-merged (INV-10)."]
    return "\n".join(lines)


# -- watch / migrations / conformance ------------------------------------------

def cmd_watch(args) -> int:
    from .core.watch import render_report, watch
    from .ports.base import get_port
    port = _port(args)
    # Upstream ports are keyed by the publisher's platform (ports spec §1) —
    # except under the neutral runner (local runs, tests, fleet audit), where
    # repo coordinates are local paths/URLs resolved by git itself.
    upstream_port = (lambda name: port) if port.name == "neutral" else get_port
    report = watch(args.consumer_root, upstream_port,
                   slice_name=args.slice, dry_run=args.dry_run)
    markdown = render_report(report)
    port.emit_summary(markdown)
    port.emit_output("findings", str(len(report.actionable)))
    if args.json:
        print(json.dumps([vars(f) for f in report.findings]))
    if not args.dry_run and not args.no_handoff:
        for f in report.actionable:
            cfgs = {c.slice_name: c for c in _slice_configs(args.consumer_root)}
            cfg = cfgs[f.slice_name]
            key = cfg.handoff_dedup_key
            if f.kind != "update-available":
                key += "-integrity"
            title = (f"vendkit({f.slice_name}): update available "
                     f"{f.pinned} → {f.latest}"
                     if f.kind == "update-available"
                     else f"vendkit({f.slice_name}): {f.kind}")
            item = port.upsert_work_item(key, title, markdown, None)
            port.emit_output(f"item-{f.slice_name}", item.url)
    return 0


def _slice_configs(consumer_root):
    from .core.sliceconfig import discover_slice_configs
    return discover_slice_configs(consumer_root)


def cmd_migrations(args) -> int:
    from .core.migrations import load_entries, resolve
    applicable, obligations = resolve(
        load_entries(args.publisher_root), args.pinned, args.target,
        args.profile)
    port = _port(args)
    port.emit_output("count", str(len(applicable)))
    port.emit_output("ids", ",".join(m["id"] for m in applicable))
    print(json.dumps({"applicable": [
        {k: v for k, v in m.items() if not k.startswith("_")}
        for m in applicable
    ], "obligations": obligations}, indent=2))
    return 0


def cmd_conformance(args) -> int:
    from .core.conformance import CORE_RULES, evaluate, load_rules
    from .core.sliceconfig import find_slice_config
    from .ports.base import detect
    cfg = find_slice_config(args.consumer_root, args.slice)
    if cfg is None:
        raise UsageError(f"no slice config for {args.slice!r} under .vendkit/")
    rule_files = [str(CORE_RULES)]
    if args.rules:
        rule_files.append(args.rules)
    platform = args.platform or (
        "github" if cfg.publisher_platform == "github" else "ado")
    if platform == "neutral":
        platform = cfg.publisher_platform
    report = evaluate(args.consumer_root, cfg, platform, load_rules(rule_files))
    port = _port(args)
    for r in report.results:
        print(f"{r.status:<9} {r.rule_id:<28} {r.detail}".rstrip())
    port.emit_output("gap-count", str(len(report.gaps)))
    if args.json:
        print(json.dumps([vars(r) for r in report.results]))
    if report.gaps and args.strict:
        port.emit_error(f"{len(report.gaps)} conformance gap(s)")
        return 1
    return 0


def cmd_onboard(args) -> int:
    from .core.onboard import onboard
    decl = ExportDecl_load(args)
    result = onboard(
        args.publisher_root, args.consumer_root, decl,
        platform=args.target_platform, version=args.version,
        profile=args.profile, mode=args.mode, base_branch=args.base_branch,
        pr_token_secret=args.pr_token_secret,
    )
    for path in result.written:
        print(f"wrote: {path}")
    print(f"vendored: {result.vendored} file(s)")
    print()
    print(result.manual_steps)
    return 0


# -- parser ----------------------------------------------------------------------

def build_parser() -> argparse.ArgumentParser:
    p = argparse.ArgumentParser(prog="vendkit", description=__doc__)
    p.add_argument("--platform", choices=["github", "ado", "neutral"],
                   help="port override (default: auto-detect)")
    sub = p.add_subparsers(dest="command", required=True)

    def common(sp, decl=False, consumer=False, publisher=False):
        if decl:
            sp.add_argument("--export-decl", default=DEFAULT_DECL)
        if consumer:
            sp.add_argument("--consumer-root", default=".")
        if publisher:
            sp.add_argument("--publisher-root", default=".")
        sp.add_argument("--json", action="store_true")

    sp = sub.add_parser("generate", help="build/verify the publisher manifest")
    sp.add_argument("--check", action="store_true")
    sp.add_argument("--root", default=".")
    common(sp, decl=True)
    sp.set_defaults(fn=cmd_generate)

    sp = sub.add_parser("gate", help="verify vendored slices (consumer PR path)")
    sp.add_argument("--strict", action="store_true")
    sp.add_argument("--all", action="store_true", default=True)
    sp.add_argument("--manifest")
    common(sp, consumer=True)
    sp.set_defaults(fn=cmd_gate)

    sp = sub.add_parser("sync", help="materialise a target release (low-level)")
    sp.add_argument("--check", dest="apply", action="store_false", default=False)
    sp.add_argument("--apply", dest="apply", action="store_true")
    sp.add_argument("--target", required=True)
    sp.add_argument("--reconcile-scope", action="store_true")
    sp.add_argument("--porcelain", action="store_true")
    common(sp, decl=True, consumer=True, publisher=True)
    sp.set_defaults(fn=cmd_sync)

    sp = sub.add_parser("sync-pipeline",
                        help="full sync lane: probe, apply, pins, branch, PR")
    sp.add_argument("--slice", required=True)
    sp.add_argument("--base-branch", default="main")
    sp.add_argument("--consumer-repo")
    sp.add_argument("--reconcile-scope", action="store_true", default=True)
    common(sp, decl=True, consumer=True, publisher=True)
    sp.set_defaults(fn=cmd_sync_pipeline)

    sp = sub.add_parser("is-newer", help="pure version compare")
    sp.add_argument("--pinned", required=True)
    sp.add_argument("--target", required=True)
    sp.add_argument("--retracted", action="append", default=[])
    common(sp)
    sp.set_defaults(fn=cmd_is_newer)

    sp = sub.add_parser("release", help="cut an immutable release tag")
    sp.add_argument("--bump", choices=["patch", "minor", "major"])
    sp.add_argument("--version")
    sp.add_argument("--summary", default="")
    sp.add_argument("--no-migrations-needed", action="store_true")
    sp.add_argument("--dry-run", action="store_true")
    sp.add_argument("--root", default=".")
    common(sp, decl=True)
    sp.set_defaults(fn=cmd_release)

    sp = sub.add_parser("watch", help="detect new publisher releases")
    sp.add_argument("--slice")
    sp.add_argument("--dry-run", action="store_true")
    sp.add_argument("--no-handoff", action="store_true")
    common(sp, consumer=True)
    sp.set_defaults(fn=cmd_watch)

    sp = sub.add_parser("migrations", help="resolve the migration window")
    sp.add_argument("--pinned", required=True)
    sp.add_argument("--target", required=True)
    sp.add_argument("--profile")
    common(sp, publisher=True)
    sp.set_defaults(fn=cmd_migrations)

    sp = sub.add_parser("migrations-verify",
                        help="deterministic obligation check (consumer PR path)")
    sp.add_argument("--obligations", required=True,
                    help="JSON document, or @path to a file")
    common(sp, consumer=True)
    sp.set_defaults(fn=cmd_migrations_verify)

    sp = sub.add_parser("conformance", help="consumer adoption check")
    sp.add_argument("--slice", required=True)
    sp.add_argument("--strict", action="store_true")
    sp.add_argument("--rules", help="additional publisher rule spec")
    common(sp, consumer=True)
    sp.set_defaults(fn=cmd_conformance)

    sp = sub.add_parser("onboard", help="scaffold a consumer (run from publisher)")
    sp.add_argument("--target-platform", "--platform-target",
                    dest="target_platform", required=True,
                    choices=["github", "ado"])
    sp.add_argument("--version", required=True)
    sp.add_argument("--profile")
    sp.add_argument("--mode", choices=["primary", "additive"], default="primary")
    sp.add_argument("--base-branch", default="main")
    sp.add_argument("--pr-token-secret", default="VENDKIT_PR_TOKEN")
    common(sp, decl=True, consumer=True, publisher=True)
    sp.set_defaults(fn=cmd_onboard)

    return p


def main(argv: list[str] | None = None) -> int:
    args = build_parser().parse_args(argv)
    try:
        return args.fn(args)
    except Refusal as exc:
        print(f"refused={exc.reason}")
        print(f"REFUSED: {exc}", file=sys.stderr)
        return exc.exit_code
    except UsageError as exc:
        print(f"usage error: {exc}", file=sys.stderr)
        return exc.exit_code
    except VendkitError as exc:
        print(f"error: {exc}", file=sys.stderr)
        return exc.exit_code


if __name__ == "__main__":
    sys.exit(main())
