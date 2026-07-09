"""Journal handler — the neutral reference implementation.

Records every intent to a JSONL journal (VENDKIT_NEUTRAL_JOURNAL) or stdout
and reports a synthetic URL. Serves three purposes: the scenario kit's
delivery assertion point, a template for writing real handlers, and a
visible-not-silent sink for local/manual runs (handler-protocol spec §6).
"""

from __future__ import annotations

import json
import os
import sys

from ._shared import emit, read_intent, run


def main() -> None:
    intent = read_intent(("pr", "handoff", "fact-verify"))
    path = os.environ.get("VENDKIT_NEUTRAL_JOURNAL")
    line = json.dumps(intent, sort_keys=True)
    if path:
        with open(path, "a", encoding="utf-8") as fh:
            fh.write(line + "\n")
    else:
        print(f"[journal-handler] {line}", file=sys.stderr)
    kind = intent["kind"]
    if kind == "pr":
        emit("url", f"neutral://pr/{intent['head_branch']}")
        emit("number", "0")
    elif kind == "handoff":
        emit("url", f"neutral://item/{intent['dedup_key']}")
    else:
        emit("verdict", "unknown")  # the journal can verify nothing


if __name__ == "__main__":
    sys.exit(run(main))
