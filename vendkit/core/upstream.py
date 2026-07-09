"""Upstream reads over the git protocol — vendor-service-free (DR-0015).

Listing a publisher's release tags and reading a file at a tag are plain git
operations that work identically against GitHub, Azure Repos, or a local
path; authentication is ordinary git credentials. The only vendor knowledge
here is the *URL template* that expands an `owner/repo` shorthand — data,
not a service integration.
"""

from __future__ import annotations

import subprocess
import tempfile
from dataclasses import dataclass
from pathlib import Path

from .util import UsageError, VendkitError

# scm → clone-URL template (handler-protocol spec §5). A repo coordinate that
# is already a URL or a filesystem path is used verbatim.
URL_TEMPLATES = {
    "github": "https://github.com/{repo}.git",
    # Azure Repos shorthand is org/project/repo (three segments).
    "azure-repos": "https://dev.azure.com/{org}/{project}/_git/{name}",
}


@dataclass(frozen=True)
class Tag:
    name: str
    commit: str


def clone_url(scm: str, repo: str) -> str:
    """Something git can clone. Verbatim for URLs and paths; otherwise the
    scm-keyed shorthand expansion."""
    if "://" in repo or repo.startswith(("/", "./", "../")) or repo.startswith("git@"):
        return repo
    parts = repo.split("/")
    if scm == "github":
        if len(parts) != 2:
            raise UsageError(f"github repo shorthand must be owner/repo: {repo!r}")
        return URL_TEMPLATES["github"].format(repo=repo)
    if scm == "azure-repos":
        if len(parts) != 3:
            raise UsageError(
                f"azure-repos shorthand must be org/project/repo: {repo!r}")
        org, project, name = parts
        return URL_TEMPLATES["azure-repos"].format(
            org=org, project=project, name=name)
    raise UsageError(f"unknown scm {scm!r}")


def list_release_tags(url: str) -> list[Tag]:
    """All tags with peeled commit SHAs, via `git ls-remote`.

    No --refs: annotated tags list the tag OBJECT sha on the bare ref and
    the peeled commit on the `^{}` line — provenance needs the commit.
    """
    proc = subprocess.run(
        ["git", "ls-remote", "--tags", url], capture_output=True, text=True,
    )
    if proc.returncode != 0:
        raise VendkitError(
            f"listing tags of {url} failed: {proc.stderr.strip()}"
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
    return [Tag(name=n, commit=s) for n, s in sorted(shas.items())]


def fetch_publisher(url: str, ref: str, dest: str) -> None:
    """A working checkout of the publisher at `ref` in `dest` (human-tier
    diff/update). Depth-1 clone of just that tag; works for URLs and local
    paths alike. NOTE: the human tier runs the *installed* engine against
    this tree — a documented relaxation of INV-6, guarded by schema-version
    gating (cli spec, human tier)."""
    proc = subprocess.run(
        ["git", "clone", "-q", "--depth", "1", "--branch", ref, url, dest],
        capture_output=True, text=True,
    )
    if proc.returncode != 0:
        raise VendkitError(
            f"cloning {url} at {ref} failed: {proc.stderr.strip()}")


def read_file_at(url: str, ref: str, path: str) -> bytes:
    """One file's bytes at a ref, without a full clone.

    Local publisher directory: `git show` directly. Remote: a throwaway
    depth-1 fetch of just that ref (blobless would be nicer; depth-1 of one
    tag is small enough and works on every git server).
    """
    if Path(url).is_dir():
        proc = subprocess.run(
            ["git", "-C", url, "show", f"{ref}:{path}"], capture_output=True,
        )
        if proc.returncode != 0:
            raise VendkitError(f"cannot read {path}@{ref} from {url}")
        return proc.stdout
    with tempfile.TemporaryDirectory(prefix="vendkit-fetch-") as tmp:
        for args in (["init", "-q"],
                     ["fetch", "-q", "--depth", "1", url, f"refs/tags/{ref}"]):
            proc = subprocess.run(["git", "-C", tmp, *args],
                                  capture_output=True, text=True)
            if proc.returncode != 0:
                raise VendkitError(
                    f"fetching {ref} from {url} failed: {proc.stderr.strip()}")
        proc = subprocess.run(
            ["git", "-C", tmp, "show", f"FETCH_HEAD:{path}"],
            capture_output=True,
        )
        if proc.returncode != 0:
            raise VendkitError(f"cannot read {path}@{ref} from {url}")
        return proc.stdout
