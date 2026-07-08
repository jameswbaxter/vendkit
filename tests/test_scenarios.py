"""Scenario kit (testing.md §2): throwaway publisher/consumer git repos,
CLI end-to-end under the neutral port, no network.
"""

from __future__ import annotations

import json
import os
import stat
import subprocess
import sys
from pathlib import Path

import pytest

REPO = Path(__file__).resolve().parent.parent


def vk(*args, cwd, check=True, env=None):
    full_env = {**os.environ, "VENDKIT_PLATFORM": "neutral",
                "PYTHONPATH": str(REPO), **(env or {})}
    proc = subprocess.run(
        [sys.executable, "-m", "vendkit.cli", *args],
        cwd=cwd, capture_output=True, text=True, env=full_env,
    )
    if check and proc.returncode != 0:
        raise AssertionError(
            f"vendkit {' '.join(args)} -> {proc.returncode}\n"
            f"stdout:\n{proc.stdout}\nstderr:\n{proc.stderr}"
        )
    return proc


def git(*args, cwd):
    subprocess.run(
        ["git", "-c", "user.name=t", "-c", "user.email=t@invalid",
         "-c", "commit.gpgsign=false", *args],
        cwd=cwd, check=True, capture_output=True,
    )


@pytest.fixture
def world(tmp_path):
    """Publisher (with origin) at v0.1.0 + onboarded consumer (with origin)."""
    pub = tmp_path / "publisher"
    (pub / "docs").mkdir(parents=True)
    (pub / "tools").mkdir()
    (pub / "docs" / "standard.md").write_text("# Standard\n\nRule one.\n")
    (pub / "docs" / "guide.md").write_text("# Guide\n")
    tool = pub / "tools" / "check"
    tool.write_text("#!/bin/sh\nexit 0\n")
    tool.chmod(tool.stat().st_mode | stat.S_IXUSR | stat.S_IXGRP | stat.S_IXOTH)
    (pub / "vendkit-export.yml").write_text("""\
schema_version: 1
slice: {name: docs, title: "Design docs"}
publisher: {platform: github, repo: example-org/pub}
include: ["docs/**/*.md", "tools/*"]
exclude: ["**/TEMPLATE.md"]
profiles:
  code-repo: {}
""")
    git("init", "-q", "-b", "main", cwd=pub)
    vk("generate", cwd=pub)
    git("add", "-A", cwd=pub)
    git("commit", "-q", "-m", "init", cwd=pub)
    pub_origin = tmp_path / "publisher-origin.git"
    git("init", "-q", "--bare", str(pub_origin), cwd=tmp_path)
    git("remote", "add", "origin", str(pub_origin), cwd=pub)
    git("push", "-q", "origin", "main", cwd=pub)
    vk("release", "--version", "v0.1.0", cwd=pub)

    con = tmp_path / "consumer"
    con.mkdir()
    (con / "CODEOWNERS").write_text("/.vendkit/ @example-org/owners\n")
    git("init", "-q", "-b", "main", cwd=con)
    vk("onboard", "--target-platform", "github", "--version", "v0.1.0",
       "--profile", "code-repo", "--publisher-root", str(pub),
       "--consumer-root", str(con), cwd=pub)
    # Neutral-port coordinates: the publisher is a local path.
    cfg = con / ".vendkit" / "docs.yml"
    cfg.write_text(cfg.read_text().replace(
        "repo: example-org/pub", f"repo: {pub}"))
    git("add", "-A", cwd=con)
    git("commit", "-q", "-m", "onboard docs slice v0.1.0", cwd=con)
    con_origin = tmp_path / "consumer-origin.git"
    git("init", "-q", "--bare", str(con_origin), cwd=tmp_path)
    git("remote", "add", "origin", str(con_origin), cwd=con)
    git("push", "-q", "origin", "main", cwd=con)
    return pub, con


def _release(pub: Path, version: str, **kw):
    vk("generate", cwd=pub)
    git("add", "-A", cwd=pub)
    git("commit", "-q", "-m", f"prep {version}", cwd=pub)
    git("push", "-q", "origin", "main", cwd=pub)
    vk("release", "--version", version, *kw.get("extra", []), cwd=pub)


