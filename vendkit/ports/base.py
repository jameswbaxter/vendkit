"""Port interface (platform-ports spec §2) and selection (§1)."""

from __future__ import annotations

import base64
import json
import os
import urllib.error
import urllib.request
from dataclasses import dataclass, field

from ..core.util import VendkitError


class PortError(VendkitError):
    """Platform operation failure. Always loud (security model §4)."""


@dataclass(frozen=True)
class RepoRef:
    platform: str  # github | ado
    repo: str      # owner/repo (github) | project/repo (ado)


@dataclass(frozen=True)
class TagInfo:
    name: str
    commit: str


@dataclass
class PrRef:
    url: str
    number: int
    existed: bool = False


@dataclass
class WorkItemRef:
    url: str
    id: str
    reused: bool = False


@dataclass
class RoutingConfig:
    labels: list[str] = field(default_factory=list)
    extra: dict = field(default_factory=dict)


def detect() -> str:
    """Runner platform detection (platform-ports spec §1)."""
    override = os.environ.get("VENDKIT_PLATFORM")
    if override:
        return override
    if os.environ.get("GITHUB_ACTIONS") == "true":
        return "github"
    if os.environ.get("TF_BUILD") == "True":
        return "ado"
    return "neutral"


def get_port(name: str | None = None):
    name = name or detect()
    if name == "github":
        from .github import GitHubPort
        return GitHubPort()
    if name == "ado":
        from .ado import AdoPort
        return AdoPort()
    if name == "neutral":
        from .neutral import NeutralPort
        return NeutralPort()
    raise PortError(f"unknown platform: {name!r}")


def token_for(purpose: str, *fallback_env: str) -> str | None:
    """Credential resolution: VENDKIT_TOKEN_<PURPOSE> wins, then platform
    conventions (platform-ports spec §3)."""
    explicit = os.environ.get(f"VENDKIT_TOKEN_{purpose.upper()}")
    if explicit:
        return explicit
    for env in fallback_env:
        if os.environ.get(env):
            return os.environ[env]
    return None


def http_json(
    url: str,
    method: str = "GET",
    token: str | None = None,
    auth_scheme: str = "Bearer",
    body: dict | None = None,
    content_type: str = "application/json",
):
    """Minimal stdlib HTTP helper shared by the REST ports."""
    data = json.dumps(body).encode("utf-8") if body is not None else None
    req = urllib.request.Request(url, data=data, method=method)
    req.add_header("Accept", "application/json")
    if data is not None:
        req.add_header("Content-Type", content_type)
    if token:
        if auth_scheme == "Basic":
            cred = base64.b64encode(f":{token}".encode()).decode()
            req.add_header("Authorization", f"Basic {cred}")
        else:
            req.add_header("Authorization", f"Bearer {token}")
    try:
        with urllib.request.urlopen(req, timeout=30) as resp:
            payload = resp.read()
            return json.loads(payload) if payload else {}
    except urllib.error.HTTPError as exc:
        detail = exc.read().decode("utf-8", "replace")[:500]
        raise PortError(f"{method} {url} -> HTTP {exc.code}: {detail}") from exc
    except urllib.error.URLError as exc:
        raise PortError(f"{method} {url} failed: {exc.reason}") from exc
