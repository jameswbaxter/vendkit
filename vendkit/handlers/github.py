"""GitHub reference handler: PR upsert, issue handoff, fact verification.

REST via stdlib urllib. Credentials: VENDKIT_TOKEN_<PURPOSE>, then
GITHUB_TOKEN / GH_TOKEN — except PR delivery, which refuses to fall back to
GITHUB_TOKEN (PRs it opens do not trigger pull_request workflows, so the
sync PR would silently skip its own gate — differences ledger #2).
"""

from __future__ import annotations

import os
import sys

from ._shared import HandlerFailure, emit, http_json, read_intent, run, token_for

API = os.environ.get("VENDKIT_GITHUB_API", "https://api.github.com")


def _repo(intent: dict) -> str:
    repo = intent.get("repo") or os.environ.get("GITHUB_REPOSITORY")
    if not repo:
        raise HandlerFailure("no target repo: set intent.repo or GITHUB_REPOSITORY")
    return repo


def _pr(intent: dict) -> None:
    token = token_for("open_pr") or os.environ.get("VENDKIT_PR_TOKEN")
    if not token:
        raise HandlerFailure(
            "PR delivery needs VENDKIT_TOKEN_OPEN_PR or VENDKIT_PR_TOKEN "
            "(a PAT/App token — GITHUB_TOKEN-opened PRs do not trigger "
            "workflows, so the sync PR would skip its own gate)")
    repo, head = _repo(intent), intent["head_branch"]
    owner = repo.split("/")[0]
    open_prs = http_json(
        f"{API}/repos/{repo}/pulls?state=open&head={owner}:{head}", token=token)
    if open_prs:
        pr = open_prs[0]
        http_json(f"{API}/repos/{repo}/pulls/{pr['number']}",
                  method="PATCH", token=token,
                  body={"title": intent["title"], "body": intent["body_md"]})
    else:
        pr = http_json(
            f"{API}/repos/{repo}/pulls", method="POST", token=token,
            body={"title": intent["title"], "body": intent["body_md"],
                  "head": head, "base": intent["base_branch"]})
    emit("url", pr["html_url"])
    emit("number", str(pr["number"]))


def _handoff(intent: dict) -> None:
    token = token_for("work_items", "GITHUB_TOKEN", "GH_TOKEN")
    if not token:
        raise HandlerFailure("no credential for issues (set GITHUB_TOKEN)")
    repo, key = _repo(intent), intent["dedup_key"]
    # Idempotency contract (handler-protocol spec §3): one open item per
    # dedup_key — find by label and comment, else create labelled.
    found = http_json(
        f"{API}/repos/{repo}/issues?state=open&labels={key}", token=token)
    if found:
        issue = found[0]
        http_json(f"{API}/repos/{repo}/issues/{issue['number']}/comments",
                  method="POST", token=token, body={"body": intent["body_md"]})
    else:
        issue = http_json(
            f"{API}/repos/{repo}/issues", method="POST", token=token,
            body={"title": intent["title"], "body": intent["body_md"],
                  "labels": [key]})
    emit("url", issue["html_url"])


def _fact_verify(intent: dict) -> None:
    # API verification of branch protection / required checks: post-1.0.
    # "unknown" keeps the conformance status at `attested` (never a fail).
    emit("verdict", "unknown")


def main() -> None:
    intent = read_intent(("pr", "handoff", "fact-verify"))
    {"pr": _pr, "handoff": _handoff, "fact-verify": _fact_verify}[intent["kind"]](intent)


if __name__ == "__main__":
    sys.exit(run(main))
