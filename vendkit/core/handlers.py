"""Handler invocation — the core side of the handler protocol (DR-0014).

The engine never talks to a vendor service. Where a judgment must become a
delivery (a sync PR, a watch/migration work item, an API-verified platform
fact), the engine composes a JSON *intent* document and hands it to a
configured executable on stdin. The handler owns the vendor API, its
credentials, and the idempotency obligations the protocol assigns it
(handler-protocol spec §3).

Resolution order per kind: VENDKIT_HANDLER_<KIND> env override (shell-split),
then the slice config's `handlers.<kind>.exec`, else None (report-only).
"""

from __future__ import annotations

import json
import os
import shlex
import subprocess

from .util import VendkitError

PROTOCOL_VERSION = 1
KINDS = ("pr", "handoff", "fact-verify")


class HandlerError(VendkitError):
    """Handler failed (nonzero exit). Always loud — a sync lane whose
    delivery silently vanishes is worse than a red pipeline."""


def resolve(kind: str, cfg) -> list[str] | None:
    """The command for a handler kind, or None when unwired."""
    env = os.environ.get(f"VENDKIT_HANDLER_{kind.upper().replace('-', '_')}")
    if env:
        return shlex.split(env)
    spec = (cfg.handlers if cfg else {}).get(kind) or {}
    return spec.get("exec") or None


def invoke(command: list[str], kind: str, payload: dict,
           cwd: str | None = None) -> dict[str, str]:
    """Run a handler: intent JSON on stdin, `key=value` facts on stdout.

    Exit 0 = delivered (facts returned); nonzero = infrastructure failure
    (HandlerError, exit >= 4 at the CLI). Handlers never encode judgment in
    their exit code — judgment happened before the handler was called.
    """
    document = {"vendkit_handler_protocol": PROTOCOL_VERSION,
                "kind": kind, **payload}
    proc = subprocess.run(
        command, input=json.dumps(document), text=True,
        capture_output=True, cwd=cwd,
    )
    if proc.returncode != 0:
        raise HandlerError(
            f"handler {command[0]} ({kind}) exited {proc.returncode}: "
            f"{proc.stderr.strip()[:500]}"
        )
    facts: dict[str, str] = {}
    for line in proc.stdout.splitlines():
        key, sep, value = line.partition("=")
        if sep and key and " " not in key:
            facts[key] = value
    return facts
