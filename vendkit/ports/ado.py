"""Azure DevOps port binding (platform-ports spec §2/§3).

REST 7.1 via stdlib urllib; PAT/Basic auth; ##vso logging commands for the
CI output surface. RepoRef.repo is '<project>/<repository>'.
"""

from __future__ import annotations

import os
import sys

from .base import (
    PortError, PrRef, RepoRef, RoutingConfig, TagInfo, WorkItemRef,
    http_json, token_for,
)

ORG_URL = os.environ.get("VENDKIT_ADO_ORG_URL", "").rstrip("/")


def _org() -> str:
    if not ORG_URL:
        raise PortError("VENDKIT_ADO_ORG_URL is not set (e.g. https://dev.azure.com/example-org)")
    return ORG_URL


def _split(repo: str) -> tuple[str, str]:
    project, _, repository = repo.partition("/")
    if not project or not repository:
        raise PortError(f"ADO repo must be '<project>/<repository>': {repo!r}")
    return project, repository


class AdoPort:
    name = "ado"

    # -- 1. release listing -------------------------------------------------

    def list_release_tags(self, repo: RepoRef) -> list[TagInfo]:
        project, repository = _split(repo.repo)
        token = self.credential("read_upstream")
        data = http_json(
            f"{_org()}/{project}/_apis/git/repositories/{repository}/refs"
            f"?filter=tags/&peelTags=true&api-version=7.1",
            token=token, auth_scheme="Basic",
        )
        tags = []
        for ref in data.get("value", []):
            name = ref.get("name", "")
            if name.startswith("refs/tags/"):
                commit = ref.get("peeledObjectId") or ref.get("objectId", "")
                tags.append(TagInfo(name=name[len("refs/tags/"):], commit=commit))
        return tags

    def read_file(self, repo: RepoRef, ref: str, path: str) -> bytes:
        project, repository = _split(repo.repo)
        token = self.credential("read_upstream")
        import urllib.parse
        version = urllib.parse.quote(ref)
        data = http_json(
            f"{_org()}/{project}/_apis/git/repositories/{repository}/items"
            f"?path={urllib.parse.quote(path)}&versionDescriptor.versionType=tag"
            f"&versionDescriptor.version={version}"
            f"&includeContent=true&api-version=7.1",
            token=token, auth_scheme="Basic",
        )
        return data.get("content", "").encode("utf-8")

    # -- 2. pull requests -----------------------------------------------------

    def find_open_pr(self, repo: RepoRef, head_branch: str) -> PrRef | None:
        project, repository = _split(repo.repo)
        token = self.credential("open_pr")
        data = http_json(
            f"{_org()}/{project}/_apis/git/repositories/{repository}/pullrequests"
            f"?searchCriteria.status=active"
            f"&searchCriteria.sourceRefName=refs/heads/{head_branch}"
            f"&api-version=7.1",
            token=token, auth_scheme="Basic",
        )
        prs = data.get("value", [])
        if prs:
            pr = prs[0]
            url = f"{_org()}/{project}/_git/{repository}/pullrequest/{pr['pullRequestId']}"
            return PrRef(url=url, number=pr["pullRequestId"], existed=True)
        return None

    def open_or_update_pr(self, repo, head_branch, base_branch, title, body_md):
        project, repository = _split(repo.repo)
        token = self.credential("open_pr")
        existing = self.find_open_pr(repo, head_branch)
        if existing:
            http_json(
                f"{_org()}/{project}/_apis/git/repositories/{repository}"
                f"/pullrequests/{existing.number}?api-version=7.1",
                method="PATCH", token=token, auth_scheme="Basic",
                body={"title": title, "description": body_md},
            )
            return existing
        data = http_json(
            f"{_org()}/{project}/_apis/git/repositories/{repository}"
            f"/pullrequests?api-version=7.1",
            method="POST", token=token, auth_scheme="Basic",
            body={
                "sourceRefName": f"refs/heads/{head_branch}",
                "targetRefName": f"refs/heads/{base_branch}",
                "title": title,
                "description": body_md,
            },
        )
        url = f"{_org()}/{project}/_git/{repository}/pullrequest/{data['pullRequestId']}"
        return PrRef(url=url, number=data["pullRequestId"])

    # -- 3. work items ---------------------------------------------------------

    def upsert_work_item(self, dedup_key, title, body_md,
                         routing: RoutingConfig | None = None) -> WorkItemRef:
        token = self.credential("work_items")
        project = os.environ.get("SYSTEM_TEAMPROJECT") or (routing.extra.get("project") if routing else None)
        if not project:
            raise PortError("ADO work items need SYSTEM_TEAMPROJECT or routing.extra.project")
        wiql = {
            "query": (
                "SELECT [System.Id] FROM WorkItems "
                f"WHERE [System.Tags] CONTAINS '{dedup_key}' "
                "AND [System.State] <> 'Closed' AND [System.State] <> 'Removed' "
                "ORDER BY [System.ChangedDate] DESC"
            )
        }
        found = http_json(
            f"{_org()}/{project}/_apis/wit/wiql?api-version=7.1",
            method="POST", token=token, auth_scheme="Basic", body=wiql,
        ).get("workItems", [])
        if found:
            wid = found[0]["id"]
            http_json(
                f"{_org()}/{project}/_apis/wit/workItems/{wid}/comments"
                "?api-version=7.1-preview.4",
                method="POST", token=token, auth_scheme="Basic",
                body={"text": body_md},
            )
            return WorkItemRef(url=f"{_org()}/{project}/_workitems/edit/{wid}",
                               id=str(wid), reused=True)
        item_type = (routing.extra.get("type") if routing else None) or "Issue"
        tags = "; ".join([dedup_key, *(routing.labels if routing else [])])
        patch = [
            {"op": "add", "path": "/fields/System.Title", "value": title},
            {"op": "add", "path": "/fields/System.Description", "value": body_md},
            {"op": "add", "path": "/fields/System.Tags", "value": tags},
        ]
        for fld, val in ((routing.extra if routing else {}).get("fields") or {}).items():
            patch.append({"op": "add", "path": f"/fields/{fld}", "value": val})
        data = http_json(
            f"{_org()}/{project}/_apis/wit/workitems/${item_type}?api-version=7.1",
            method="POST", token=token, auth_scheme="Basic", body=patch,
            content_type="application/json-patch+json",
        )
        wid = data["id"]
        return WorkItemRef(url=f"{_org()}/{project}/_workitems/edit/{wid}", id=str(wid))

    # -- 4. CI output surface ----------------------------------------------------

    def emit_output(self, key: str, value: str) -> None:
        print(f"{key}={value}")  # neutral line first (spec §2)
        print(f"##vso[task.setvariable variable={key};isOutput=true]{value}")

    def emit_summary(self, markdown: str) -> None:
        path = os.environ.get("VENDKIT_ADO_SUMMARY_FILE")
        if path:
            with open(path, "a", encoding="utf-8") as fh:
                fh.write(markdown + "\n")
            print(f"##vso[task.uploadsummary]{path}")
        else:
            print(markdown, file=sys.stderr)

    def emit_error(self, message: str) -> None:
        print(f"##vso[task.logissue type=error]{message}")

    # -- 5/6. platform facts and credentials --------------------------------------

    def verify_platform_fact(self, fact: str) -> bool | None:
        return None  # API verification: post-1.0 (conformance spec §4)

    def credential(self, purpose: str) -> str | None:
        token = token_for(purpose, "SYSTEM_ACCESSTOKEN", "ADO_PAT")
        if token is None and purpose in ("open_pr", "work_items", "read_upstream"):
            raise PortError(
                f"no credential for {purpose} (set VENDKIT_TOKEN_"
                f"{purpose.upper()}, SYSTEM_ACCESSTOKEN, or ADO_PAT)"
            )
        return token
