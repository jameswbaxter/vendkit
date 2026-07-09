"""Azure DevOps reference handler: PR upsert, work-item handoff, fact
verification. REST 7.1 via stdlib urllib; PAT/Basic auth.

Coordinates: VENDKIT_ADO_ORG_URL (e.g. https://dev.azure.com/example-org);
the target repo is intent.repo ('<project>/<repository>') or the pipeline's
SYSTEM_TEAMPROJECT / BUILD_REPOSITORY_NAME.
"""

from __future__ import annotations

import os
import sys

from ._shared import HandlerFailure, emit, http_json, read_intent, run, token_for

ORG_URL = os.environ.get("VENDKIT_ADO_ORG_URL", "").rstrip("/")


def _org() -> str:
    if not ORG_URL:
        raise HandlerFailure(
            "VENDKIT_ADO_ORG_URL is not set (e.g. https://dev.azure.com/example-org)")
    return ORG_URL


def _project_repo(intent: dict) -> tuple[str, str]:
    repo = intent.get("repo") or (
        f"{os.environ.get('SYSTEM_TEAMPROJECT', '')}/"
        f"{os.environ.get('BUILD_REPOSITORY_NAME', '')}")
    project, _, repository = repo.partition("/")
    if not project or not repository:
        raise HandlerFailure(
            f"target repo must be '<project>/<repository>': {repo!r}")
    return project, repository


def _token(purpose: str) -> str:
    token = token_for(purpose, "SYSTEM_ACCESSTOKEN", "ADO_PAT")
    if not token:
        raise HandlerFailure(
            f"no credential for {purpose} (set VENDKIT_TOKEN_{purpose.upper()}, "
            "SYSTEM_ACCESSTOKEN, or ADO_PAT)")
    return token


def _pr(intent: dict) -> None:
    project, repository = _project_repo(intent)
    token, head = _token("open_pr"), intent["head_branch"]
    base = f"{_org()}/{project}/_apis/git/repositories/{repository}"
    active = http_json(
        f"{base}/pullrequests?searchCriteria.status=active"
        f"&searchCriteria.sourceRefName=refs/heads/{head}&api-version=7.1",
        token=token, auth_scheme="Basic",
    ).get("value", [])
    if active:
        pr_id = active[0]["pullRequestId"]
        http_json(f"{base}/pullrequests/{pr_id}?api-version=7.1",
                  method="PATCH", token=token, auth_scheme="Basic",
                  body={"title": intent["title"],
                        "description": intent["body_md"]})
    else:
        pr_id = http_json(
            f"{base}/pullrequests?api-version=7.1",
            method="POST", token=token, auth_scheme="Basic",
            body={"sourceRefName": f"refs/heads/{head}",
                  "targetRefName": f"refs/heads/{intent['base_branch']}",
                  "title": intent["title"],
                  "description": intent["body_md"]},
        )["pullRequestId"]
    emit("url", f"{_org()}/{project}/_git/{repository}/pullrequest/{pr_id}")
    emit("number", str(pr_id))


def _handoff(intent: dict) -> None:
    token, key = _token("work_items"), intent["dedup_key"]
    project = intent.get("project") or os.environ.get("SYSTEM_TEAMPROJECT")
    if not project:
        raise HandlerFailure(
            "work items need intent.project or SYSTEM_TEAMPROJECT")
    wiql = {"query": (
        "SELECT [System.Id] FROM WorkItems "
        f"WHERE [System.Tags] CONTAINS '{key}' "
        "AND [System.State] <> 'Closed' AND [System.State] <> 'Removed' "
        "ORDER BY [System.ChangedDate] DESC")}
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
            body={"text": intent["body_md"]})
    else:
        item_type = intent.get("item_type") or "Issue"
        patch = [
            {"op": "add", "path": "/fields/System.Title",
             "value": intent["title"]},
            {"op": "add", "path": "/fields/System.Description",
             "value": intent["body_md"]},
            {"op": "add", "path": "/fields/System.Tags", "value": key},
        ]
        wid = http_json(
            f"{_org()}/{project}/_apis/wit/workitems/${item_type}"
            "?api-version=7.1",
            method="POST", token=token, auth_scheme="Basic", body=patch,
            content_type="application/json-patch+json",
        )["id"]
    emit("url", f"{_org()}/{project}/_workitems/edit/{wid}")


def _fact_verify(intent: dict) -> None:
    emit("verdict", "unknown")  # API verification: post-1.0


def main() -> None:
    intent = read_intent(("pr", "handoff", "fact-verify"))
    {"pr": _pr, "handoff": _handoff, "fact-verify": _fact_verify}[intent["kind"]](intent)


if __name__ == "__main__":
    sys.exit(run(main))