# -- gate lane -------------------------------------------------------------------

def test_clean_tree_passes_strict_gate(world):
    _, con = world
    proc = vk("gate", "--strict", cwd=con)
    assert "findings=0" in proc.stdout


def test_hand_edit_fails_strict_advisory_reports(world):
    _, con = world
    f = con / "docs" / "standard.md"
    f.write_text(f.read_text() + "sneaky edit\n")
    proc = vk("gate", "--strict", cwd=con, check=False)
    assert proc.returncode == 1
    assert "changed: docs/standard.md" in proc.stdout
    advisory = vk("gate", cwd=con)          # advisory: reports, exits 0
    assert "findings=1" in advisory.stdout


def test_delete_and_chmod_are_drift(world):
    _, con = world
    (con / "docs" / "guide.md").unlink()
    tool = con / "tools" / "check"
    tool.chmod(tool.stat().st_mode & ~stat.S_IXUSR & ~stat.S_IXGRP & ~stat.S_IXOTH)
    proc = vk("gate", "--strict", cwd=con, check=False)
    assert proc.returncode == 1
    assert "removed: docs/guide.md" in proc.stdout
    assert "changed: tools/check" in proc.stdout and "exec bit" in proc.stdout


def test_crlf_recheckout_does_not_trip_gate(world):
    _, con = world
    f = con / "docs" / "standard.md"
    f.write_bytes(f.read_text().replace("\n", "\r\n").encode())
    vk("gate", "--strict", cwd=con)


def test_collision_between_slices_detected(world):
    _, con = world
    manifest = json.loads((con / ".vendkit" / "docs-manifest.json").read_text())
    rogue = {**manifest, "slice": "rogue",
             "entries": [manifest["entries"][0]]}
    (con / ".vendkit" / "rogue-manifest.json").write_text(json.dumps(rogue))
    proc = vk("gate", "--strict", cwd=con, check=False)
    assert proc.returncode == 1
    assert "collision" in proc.stdout


# -- sync lane --------------------------------------------------------------------

def test_sync_same_version_is_noop(world):
    pub, con = world
    proc = vk("sync-pipeline", "--slice", "docs", "--publisher-root", str(pub),
              "--consumer-root", str(con), cwd=con)
    assert "update-available=false" in proc.stdout


def test_sync_upgrade_composes_with_gate(world, tmp_path):
    """The composition invariant INV-1 plus PR mechanics end-to-end."""
    pub, con = world
    (pub / "docs" / "standard.md").write_text("# Standard\n\nRule one.\nRule two.\n")
    (pub / "docs" / "new.md").write_text("# New\n")
    _release(pub, "v0.2.0")

    journal = tmp_path / "journal.jsonl"
    proc = vk("sync-pipeline", "--slice", "docs", "--publisher-root", str(pub),
              "--consumer-root", str(con), cwd=con,
              env={"VENDKIT_NEUTRAL_JOURNAL": str(journal)})
    assert "update-available=true" in proc.stdout
    assert "changed=true" in proc.stdout

    # One PR, deterministic branch, via the port.
    records = [json.loads(l) for l in journal.read_text().splitlines()]
    prs = [r for r in records if r["op"] == "open_pr"]
    assert len(prs) == 1
    assert prs[0]["head"] == "vendkit/docs/sync-v0.1.0-to-v0.2.0"

    # Content refreshed, addition reconciled, pins advanced in lockstep.
    assert "Rule two." in (con / "docs" / "standard.md").read_text()
    assert (con / "docs" / "new.md").is_file()
    for wf in ("docs-sync", "vendkit-gate", "vendkit-watch"):
        text = (con / ".github" / "workflows" / f"{wf}.yml").read_text()
        assert "refs/tags/v0.2.0" in text, wf

    # Provenance recorded (manifest spec §1).
    manifest = json.loads((con / ".vendkit" / "docs-manifest.json").read_text())
    assert manifest["source"]["release"] == "v0.2.0"
    assert len(manifest["source"]["commit"]) == 40

    # INV-1: the sync output passes the strict gate.
    vk("gate", "--strict", cwd=con)

    # Idempotency: re-running the pipeline finds nothing to do.
    again = vk("sync-pipeline", "--slice", "docs", "--publisher-root", str(pub),
               "--consumer-root", str(con), cwd=con)
    assert "changed=false" in again.stdout


