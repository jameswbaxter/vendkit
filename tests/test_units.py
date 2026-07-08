"""Unit/property coverage (testing.md §1)."""

import pytest

from vendkit.core import versions
from vendkit.core.normalise import hash_as_recorded, normalise_hash
from vendkit.core.util import Refusal, UsageError, path_match


# -- versions ------------------------------------------------------------------

def test_grammar_stable_and_rc():
    assert versions.parse("v1.2.3") == (1, 2, 3, 1, 0)
    assert versions.parse("v1.2.3-rc.1") is None  # invisible on stable
    assert versions.parse("v1.2.3-rc.1", "rc") == (1, 2, 3, 0, 1)
    for bad in ("1.2.3", "v1.2", "v01.2.3", "v1.2.3-rc.0", "v1.2.3.4"):
        assert versions.parse(bad, "rc") is None, bad


def test_ordering_rc_below_stable():
    assert versions.require("v1.2.3") > versions.require("v1.2.3-rc.9")
    assert versions.require("v1.2.4-rc.1") > versions.require("v1.2.3")


def test_is_newer_and_retraction():
    assert versions.is_newer("v1.0.0", "v1.0.1")
    assert not versions.is_newer("v1.0.1", "v1.0.1")
    with pytest.raises(Refusal) as exc:
        versions.is_newer("v1.0.0", "v1.0.1", retracted=["v1.0.1"])
    assert exc.value.reason == "retracted"


def test_latest_channels_and_retraction():
    tags = ["v1.0.0", "v1.1.0-rc.1", "junk", "v0.9.0"]
    assert versions.latest(tags) == "v1.0.0"
    assert versions.latest(tags, channel="rc") == "v1.1.0-rc.1"
    assert versions.latest(tags, retracted=["v1.0.0"]) == "v0.9.0"
    assert versions.latest(["nope"]) is None


def test_bump_and_window():
    assert versions.bump("v1.2.3", "major") == "v2.0.0"
    assert versions.bump("v1.2.3", "minor") == "v1.3.0"
    assert versions.bump("v1.2.3", "patch") == "v1.2.4"
    assert versions.classify_bump("v1.2.3", "v1.3.0") == "minor"
    assert versions.in_window("v1.0.0", "v1.5.0", "v2.0.0")
    assert not versions.in_window("v1.5.0", "v1.5.0", "v2.0.0")
    assert not versions.in_window("v1.0.0", "v2.1.0", "v2.0.0")


# -- normalisation (DR-0004) -----------------------------------------------------

def test_crlf_and_trailing_ws_invisible():
    a, _ = normalise_hash(b"line one\nline two\n")
    b, _ = normalise_hash(b"line one \r\nline two\t\r\n\r\n\r\n")
    assert a == b


def test_real_edit_detected():
    a, _ = normalise_hash(b"alpha\n")
    b, _ = normalise_hash(b"alpha!\n")
    assert a != b


def test_binary_is_raw():
    digest, raw = normalise_hash(b"\xff\xfe\x00binary")
    assert raw
    assert hash_as_recorded(b"\xff\xfe\x00binary", True) == digest


def test_empty_file_stable():
    a, _ = normalise_hash(b"")
    b, _ = normalise_hash(b"\n\n\n")
    assert a == b


# -- the one glob matcher ----------------------------------------------------------

def test_path_match_crosses_segments():
    assert path_match("docs/a/b/c.md", "docs/**")
    assert path_match("docs/x.md", "docs/*.md")
    assert not path_match("docs/x.md", "src/*")


# -- declaration / adapters --------------------------------------------------------

def _decl(tmp_path, extra=""):
    (tmp_path / "docs").mkdir()
    (tmp_path / "docs" / "a.md").write_text("# a\n")
    (tmp_path / "docs" / "TEMPLATE.md").write_text("t\n")
    (tmp_path / "vendkit-export.yml").write_text(f"""\
schema_version: 1
slice: {{name: docs, title: Docs}}
publisher: {{platform: github, repo: example-org/pub}}
include: ["docs/**/*.md"]
exclude: ["**/TEMPLATE.md"]
{extra}
""")
    from vendkit.core.declaration import ExportDecl
    return ExportDecl.load(str(tmp_path / "vendkit-export.yml"))


def test_declaration_surface(tmp_path):
    decl = _decl(tmp_path)
    assert decl.exported_files(str(tmp_path)) == ["docs/a.md"]


def test_unknown_adapter_kind_is_hard_error(tmp_path):
    with pytest.raises(UsageError, match="unknown kind"):
        _decl(tmp_path, "adapters: [{kind: mystery, match: '*'}]")


def test_unknown_top_level_key_rejected(tmp_path):
    with pytest.raises(UsageError, match="unknown top-level key"):
        _decl(tmp_path, "surprise: true")


def test_prefix_namespace_consumer_path(tmp_path):
    decl = _decl(
        tmp_path,
        "adapters: [{kind: prefix-namespace, match: 'docs/*.md', prefix: 'vnd-'}]",
    )
    assert decl.consumer_path("docs/a.md") == "docs/vnd-a.md"
    assert decl.consumer_path("docs/vnd-a.md") == "docs/vnd-a.md"  # idempotent


def test_glob_localise_prunes_other_profiles(tmp_path):
    decl = _decl(tmp_path, """\
adapters:
  - kind: glob-localise
    match: "docs/*.md"
    field: applyTo
    catalogue:
      code: ["src/**"]
      docs-only: ["manuals/**"]
profiles: {code: {}, docs-only: {}}
""")
    from vendkit.core.adapters import apply_adapters
    body = b'---\napplyTo: "src/**, manuals/**, **/*.py"\n---\nx\n'
    out = apply_adapters(decl, "docs/a.md", body, "code")
    assert b'applyTo: "src/**, **/*.py"' in out       # other-profile glob dropped
    unbound = apply_adapters(decl, "docs/a.md", body, None)
    assert unbound == body                            # unbound keeps the union


def test_profile_export_slice(tmp_path):
    decl = _decl(tmp_path, """\
profiles:
  lean:
    export_slice: {include: ["docs/*"], exclude: ["docs/a.md"]}
""")
    assert not decl.profile_in_scope("lean", "docs/a.md")
    assert decl.profile_in_scope(None, "docs/a.md")
