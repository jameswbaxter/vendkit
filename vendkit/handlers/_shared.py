"""Shared plumbing for the reference handlers: stdin/stdout protocol frame,
HTTP helper, credential resolution. Stdlib only."""

from __future__ import annotations

import base64
import json
import os
import sys
import urllib.error
import urllib.request

PROTOCOL_VERSION = 1


class HandlerFailure(Exception):
    """Delivery failed. main() maps this to exit 1 with the message on
    stderr — the engine treats any nonzero handler exit as loud
    infrastructure failure."""


def read_intent(expected_kinds: tuple[str, ...]) -> dict:
    try:
        document = json.load(sys.stdin)
    except json.JSONDecodeError as exc:
        raise HandlerFailure(f"stdin is not a JSON intent document: {exc}")
    if document.get("vendkit_handler_protocol") != PROTOCOL_VERSION:
        raise HandlerFailure(
            f"unsupported protocol version "
            f"{document.get('vendkit_handler_protocol')!r}")
    if document.get("kind") not in expected_kinds:
        raise HandlerFailure(
            f"this handler serves {expected_kinds}, got {document.get('kind')!r}")
    return document


def emit(key: str, value: str) -> None:
    print(f"{key}={value}")


def run(handler) -> int:
    """Wrap a handler entrypoint with the protocol's exit-code contract."""
    try:
        handler()
        return 0
    except HandlerFailure as exc:
        print(f"handler failure: {exc}", file=sys.stderr)
        return 1


def token_for(purpose: str, *fallback_env: str) -> str | None:
    """VENDKIT_TOKEN_<PURPOSE> wins, then vendor conventions."""
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
    body: dict | list | None = None,
    content_type: str = "application/json",
):
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
        raise HandlerFailure(f"{method} {url} -> HTTP {exc.code}: {detail}")
    except urllib.error.URLError as exc:
        raise HandlerFailure(f"{method} {url} failed: {exc.reason}")
