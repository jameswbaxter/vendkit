"""CI output surface — the only CI-platform adaptation in the engine.

After DR-0014 expelled every vendor *service* integration to handlers and
DR-0015 moved upstream reads onto the git protocol, what remains
platform-specific in-process is purely the host CI's output dialect:
step outputs, summaries, and error annotations. Everything else the engine
does is git + filesystem.

Every surface also prints the plain `key=value` line, so logs are greppable
identically everywhere and Layer 2 wrappers never parse prose.
"""

from __future__ import annotations

import os
import sys

from .core.util import UsageError

CI_VALUES = ("github-actions", "azure-pipelines", "neutral")


def detect() -> str:
    """Host detection: explicit override, then CI env conventions."""
    override = os.environ.get("VENDKIT_PLATFORM")
    if override:
        return override
    if os.environ.get("GITHUB_ACTIONS") == "true":
        return "github-actions"
    if os.environ.get("TF_BUILD") == "True":
        return "azure-pipelines"
    return "neutral"


class NeutralOutput:
    name = "neutral"

    def emit_output(self, key: str, value: str) -> None:
        print(f"{key}={value}")

    def emit_summary(self, markdown: str) -> None:
        print(markdown, file=sys.stderr)

    def emit_error(self, message: str) -> None:
        print(f"ERROR: {message}", file=sys.stderr)


class GitHubActionsOutput(NeutralOutput):
    name = "github-actions"

    def emit_output(self, key: str, value: str) -> None:
        print(f"{key}={value}")
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


class AzurePipelinesOutput(NeutralOutput):
    name = "azure-pipelines"

    def emit_output(self, key: str, value: str) -> None:
        print(f"{key}={value}")
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


def get_surface(name: str | None = None):
    name = name or detect()
    surfaces = {
        "github-actions": GitHubActionsOutput,
        "azure-pipelines": AzurePipelinesOutput,
        "neutral": NeutralOutput,
    }
    if name not in surfaces:
        raise UsageError(
            f"unknown CI surface {name!r} (expected one of {CI_VALUES})")
    return surfaces[name]()
