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


def _ci(args):
    from .ci import get_surface
    return get_surface(getattr(args, "platform", None))


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
    port = _ci(args)
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
    port = _ci(args)
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
    port = _ci(args)
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


def cmd_migrations_verify(args) -> int:
    from .core.migrations import load_obligations, verify
    report = verify(args.consumer_root, load_obligations(args.obligations))
    port = _ci(args)
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
    decl = ExportDecl_load(args)  # declaration lives in the publisher checkout
    report = materialise(
        args.publisher_root, args.consumer_root, decl, target=args.target,
        apply=args.apply, reconcile_scope=args.reconcile_scope,
    )
    port = _ci(args)
    if not args.porcelain:
        for c in report.updated:
            print(f"updated: {c}")
        for c in report.added:
            print(f"added: {c}")
        for c in report.removed_upstream:
            print(f"removed-upstream: {c} (left on disk — delete in this PR)")
        for c in report.seeded:
            print(f"seeded: {c} (scaffold-once; yours to change from now on)")
        for c in report.seed_retired:
            print(f"seed-retired: {c} (template gone upstream; file is yours, nothing to delete)")
        for c in report.template_updated:
            print(f"template-updated: {c} (informational; your copy is untouched)")
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
    apply, advance pins, branch, push, and hand ONE reviewed-PR intent to
    the configured PR handler (DR-0014)."""
    from .core import handlers, migrations, upstream, versions
    from .core.manifest import VENDKIT_DIR, load_manifest
    from .core.materialise import materialise
    from .core.sliceconfig import find_slice_config, read_pin

    ci_out = _ci(args)
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
    # the target declaration's list with a best-effort read of the newest one
    # — over the git protocol, no vendor API (DR-0015).
    retracted = list(decl.retracted)
    try:
        from .core.watch import retracted_at_newest
        url = upstream.clone_url(cfg.publisher_scm, cfg.publisher_repo)
        tags = [t.name for t in upstream.list_release_tags(url)]
        newest = versions.latest(tags, channel="rc")
        if newest:
            retracted += retracted_at_newest(url, newest)
    except Exception:
        pass  # advisory steering, not integrity: proceed on target's own list

    if not versions.is_newer(pinned, target, retracted=retracted):
        ci_out.emit_output("update-available", "false")
        ci_out.emit_output("changed", "false")
        return 0
    ci_out.emit_output("update-available", "true")

    # Probe (INV-3: a crash can never masquerade as staleness).
    probe = materialise(args.publisher_root, consumer_root, decl, target=target)
    if not probe.changed:
        ci_out.emit_output("changed", "false")
        return 0

    report = materialise(args.publisher_root, consumer_root, decl,
                         target=target, apply=True,
                         reconcile_scope=args.reconcile_scope)
    ci_out.emit_output("changed", "true")

    # Advance every pin line in lockstep (sync spec §3 step 4).
    # Under ci: none there are no pin files — provenance is the pin.
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
                    load_manifest(str(manifest_path)).get("source") or {},
                    seed_notes=cfg.seed_notes)

    def git(*a):
        subprocess.run(["git", "-c", "user.name=vendkit-sync",
                        "-c", "user.email=vendkit-sync@invalid", *a],
                       cwd=consumer_root, check=True)

    git("checkout", "-B", branch)
    git("add", "-A")
    git("commit", "-m", f"sync({cfg.slice_name}): {pinned} -> {target}")
    git("push", "--force", "origin", branch)

    # PR delivery is a handler concern (DR-0014): the engine composes the
    # intent; the handler owns the vendor API. The deterministic branch name
    # is the idempotency key the handler must honour (protocol spec §3).
    intent = {
        "head_branch": branch,
        "base_branch": args.base_branch,
        "title": f"sync({cfg.slice_name}): {pinned} → {target}",
        "body_md": body,
        "slice": cfg.slice_name,
    }
    if args.consumer_repo:
        intent["repo"] = args.consumer_repo
    command = handlers.resolve("pr", cfg)
    if command is None:
        # Unwired (e.g. ci: none, fully manual): the branch is pushed and
        # the intent is printed — the human delivers the PR themselves.
        ci_out.emit_output("pr-delivered", "false")
        ci_out.emit_output("pr-intent", json.dumps(intent, sort_keys=True))
        return 0
    facts = handlers.invoke(command, "pr", intent, cwd=consumer_root)
    ci_out.emit_output("pr-delivered", "true")
    ci_out.emit_output("pr-url", facts.get("url", ""))
    return 0


def ExportDecl_load(args):
    from .core.declaration import ExportDecl
    decl_path = Path(args.publisher_root) / DEFAULT_DECL
    if getattr(args, "export_decl", None) and args.export_decl != DEFAULT_DECL:
        decl_path = Path(args.export_decl)
    return ExportDecl.load(str(decl_path))


def _pr_body(slice_name, pinned, target, report, applicable, source,
             seed_notes="informational") -> str:
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
    if report.seeded:
        lines += ["", "**Seeded in this PR** (scaffold-once — yours to change "
                      "from now on, the gate never checks them):"]
        lines += [f"- `{c}`" for c in report.seeded]
    if report.seed_retired:
        lines += ["", "**Seed templates retired upstream** (your copies are "
                      "unaffected; they simply stop being tracked):"]
        lines += [f"- `{c}`" for c in report.seed_retired]
    if seed_notes == "informational" and report.template_updated:
        lines += ["", "**Seeded files whose upstream template changed** — "
                      "your copies are yours and were not touched; review the "
                      "publisher's template manually if interested:"]
        lines += [f"- `{c}`" for c in report.template_updated]
    if applicable:
        lines += ["", "**Applicable migrations in this window** "
                      "(judgment-bearing; see work items):"]
        lines += [f"- `{m['id']}` ({m['kind']}): {m.get('summary', '')}"
                  for m in applicable]
    lines += ["", "Review per your normal rules; the gate lane re-verifies "
                  "this PR (INV-1). Never auto-merged (INV-10)."]
    return "\n".join(lines)


# -- human tier ------------------------------------------------------------------
# Compositions over the machine tier — never a parallel code path (cli spec).
# Output formatting here is exempt from the key=value stability promise.

def _slice_or_only(args):
    """--slice, or the sole configured slice; ambiguity is a usage error."""
    from .core.sliceconfig import discover_slice_configs
    configs = discover_slice_configs(args.consumer_root)
    if not configs:
        raise UsageError("no slice configs under .vendkit/ — run `vendkit init`")
    if getattr(args, "slice", None):
        hits = [c for c in configs if c.slice_name == args.slice]
        if not hits:
            raise UsageError(f"no slice config for {args.slice!r}")
        return hits[0]
    if len(configs) == 1:
        return configs[0]
    raise UsageError(
        f"{len(configs)} slices configured — pass --slice "
        f"({', '.join(c.slice_name for c in configs)})")


def _pinned_release(consumer_root: str, cfg) -> str:
    from .core.manifest import VENDKIT_DIR, load_manifest
    from .core.sliceconfig import read_pin
    mpath = Path(consumer_root) / VENDKIT_DIR / f"{cfg.slice_name}-manifest.json"
    if mpath.is_file():
        release = (load_manifest(str(mpath)).get("source") or {}).get("release")
        if release:
            return release
    return read_pin(consumer_root, cfg)


def _latest_release(cfg) -> str | None:
    from .core import upstream, versions
    from .core.watch import retracted_at_newest
    url = upstream.clone_url(cfg.publisher_scm, cfg.publisher_repo)
    names = [t.name for t in upstream.list_release_tags(url)]
    newest = versions.latest(names, channel="rc")
    retracted = retracted_at_newest(url, newest) if newest else []
    return versions.latest(names, channel=cfg.channel, retracted=retracted)


def cmd_status(args) -> int:
    """The human entry point: where is every slice, and does anything need
    attention?"""
    from .core import versions
    from .core.manifest import VENDKIT_DIR, gate_check
    from .core.sliceconfig import discover_slice_configs
    configs = discover_slice_configs(args.consumer_root)
    if args.slice:
        configs = [c for c in configs if c.slice_name == args.slice]
        if not configs:
            raise UsageError(f"no slice config for {args.slice!r}")
    if not configs:
        raise UsageError("no slice configs under .vendkit/ — run `vendkit init`")
    rows = []
    for cfg in configs:
        row = {"slice": cfg.slice_name, "ci": cfg.ci, "scm": cfg.scm,
               "profile": cfg.profile}
        try:
            row["pinned"] = _pinned_release(args.consumer_root, cfg)
        except VendkitError as exc:
            row["pinned"], row["pin_error"] = None, str(exc)
        try:
            row["latest"] = _latest_release(cfg)
        except VendkitError as exc:
            row["latest"], row["latest_error"] = None, str(exc)
        if row.get("pinned") and row.get("latest"):
            row["update"] = (versions.require(row["latest"])
                             > versions.require(row["pinned"]))
            row["bump"] = (versions.classify_bump(row["pinned"], row["latest"])
                           if row["update"] else "")
        mpath = Path(args.consumer_root) / VENDKIT_DIR / f"{cfg.slice_name}-manifest.json"
        row["drift"] = (len(gate_check(args.consumer_root, [str(mpath)]).findings)
                        if mpath.is_file() else None)
        rows.append(row)
    if args.json:
        print(json.dumps(rows, indent=2))
        return 0
    for r in rows:
        line = f"{r['slice']:<12} pinned {r.get('pinned') or '?':<10}"
        if r.get("latest"):
            line += f" latest {r['latest']:<10}"
            line += (f" UPDATE AVAILABLE ({r['bump']})" if r.get("update")
                     else " up to date")
        else:
            line += " latest unknown"
        if r.get("drift"):
            line += f"  DRIFT: {r['drift']} finding(s) — run `vendkit gate`"
        elif r.get("drift") == 0:
            line += "  clean"
        line += f"  [ci: {r['ci']}]"
        print(line)
        for key in ("pin_error", "latest_error"):
            if r.get(key):
                print(f"  ! {r[key]}")
    return 0


def _fetched_publisher(cfg, target: str):
    """TemporaryDirectory context with the publisher cloned at `target`."""
    import tempfile

    from .core import upstream
    url = upstream.clone_url(cfg.publisher_scm, cfg.publisher_repo)
    tmp = tempfile.TemporaryDirectory(prefix="vendkit-publisher-")
    try:
        upstream.fetch_publisher(url, target, tmp.name)
    except Exception:
        tmp.cleanup()
        raise
    return tmp


def cmd_diff(args) -> int:
    """What would `update` change? Unified diff of every file apply would
    write, against a throwaway checkout of the target release."""
    import difflib

    from .core.declaration import ExportDecl
    from .core.materialise import preview
    cfg = _slice_or_only(args)
    target = args.target or _latest_release(cfg)
    if target is None:
        raise UsageError("publisher has no qualifying releases")
    pinned = _pinned_release(args.consumer_root, cfg)
    if not args.target and target == pinned:
        print(f"{cfg.slice_name}: up to date at {pinned}")
        return 0
    with _fetched_publisher(cfg, target) as pub:
        decl = ExportDecl.load(str(Path(pub) / DEFAULT_DECL))
        changes = preview(pub, args.consumer_root, decl)
        print(f"# {cfg.slice_name}: {pinned} → {target} "
              f"({len(changes)} file(s) would change)")
        for cpath, old, new in changes:
            try:
                old_lines = (old or b"").decode("utf-8").splitlines(keepends=True)
                new_lines = new.decode("utf-8").splitlines(keepends=True)
            except UnicodeDecodeError:
                print(f"Binary file {cpath} differs")
                continue
            sys.stdout.writelines(difflib.unified_diff(
                old_lines, new_lines,
                fromfile=f"a/{cpath}" + ("" if old is not None else " (new file)"),
                tofile=f"b/{cpath}"))
    return 0


def cmd_update(args) -> int:
    """The whole upgrade, human-invoked. --local (default): apply to the
    working tree + advance pins, you review and commit. --pr: the full sync
    lane against a fetched checkout (composition over sync-pipeline).

    Note: the human tier runs the INSTALLED engine against the fetched
    target tree — a documented INV-6 relaxation guarded by schema-version
    gating; the CI sync lane preserves INV-6 exactly."""
    import argparse as _argparse

    from .core import versions
    from .core.declaration import ExportDecl
    from .core.materialise import materialise
    from .core.watch import retracted_at_newest
    cfg = _slice_or_only(args)
    pinned = _pinned_release(args.consumer_root, cfg)
    target = args.target or _latest_release(cfg)
    if target is None:
        raise UsageError("publisher has no qualifying releases")
    if target == pinned:
        print(f"{cfg.slice_name}: already at {target}")
        return 0
    with _fetched_publisher(cfg, target) as pub:
        if args.pr:
            ns = _argparse.Namespace(
                slice=cfg.slice_name, base_branch=args.base_branch,
                consumer_repo=None, reconcile_scope=True,
                consumer_root=args.consumer_root, publisher_root=pub,
                export_decl=DEFAULT_DECL, json=False,
                platform=getattr(args, "platform", None))
            return cmd_sync_pipeline(ns)
        decl = ExportDecl.load(str(Path(pub) / DEFAULT_DECL))
        retracted = list(decl.retracted)
        try:
            from .core import upstream
            url = upstream.clone_url(cfg.publisher_scm, cfg.publisher_repo)
            newest = versions.latest(
                [t.name for t in upstream.list_release_tags(url)], channel="rc")
            if newest:
                retracted += retracted_at_newest(url, newest)
        except Exception:
            pass
        if not versions.is_newer(pinned, target, retracted=retracted):
            print(f"{cfg.slice_name}: {target} is not newer than {pinned}")
            return 0
        report = materialise(pub, args.consumer_root, decl, target=target,
                             apply=True, reconcile_scope=True)
        for rel in cfg.pin_files:
            f = Path(args.consumer_root) / rel
            if f.is_file():
                f.write_text(f.read_text(encoding="utf-8").replace(
                    f"refs/tags/{pinned}", f"refs/tags/{target}"),
                    encoding="utf-8")
        for label, paths in (
            ("updated", report.updated), ("added", report.added),
            ("seeded", report.seeded),
            ("removed upstream (delete when you commit)", report.removed_upstream),
            ("seed retired (file is yours)", report.seed_retired),
            ("template updated (informational)", report.template_updated),
        ):
            for c in paths:
                print(f"{label}: {c}")
        print(f"\n{cfg.slice_name}: {pinned} → {target} applied to the working "
              "tree (manifest + pins advanced). Review and commit; the gate "
              "re-verifies your PR (INV-1).")
    return 0


_EXPLANATIONS = {
    # gate findings
    "changed": "A vendored file's content or exec bit differs from the "
        "manifest. Vendored files change only via sync PRs (INV-10). Fix: "
        "revert the edit (`git checkout -- <path>`); to change it for real, "
        "contribute upstream and let a release deliver it.",
    "removed": "A manifest-tracked file is missing. Restore it, or if the "
        "publisher retired it, a sync PR will drop it from tracking.",
    "collision": "Two slices claim the same consumer path (INV-7). One "
        "publisher must rename or exclude; a prefix-namespace adapter is the "
        "usual fix.",
    # watch findings
    "update-available": "The publisher released something newer than your "
        "pin. Run `vendkit diff` to inspect, `vendkit update` to adopt, or "
        "wait for the scheduled sync PR.",
    "tag-moved": "The pinned tag no longer resolves to the commit your "
        "manifest recorded — possible tampering (INV-5). Sync refuses until "
        "resolved. Contact the publisher; a legitimate fix ships as a NEW "
        "release, never a re-tag.",
    "pin-unreadable": "The pin pattern found no parsable version in "
        "pin.files[0]. Fix the reference line or the pattern in "
        ".vendkit/<slice>.yml.",
    "no-releases": "The publisher has no qualifying release tags on your "
        "channel. Benign for new publishers; check watch.channel otherwise.",
    # refusals
    "retracted": "The target release was retracted by the publisher — do "
        "not adopt it. Wait for (or request) the fixed, newer release.",
    "stale-manifest": "The committed publisher manifest does not match the "
        "tree. Run `vendkit generate`, commit, and cut the release again.",
    "tag-exists": "That version is already released; tags are immutable "
        "(INV-5). Pick the next version.",
    "bump-too-small": "The surface delta demands a bigger version bump "
        "(removals ⇒ MAJOR, additions ⇒ MINOR).",
    "migration-missing": "The release removes exported files but ships no "
        "migrations/ entry for this version. Add one, or record an override "
        "with --no-migrations-needed.",
    "not-newer": "The requested version does not exceed the latest release.",
    # sync report classes
    "removed-upstream": "The target release no longer exports this file. "
        "Sync left it on disk; delete it in the sync PR under review "
        "(INV-4: nothing is auto-deleted).",
    "seeded": "A scaffold-once template landed because the path did not "
        "exist. It is yours now: edit or delete freely (DR-0013).",
    "seed-retired": "The publisher retired a seed template. Your copy is "
        "untouched; it simply stops being tracked.",
    "template-updated": "The upstream template behind one of your seeded "
        "files changed. Informational only — your copy was not touched.",
    # conformance statuses
    "attested": "The rule depends on a platform fact that is not decidable "
        "from the tree; your slice config asserts it. Verify via "
        "`conformance --verify-attestations` with a fact-verify handler.",
    "waived": "You waived this (waivable) rule with a recorded reason in "
        "the slice config.",
    "skipped": "Not applicable in this configuration (e.g. pipeline rules "
        "under ci: none — manual mode forfeits automated enforcement).",
}


def cmd_explain(args) -> int:
    if args.topic in (None, "list"):
        for key in sorted(_EXPLANATIONS):
            print(key)
        return 0
    text = _EXPLANATIONS.get(args.topic)
    if text is None:
        raise UsageError(
            f"unknown topic {args.topic!r} — `vendkit explain list`")
    print(f"{args.topic}: {text}")
    return 0


# -- watch / migrations / conformance ------------------------------------------

def cmd_watch(args) -> int:
    """Detection is core; delivery is the handoff handler's (DR-0014).
    Watch itself never talks to a ticket system."""
    from .core import handlers
    from .core.watch import render_report, watch
    ci_out = _ci(args)
    report = watch(args.consumer_root,
                   slice_name=args.slice, dry_run=args.dry_run)
    markdown = render_report(report)
    ci_out.emit_summary(markdown)
    ci_out.emit_output("findings", str(len(report.actionable)))
    if args.json:
        print(json.dumps([vars(f) for f in report.findings]))
    if args.dry_run or args.no_handoff or not report.actionable:
        return 0
    cfgs = {c.slice_name: c for c in _slice_configs(args.consumer_root)}
    unwired = False
    for f in report.actionable:
        cfg = cfgs[f.slice_name]
        command = handlers.resolve("handoff", cfg)
        if command is None:
            unwired = True
            continue  # report-only: findings are on stdout/summary already
        key = cfg.handoff_dedup_key
        if f.kind != "update-available":
            key += "-integrity"  # integrity findings never share the
            #                      update item (release-watch spec §3)
        title = (f"vendkit({f.slice_name}): update available "
                 f"{f.pinned} → {f.latest}"
                 if f.kind == "update-available"
                 else f"vendkit({f.slice_name}): {f.kind}")
        facts = handlers.invoke(
            command, "handoff",
            {"dedup_key": key, "title": title, "body_md": markdown,
             "slice": f.slice_name},
            cwd=args.consumer_root)
        ci_out.emit_output(f"item-{f.slice_name}", facts.get("url", ""))
    ci_out.emit_output("handoff", "unwired" if unwired else "delivered")
    return 0


def _slice_configs(consumer_root):
    from .core.sliceconfig import discover_slice_configs
    return discover_slice_configs(consumer_root)


def cmd_migrations(args) -> int:
    from .core.migrations import load_entries, resolve
    applicable, obligations = resolve(
        load_entries(args.publisher_root), args.pinned, args.target,
        args.profile)
    port = _ci(args)
    port.emit_output("count", str(len(applicable)))
    port.emit_output("ids", ",".join(m["id"] for m in applicable))
    print(json.dumps({"applicable": [
        {k: v for k, v in m.items() if not k.startswith("_")}
        for m in applicable
    ], "obligations": obligations}, indent=2))
    return 0


def cmd_conformance(args) -> int:
    from .core import handlers
    from .core.conformance import CORE_RULES, evaluate, load_rules
    from .core.sliceconfig import find_slice_config
    cfg = find_slice_config(args.consumer_root, args.slice)
    if cfg is None:
        raise UsageError(f"no slice config for {args.slice!r} under .vendkit/")
    rule_files = [str(CORE_RULES)]
    if args.rules:
        rule_files.append(args.rules)
    report = evaluate(args.consumer_root, cfg, load_rules(rule_files))
    if args.verify_attestations:
        # Promote attested → pass via the fact-verify handler; a false
        # verdict is a fail; unknown stays attested (conformance spec §4).
        command = handlers.resolve("fact-verify", cfg)
        if command is None:
            raise UsageError(
                "--verify-attestations needs a fact-verify handler "
                "(handlers.fact-verify in the slice config)")
        for r in report.results:
            if r.status != "attested":
                continue
            facts = handlers.invoke(
                command, "fact-verify",
                {"fact": r.detail, "slice": cfg.slice_name},
                cwd=args.consumer_root)
            verdict = facts.get("verdict", "unknown")
            if verdict == "true":
                r.status, r.detail = "pass", f"{r.detail} (verified)"
            elif verdict == "false":
                r.status, r.detail = "fail", f"{r.detail} (verification refuted)"
    ci_out = _ci(args)
    for r in report.results:
        print(f"{r.status:<9} {r.rule_id:<28} {r.detail}".rstrip())
    ci_out.emit_output("gap-count", str(len(report.gaps)))
    if args.json:
        print(json.dumps([vars(r) for r in report.results]))
    if report.gaps and args.strict:
        ci_out.emit_error(f"{len(report.gaps)} conformance gap(s)")
        return 1
    return 0


def _infer_scm(consumer_root: str) -> str | None:
    """The origin remote discriminates the SCM host (DR-0015): init runs in
    the consumer repo, so the answer is usually already on disk."""
    proc = subprocess.run(
        ["git", "-C", consumer_root, "remote", "get-url", "origin"],
        capture_output=True, text=True,
    )
    if proc.returncode != 0:
        return None
    url = proc.stdout.strip()
    if "github.com" in url:
        return "github"
    if "dev.azure.com" in url or "visualstudio.com" in url:
        return "azure-repos"
    return None


def _prompt(question: str, choices: list[str] | None = None,
            allow_empty: bool = False) -> str:
    """TTY-only prompt for init's human path; never used non-interactively."""
    hint = f" [{'/'.join(choices)}]" if choices else ""
    while True:
        answer = input(f"{question}{hint}: ").strip()
        if not answer and allow_empty:
            return ""
        if answer and (choices is None or answer in choices):
            return answer


