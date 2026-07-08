"""Neutral port: local runs, tests, fleet audit (platform-ports spec §1).

Release listing and file reads resolve RepoRef.repo as a *local path or git
URL* via git itself, which makes the scenario kit runnable with no network
and no platform (testing spec §2). Work items and PRs are recorded to a JSON
journal (VENDKIT_NEUTRAL_JOURNAL) or stdout — visible, never silent.
"""

from __future__ import annotations

import json
import os
import subprocess
import sys

from .base import PortError, PrRef, RepoRef, RoutingConfig, TagInfo, WorkItemRef


class NeutralPort:
    name = "neutral"

    # -- 1. release listing -------------------------------------------------

    def list_release_tags(self, repo: RepoRef) -> list[TagInfo]:
        # No --refs: annotated tags list the tag OBJECT sha on the bare ref
        # and the peeled commit on the `^{}` line — provenance needs the
        # commit (the other ports peel too: ADO peelTags=true, GitHub's API
        # peels server-side).
        proc = subprocess.run(
            ["git", "ls-remote", "--tags", repo.repo],
            capture_output=True, text=True,
        )
        if proc.returncode != 0:
            raise PortError(
                f"listing tags of {repo.repo} failed: {proc.stderr.strip()}"
            )
        shas: dict[str, str] = {}
        for line in proc.stdout.splitlines():
            sha, _, ref = line.partition("\t")
            if not ref.startswith("refs/tags/"):
                continue
            name = ref[len("refs/tags/"):]
            if name.endswith("^{}"):
                shas[name[:-3]] = sha  # peeled commit wins
            else:
                shas.setdefault(name, sha)
        return [TagInfo(name=n, commit=s) for n, s in sorted(shas.items())]

    def read_file(self, repo: RepoRef, ref: str, path: str) -> bytes:
        proc = subprocess.run(
            ["git", "-C", repo.repo, "show", f"{ref}:{path}"],
            capture_output=True,
        )
        if proc.returncode != 0:
            raise PortError(f"cannot read {path}@{ref} from {repo.repo}")
        return proc.stdout

    # -- 2/3. PRs and work items --------------------------------------------

    def _journal(self, record: dict) -> None:
        path = os.environ.get("VENDKIT_NEUTRAL_JOURNAL")
        line = json.dumps(record, sort_keys=True)
        if path:
            with open(path, "a", encoding="utf-8") as fh:
                fh.write(line + "\n")
        else:
            print(f"[neutral-port] {line}")

    def find_open_pr(self, repo: RepoRef, head_branch: str) -> PrRef | None:
        return None

    def open_or_update_pr(self, repo, head_branch, base_branch, title, body_md):
        self._journal({"op": "open_pr", "repo": repo.repo,
                       "head": head_branch, "base": base_branch, "title": title})
        return PrRef(url=f"neutral://pr/{head_branch}", number=0)

    def upsert_work_item(self, dedup_key, title, body_md,
                         routing: RoutingConfig | None = None) -> WorkItemRef:
        self._journal({"op": "upsert_work_item", "dedup_key": dedup_key,
                       "title": title})
        return WorkItemRef(url=f"neutral://item/{dedup_key}", id=dedup_key)

    # -- 4. CI output surface -----------------------------------------------

    def emit_output(self, key: str, value: str) -> None:
        print(f"{key}={value}")

    def emit_summary(self, markdown: str) -> None:
        print(markdown, file=sys.stderr)

    def emit_error(self, message: str) -> None:
        print(f"ERROR: {message}", file=sys.stderr)

    # -- 5/6. platform facts and credentials ---------------------------------

    def verify_platform_fact(self, fact: str) -> bool | None:
        return None  # cannot verify anything: stay attested

    def credential(self, purpose: str) -> str | None:
        from .base import token_for
        return token_for(purpose)