def test_retracted_target_refused(world):
    pub, con = world
    (pub / "docs" / "guide.md").write_text("# Guide v2\n")
    _release(pub, "v0.2.0")
    decl = pub / "vendkit-export.yml"
    decl.write_text(decl.read_text() + "retracted: [v0.2.0]\n")
    _release(pub, "v0.2.1")
    # Publisher checkout sits at v0.2.0 (the retracted one): refuse, exit 3.
    git("checkout", "-q", "v0.2.0", cwd=pub)
    proc = vk("sync-pipeline", "--slice", "docs", "--publisher-root", str(pub),
              "--consumer-root", str(con), cwd=con, check=False)
    assert proc.returncode == 3
    assert "refused=retracted" in proc.stdout


# -- releases ----------------------------------------------------------------------

def test_release_refuses_stale_manifest(world):
    pub, _ = world
    (pub / "docs" / "guide.md").write_text("# Guide changed\n")
    proc = vk("release", "--version", "v0.2.0", cwd=pub, check=False)
    assert proc.returncode == 3
    assert "refused=stale-manifest" in proc.stdout


def test_release_bump_and_migration_gates(world):
    pub, _ = world
    (pub / "docs" / "guide.md").unlink()  # surface removal
    vk("generate", cwd=pub)
    git("add", "-A", cwd=pub)
    git("commit", "-q", "-m", "drop guide", cwd=pub)
    # Removal with only a patch bump: refused.
    proc = vk("release", "--version", "v0.1.1", cwd=pub, check=False)
    assert "refused=bump-too-small" in proc.stdout
    # Major bump but no migration payload: refused.
    proc = vk("release", "--version", "v1.0.0", cwd=pub, check=False)
    assert "refused=migration-missing" in proc.stdout
    # With a payload: allowed.
    (pub / "migrations").mkdir()
    (pub / "migrations" / "drop-guide.yml").write_text("""\
schema_version: 1
id: drop-guide
applies_from: v1.0.0
kind: structural
profiles: ["*"]
summary: "guide.md retired; fold content into standard.md"
rationale: "One doc is enough."
detection: [{glob: "docs/guide.md"}]
instructions: "Merge guide content into standard.md, delete guide.md."
verification:
  must_be_absent: ["docs/guide.md"]
  must_be_present: ["docs/standard.md"]
""")
    git("add", "-A", cwd=pub)
    git("commit", "-q", "-m", "migration payload", cwd=pub)
    vk("release", "--version", "v1.0.0", cwd=pub)


def test_release_tag_exists_refused(world):
    pub, _ = world
    proc = vk("release", "--version", "v0.1.0", cwd=pub, check=False)
    assert proc.returncode == 3
    assert "refused=" in proc.stdout


# -- migrations -----------------------------------------------------------------------

def test_migration_window_and_verify(world):
    pub, con = world
    (pub / "migrations").mkdir()
    (pub / "migrations" / "drop-guide.yml").write_text("""\
schema_version: 1
id: drop-guide
applies_from: v0.2.0
kind: structural
profiles: ["*"]
summary: "guide retired"
rationale: "consolidation"
verification:
  must_be_absent: ["docs/guide.md"]
""")
    proc = vk("migrations", "--pinned", "v0.1.0", "--target", "v0.2.0",
              "--publisher-root", str(pub), cwd=pub)
    doc = json.loads(proc.stdout[proc.stdout.index("{"):])
    assert [m["id"] for m in doc["applicable"]] == ["drop-guide"]
    # Outside the window: nothing applies.
    proc = vk("migrations", "--pinned", "v0.2.0", "--target", "v0.3.0",
              "--publisher-root", str(pub), cwd=pub)
    assert "count=0" in proc.stdout

    obligations = json.dumps(doc["obligations"])
    proc = vk("migrations-verify", "--obligations", obligations,
              "--consumer-root", str(con), cwd=con, check=False)
    assert proc.returncode == 1                      # guide.md still present
    (con / "docs" / "guide.md").unlink()
    git("add", "-A", cwd=con)
    git("commit", "-q", "-m", "apply migration", cwd=con)
    vk("migrations-verify", "--obligations", obligations,
       "--consumer-root", str(con), cwd=con)
    # Zero obligations: green no-op (safe as an always-on required check).
    vk("migrations-verify", "--obligations", "{}",
       "--consumer-root", str(con), cwd=con)