def cmd_init(args) -> int:
    from .core.onboard import onboard
    decl = ExportDecl_load(args)
    interactive = sys.stdin.isatty() and sys.stdout.isatty()
    if not args.ci:
        if not interactive:
            raise UsageError("--ci is required (github-actions|azure-pipelines|none)")
        args.ci = _prompt("CI host ('none' = fully manual orchestration)",
                          ["github-actions", "azure-pipelines", "none"])
    if not args.version:
        if not interactive:
            raise UsageError("--version is required (the release to pin)")
        args.version = _prompt("Publisher release to pin (e.g. v1.2.3)")
    if not args.profile and decl.profiles and interactive:
        args.profile = _prompt(
            "Profile", sorted(decl.profiles), allow_empty=True) or None
    scm = args.scm or _infer_scm(args.consumer_root)
    if scm is None:
        if interactive:
            scm = _prompt("SCM host (no origin remote to infer from)",
                          ["github", "azure-repos"])
        else:
            raise UsageError(
                "cannot infer the SCM host from the consumer's origin remote — "
                "pass --scm github|azure-repos")
    result = onboard(
        args.publisher_root, args.consumer_root, decl,
        ci=args.ci, scm=scm, version=args.version,
        profile=args.profile, mode=args.mode, base_branch=args.base_branch,
        pr_token_secret=args.pr_token_secret, codeowners=args.codeowners,
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
    p.add_argument("--platform",
                   choices=["github-actions", "azure-pipelines", "neutral"],
                   help="CI output surface override (default: auto-detect)")
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
    sp.add_argument("--verify-attestations", action="store_true",
                    help="promote attested facts via the fact-verify handler")
    common(sp, consumer=True)
    sp.set_defaults(fn=cmd_conformance)

    sp = sub.add_parser("init", aliases=["onboard"],
                        help="scaffold a consumer (run from publisher checkout)")
    sp.add_argument("--ci",
                    choices=["github-actions", "azure-pipelines", "none"],
                    help="the consumer's CI host; 'none' = fully manual "
                         "orchestration, no pipelines scaffolded "
                         "(prompted on a TTY when omitted)")
    sp.add_argument("--scm", choices=["github", "azure-repos"],
                    help="the consumer's SCM host (default: inferred from "
                         "the origin remote)")
    sp.add_argument("--version",
                    help="publisher release to pin (prompted on a TTY)")
    sp.add_argument("--profile")
    sp.add_argument("--mode", choices=["primary", "additive"], default="primary")
    sp.add_argument("--base-branch", default="main")
    sp.add_argument("--pr-token-secret", default="VENDKIT_PR_TOKEN")
    sp.add_argument("--codeowners", metavar="OWNERS",
                    help="opt-in: write a CODEOWNERS stanza covering "
                         ".vendkit/ (GitHub only; Azure Repos uses a "
                         "required-reviewers policy instead)")
    common(sp, decl=True, consumer=True, publisher=True)
    sp.set_defaults(fn=cmd_init)

    # -- human tier (compositions; formatting exempt from output stability) --

    sp = sub.add_parser("status", help="per-slice rollup: pinned vs latest, "
                                       "drift, attention items")
    sp.add_argument("--slice")
    common(sp, consumer=True)
    sp.set_defaults(fn=cmd_status)

    sp = sub.add_parser("diff", help="unified diff of what `update` would "
                                     "change (fetches the target release)")
    sp.add_argument("--slice")
    sp.add_argument("--target", help="release to compare (default: latest)")
    common(sp, consumer=True)
    sp.set_defaults(fn=cmd_diff)

    sp = sub.add_parser("update", help="upgrade a slice: apply locally "
                                       "(default) or run the full PR lane")
    sp.add_argument("--slice")
    sp.add_argument("--target", help="release to adopt (default: latest)")
    mode = sp.add_mutually_exclusive_group()
    mode.add_argument("--local", dest="pr", action="store_false", default=False,
                      help="apply to the working tree; you review and commit")
    mode.add_argument("--pr", dest="pr", action="store_true",
                      help="branch, push, and deliver a PR via the handler")
    sp.add_argument("--base-branch", default="main")
    common(sp, consumer=True)
    sp.set_defaults(fn=cmd_update)

    sp = sub.add_parser("explain", help="what a finding/refusal/status means "
                                        "and the sanctioned fix")
    sp.add_argument("topic", nargs="?", help="e.g. tag-moved, retracted; "
                                             "'list' to enumerate")
    common(sp)
    sp.set_defaults(fn=cmd_explain)

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
