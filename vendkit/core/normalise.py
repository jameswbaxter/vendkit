"""Normalised content hashing (DR-0004, manifest spec §2). Stdlib only."""

from __future__ import annotations

import hashlib

RECIPE = "utf8;lf;strip-trailing-ws;single-final-newline;sha256"


def normalise_hash(data: bytes) -> tuple[str, bool]:
    """Return (sha256 hexdigest, raw) per normalisation recipe v1.

    raw=True means the bytes are not valid UTF-8; the hash is then over the
    raw bytes. Text/binary is decided here at generate time and recorded in
    the manifest — verify honours the recorded flag (manifest spec §1).
    """
    try:
        text = data.decode("utf-8", errors="strict")
    except UnicodeDecodeError:
        return hashlib.sha256(data).hexdigest(), True
    text = text.replace("\r\n", "\n").replace("\r", "\n")
    body = "\n".join(line.rstrip(" \t") for line in text.split("\n"))
    body = body.rstrip("\n") + "\n"
    return hashlib.sha256(body.encode("utf-8")).hexdigest(), False


def hash_as_recorded(data: bytes, raw: bool) -> str:
    """Hash honouring a recorded raw flag (never re-guess; manifest spec §1)."""
    if raw:
        return hashlib.sha256(data).hexdigest()
    digest, now_raw = normalise_hash(data)
    if now_raw:
        # Recorded as text but no longer decodable — hash raw so the
        # comparison fails as `changed` rather than crashing.
        return hashlib.sha256(data).hexdigest()
    return digest