# -- watch --------------------------------------------------------------------------

def test_watch_detects_update_and_dry_run_is_offline(world, tmp_path):
    pub, con = world
    (pub / "docs" / "guide.md").write_text("# Guide v2\n")
    _release(pub, "v0.2.0")
    journal = tmp_path / "journal.jsonl"
    proc = vk("watch", "--no-handoff", "--json", cwd=con,
              env={"VENDKIT_NEUTRAL_JOURNAL": str(journal)})
    assert "findings=1" in proc.stdout
    findings = json.loads(proc.stdout[proc.stdout.index("["):])
    assert findings[0]["kind"] == "update-available"
    assert findings[0]["latest"] == "v0.2.0"
    assert findings[0]["bump"] == "minor"
    # Handoff creates exactly one deduped work item.
    vk("watch", cwd=con, env={"VENDKIT_NEUTRAL_JOURNAL": str(journal)})
    items = [json.loads(l) for l in journal.read_text().splitlines()
             if json.loads(l)["op"] == "upsert_work_item"]
    assert len(items) == 1 and items[0]["dedup_key"] == "vendkit-watch-docs"
    # Dry-run: no findings, exit 0 (PR self-test, no network).
    dry = vk("watch", "--dry-run", cwd=con)
    assert "findings=0" in dry.stdout


def test_watch_detects_tag_moved(world):
    pub, con = world
    # Simulate tag substitution: delete and re-point v0.1.0.
    (pub / "docs" / "guide.md").write_text("# tampered\n")
    vk("generate", cwd=pub)
    git("add", "-A", cwd=pub)
    git("commit", "-q", "-m", "tamper", cwd=pub)
    git("tag", "-f", "-a", "v0.1.0", "-m", "moved", cwd=pub)
    git("push", "-q", "--force", "origin", "refs/tags/v0.1.0", cwd=pub)
    proc = vk("watch", "--no-handoff", "--json", cwd=con)
    findings = json.loads(proc.stdout[proc.stdout.index("["):])
    assert any(f["kind"] == "tag-moved" for f in findings)


# -- conformance ----------------------------------------------------------------------

def test_conformance_reports_and_attestations(world):
    _, con = world
    proc = vk("conformance", "--slice", "docs", cwd=con)
    lines = proc.stdout.splitlines()
    # Wiring rules pass from the scaffolded tree; trust-bootstrap rules fail
    # until attested (onboarding spec §4).
    assert any(l.startswith("pass") and "manifest-committed" in l for l in lines)
    assert any(l.startswith("pass") and "control-plane-owned" in l for l in lines)
    assert any(l.startswith("fail") and "branch-protected" in l for l in lines)
    strict = vk("conformance", "--slice", "docs", "--strict", cwd=con, check=False)
    assert strict.returncode == 1
    # Attest + record the non-tree-decidable facts: strict goes green.
    cfg = con / ".vendkit" / "docs.yml"
    cfg.write_text(cfg.read_text().replace(
        "branch_protection_enabled: false", "branch_protection_enabled: true"
    ).replace(
        "sync_credential_provisioned: false", "sync_credential_provisioned: true"
    ).replace("attestations:", "attestations:\n  required_check_enforced: true"))
    vk("conformance", "--slice", "docs", "--strict", cwd=con)


def test_gate_path_is_stdlib_only(world):
    """INV-9: the gate must run with PyYAML unimportable."""
    _, con = world
    proc = subprocess.run(
        [sys.executable, "-c",
         "import sys; sys.modules['yaml'] = None\n"
         "import vendkit.cli as c; sys.exit(c.main(['gate', '--strict']))"],
        cwd=con, capture_output=True, text=True,
        env={**os.environ, "VENDKIT_PLATFORM": "neutral",
             "PYTHONPATH": str(REPO)},
    )
    assert proc.returncode == 0, proc.stderr
