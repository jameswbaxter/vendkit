"""GitHub port binding (platform-ports spec §2/§3).

REST via stdlib urllib. Every operation is loud on failure (PortError).
"""

from __future__ import annotations

import base64
import os
import sys

from .base import (
    PortError, PrRef, RepoRef, RoutingConfig, TagInfo, WorkItemRef,
    http_json, token_for,
)

API = os.environ.get("VENDKIT_GITHUB_API", "https://api.github.com")


class GitHubPort:
    name = "github"

    # -- 1. release listing -------------------------------------------------

    def list_release_tags(self, repo: RepoRef) -> list[TagInfo]:
        token = self.credential("read_upstream")
        tags: list[TagInfo] = []
        page = 1
        while True:
            data = http_json(
                f"{API}/repos/{repo.repo}/tags?per_page=100&page={page}",
                token=token,
            )
            if not data:
                break
            tags.extend(
                TagInfo(name=t["name"], commit=t["commit"]["sha"]) for t in data
            )
            if len(data) < 100:
                break
            page += 1
        return tags

    def read_file(self, repo: RepoRef, ref: str, path: str) -> bytes:
        token = self.credential("read_upstream")
        data = http_json(
            f"{API}/repos/{repo.repo}/contents/{path}?ref={ref}", token=token
        )
        if data.get("encoding") != "base64":
            raise PortError(f"unexpected contents encoding for {path}@{ref}")
        return base64.b64decode(data["content"])

    # -- 2. pull requests -----------------------------------------------------

    def find_open_pr(self, repo: RepoRef, head_branch: str) -> PrRef | None:
        token = self.credential("open_pr")
        owner = repo.repo.split("/")[0]
        data = http_json(
            f"{API}/repos/{repo.repo}/pulls?state=open&head={owner}:{head_branch}",
            token=token,
        )
        if data:
            return PrRef(url=data[0]["html_url"], number=data[0]["number"],
                         existed=True)
        return None

    def open_or_update_pr(self, repo, head_branch, base_branch, title, body_md):
        # The PR credential must NOT be GITHUB_TOKEN: PRs it opens do not
        # trigger pull_request workflows, so the sync PR would silently skip
        # its own gate (differences ledger #2). credential() enforces.
        token = self.credential("open_pr")
        existing = self.find_open_pr(repo, head_branch)
        if existing:
            http_json(
                f"{API}/repos/{repo.repo}/pulls/{existing.number}",
                method="PATCH", token=token,
                body={"title": title, "body": body_md},
            )
            return existing
        data = http_json(
            f"{API}/repos/{repo.repo}/pulls", method="POST", token=token,
            body={"title": title, "body": body_md,
                  "head": head_branch, "base": base_branch},
        )
        return PrRef(url=data["html_url"], number=data["number"])

    # -- 3. work items (issues) ----------------------------------------------

    def upsert_work_item(self, dedup_key, title, body_md,
                         routing: RoutingConfig | None = None) -> WorkItemRef:
        token = self.credential("work_items")
        repo = RepoRef("github", os.environ["GITHUB_REPOSITORY"])
        labels = [dedup_key, *(routing.labels if routing else [])]
        data = http_json(
            f"{API}/repos/{repo.repo}/issues?state=open&labels={dedup_key}",
            token=token,
        )
        if data:
            issue = data[0]
            http_json(
                f"{API}/repos/{repo.repo}/issues/{issue['number']}/comments",
                method="POST", token=token, body={"body": body_md},
            )
            return WorkItemRef(url=issue["html_url"], id=str(issue["number"]),
                               reused=True)
        data = http_json(
            f"{API}/repos/{repo.repo}/issues", method="POST", token=token,
            body={"title": title, "body": body_md, "labels": labels},
        )
        return WorkItemRef(url=data["html_url"], id=str(data["number"]))

    # -- 4. CI output surface -------------------------------------------------

    def emit_output(self, key: str, value: str) -> None:
        print(f"{key}={value}")  # all ports also emit key=value (spec §2)
        out = os.environ.get("GITHUB_OUTPUT")
        if out:
            with open(out, "a", encoding="utf-8") as fh:
                fh.write(f"{key}={value}\n")

    def emit_summary(self, markdown: str) -> None:
        path = os.environ.get("GITHUB_STEP_SUMMARY")
        if path:
            with open(path, "a", encoding="utf-8") as fh:
                fh.write(markdown + "\n")
        else:
            print(markdown, file=sys.stderr)

    def emit_error(self, message: str) -> None:
        print(f"::error::{message}")

    # -- 5/6. platform facts and credentials -----------------------------------

    def verify_platform_fact(self, fact: str) -> bool | None:
        return None  # API verification: post-1.0 (conformance spec §4)

    def credential(self, purpose: str) -> str | None:
        token = token_for(purpose, "GITHUB_TOKEN", "GH_TOKEN")
        if purpose == "open_pr":
            explicit = token_for(purpose) or os.environ.get("VENDKIT_PR_TOKEN")
            if not explicit:
                raise PortError(
                    "open_pr requires VENDKIT_TOKEN_OPEN_PR or VENDKIT_PR_TOKEN "
                    "(a PAT/App token — GITHUB_TOKEN-opened PRs do not trigger "
                    "workflows, so the sync PR would skip its own gate)"
                )
            return explicit
        if purpose in ("work_items",) and not token:
            raise PortError(f"no credential for {purpose} (set GITHUB_TOKEN)")
        return token
