// vendkitw launcher behaviour (DR-0016 amendment): resolve the pin from the
// slice config, fetch from a base URL, verify against SHA256SUMS.txt BEFORE
// exec, cache outside the repo, and fail loudly — never a silent fallback.
// The "release" is a local directory served over file:// URLs; the "engine"
// is a stub script that echoes its argv, so a run that prints proves
// fetch → verify → exec end-to-end with no network.

package e2e

import (
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

const fakeVersion = "v9.9.9"

func launcherScript(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Join(filepath.Dir(thisFile), "..", "..", "scaffold", "vendkitw")
}

func requireLauncherDeps(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("vendkitw is exercised via git-bash on Windows in release-smoke, not here")
	}
	for _, tool := range []string{"bash", "curl"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("%s not available", tool)
		}
	}
}

func writeReleaseAssets(t *testing.T, dir string) {
	t.Helper()
	asset := fmt.Sprintf("vendkit_%s_%s_%s", fakeVersion, runtime.GOOS, runtime.GOARCH)
	stub := "#!/usr/bin/env bash\necho \"FAKE-ENGINE $*\"\n"
	write(t, filepath.Join(dir, asset), stub)
	sum := sha256.Sum256([]byte(stub))
	write(t, filepath.Join(dir, "SHA256SUMS.txt"),
		fmt.Sprintf("%x  %s\n", sum, asset))
}

// fakeRelease lays out a release-asset directory: a stub engine for the host
// platform plus a SHA256SUMS.txt covering it, and returns its file:// base.
func fakeRelease(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	writeReleaseAssets(t, dir)
	return "file://" + dir
}

// fakeReleaseRoot lays the same assets under <root>/<version>/ — the
// version-independent layout an engine.base_url points at (the launcher
// appends the version segment itself, issue #15).
func fakeReleaseRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, fakeVersion)
	mkdirAll(t, dir)
	writeReleaseAssets(t, dir)
	return "file://" + root
}

// consumerWithPin scaffolds the minimal consumer shape the launcher reads:
// .vendkit/consumer/<slice>.yml with a publisher block and an engine pin.
func consumerWithPin(t *testing.T, slice, version string) string {
	t.Helper()
	con := t.TempDir()
	mkdirAll(t, filepath.Join(con, ".vendkit", "consumer"))
	write(t, filepath.Join(con, ".vendkit", "consumer", slice+".yml"),
		"schema_version: 1\n"+
			"slice: "+slice+"\n"+
			"publisher:\n"+
			"  scm: github\n"+
			"  repo: example-org/pub\n"+
			"engine:\n"+
			"  version: "+version+"\n")
	return con
}

// publisherWithPin scaffolds the publisher-checkout shape the launcher
// detects (issue #15): export-declaration.yml identifies the checkout, and
// vendkit-engine.yml — when non-empty — carries the engine pin under the
// same key paths a slice config uses.
func publisherWithPin(t *testing.T, pinYAML string) string {
	t.Helper()
	pub := t.TempDir()
	mkdirAll(t, filepath.Join(pub, ".vendkit", "publisher"))
	write(t, filepath.Join(pub, ".vendkit", "publisher", "export-declaration.yml"),
		"schema_version: 1\nslice: docs\n")
	if pinYAML != "" {
		write(t, filepath.Join(pub, ".vendkit", "publisher", "vendkit-engine.yml"), pinYAML)
	}
	return pub
}

func runLauncher(t *testing.T, dir string, env map[string]string,
	args ...string) (string, string, int) {
	t.Helper()
	return runCmd(t, "bash", dir, env,
		append([]string{launcherScript(t)}, args...)...)
}

func TestVendkitwFetchVerifyExecAndCache(t *testing.T) {
	requireLauncherDeps(t)
	base := fakeRelease(t)
	con := consumerWithPin(t, "docs", fakeVersion)
	cache := t.TempDir()
	env := map[string]string{"VENDKIT_BASE_URL": base, "VENDKIT_CACHE_DIR": cache}

	so, se, code := runLauncher(t, con, env, "gate", "--all")
	if code != 0 || !strings.Contains(so, "FAKE-ENGINE gate --all") {
		t.Fatalf("launcher run failed: code=%d\nstdout:\n%s\nstderr:\n%s", code, so, se)
	}

	// The verified binary is cached outside the repo, keyed by
	// version/os/arch — a second run needs no release assets at all.
	cached := filepath.Join(cache, fakeVersion,
		runtime.GOOS+"_"+runtime.GOARCH, "vendkit")
	if !exists(cached) {
		t.Fatalf("engine not cached at %s", cached)
	}
	env["VENDKIT_BASE_URL"] = "file:///nonexistent"
	so, se, code = runLauncher(t, con, env, "again")
	if code != 0 || !strings.Contains(so, "FAKE-ENGINE again") {
		t.Fatalf("cached run failed: code=%d\nstdout:\n%s\nstderr:\n%s", code, so, se)
	}
}

func TestVendkitwRefusesChecksumMismatch(t *testing.T) {
	requireLauncherDeps(t)
	base := fakeRelease(t)
	// Corrupt the checksum entry after the fact.
	dir := strings.TrimPrefix(base, "file://")
	asset := fmt.Sprintf("vendkit_%s_%s_%s", fakeVersion, runtime.GOOS, runtime.GOARCH)
	write(t, filepath.Join(dir, "SHA256SUMS.txt"),
		strings.Repeat("0", 64)+"  "+asset+"\n")
	con := consumerWithPin(t, "docs", fakeVersion)
	env := map[string]string{"VENDKIT_BASE_URL": base, "VENDKIT_CACHE_DIR": t.TempDir()}

	so, se, code := runLauncher(t, con, env, "gate")
	if code == 0 {
		t.Fatal("checksum mismatch must be fatal")
	}
	if strings.Contains(so, "FAKE-ENGINE") {
		t.Fatal("engine executed despite a checksum mismatch")
	}
	if !strings.Contains(se, "checksum mismatch") {
		t.Errorf("failure is not loud/specific: %s", se)
	}
}

func TestVendkitwRefusesMissingChecksumEntry(t *testing.T) {
	requireLauncherDeps(t)
	base := fakeRelease(t)
	dir := strings.TrimPrefix(base, "file://")
	write(t, filepath.Join(dir, "SHA256SUMS.txt"),
		strings.Repeat("0", 64)+"  some_other_asset\n")
	con := consumerWithPin(t, "docs", fakeVersion)
	env := map[string]string{"VENDKIT_BASE_URL": base, "VENDKIT_CACHE_DIR": t.TempDir()}

	so, se, code := runLauncher(t, con, env, "gate")
	if code == 0 || strings.Contains(so, "FAKE-ENGINE") {
		t.Fatal("missing checksum entry must refuse to run the engine")
	}
	if !strings.Contains(se, "no checksum") {
		t.Errorf("failure is not loud/specific: %s", se)
	}
}

func TestVendkitwBinEscapeHatch(t *testing.T) {
	requireLauncherDeps(t)
	stub := filepath.Join(t.TempDir(), "local-vendkit")
	write(t, stub, "#!/usr/bin/env bash\necho \"LOCAL-ENGINE $*\"\n")
	if err := os.Chmod(stub, 0o755); err != nil {
		t.Fatal(err)
	}
	// No slice config, no release, no cache: VENDKIT_BIN short-circuits all.
	so, se, code := runLauncher(t, t.TempDir(),
		map[string]string{"VENDKIT_BIN": stub}, "self-verify")
	if code != 0 || !strings.Contains(so, "LOCAL-ENGINE self-verify") {
		t.Fatalf("VENDKIT_BIN escape hatch failed: code=%d\nstdout:\n%s\nstderr:\n%s",
			code, so, se)
	}
}

func TestVendkitwUnresolvedPinIsLoud(t *testing.T) {
	requireLauncherDeps(t)
	so, se, code := runLauncher(t, t.TempDir(), nil, "gate")
	if code == 0 || strings.Contains(so, "FAKE-ENGINE") {
		t.Fatal("an unresolved pin must be fatal, never a silent fallback")
	}
	if !strings.Contains(se, "VENDKIT_VERSION") {
		t.Errorf("failure does not say how to proceed: %s", se)
	}
}

func TestVendkitwSliceConfigsMustAgree(t *testing.T) {
	requireLauncherDeps(t)
	con := consumerWithPin(t, "docs", fakeVersion)
	write(t, filepath.Join(con, ".vendkit", "consumer", "other.yml"),
		"schema_version: 1\nslice: other\nengine:\n  version: v1.0.0\n")
	_, se, code := runLauncher(t, con,
		map[string]string{"VENDKIT_CACHE_DIR": t.TempDir()}, "gate")
	if code == 0 {
		t.Fatal("disagreeing engine pins must be fatal")
	}
	if !strings.Contains(se, "disagree") {
		t.Errorf("failure is not loud/specific: %s", se)
	}
}

// -- issue #15: publisher-side pin file + engine.base_url ------------------

func TestVendkitwPublisherPinFile(t *testing.T) {
	requireLauncherDeps(t)
	base := fakeRelease(t)
	pub := publisherWithPin(t, "engine:\n  version: "+fakeVersion+"\n")

	so, se, code := runLauncher(t, pub, map[string]string{
		"VENDKIT_BASE_URL": base, "VENDKIT_CACHE_DIR": t.TempDir()}, "generate", "--check")
	if code != 0 || !strings.Contains(so, "FAKE-ENGINE generate --check") {
		t.Fatalf("publisher-side pin not resolved: code=%d\nstdout:\n%s\nstderr:\n%s",
			code, so, se)
	}
}

func TestVendkitwPublisherPinCarriesBaseURL(t *testing.T) {
	requireLauncherDeps(t)
	root := fakeReleaseRoot(t)
	// No VENDKIT_BASE_URL: both the version and the release root come from
	// the committed pin file — no per-environment variables at all.
	pub := publisherWithPin(t,
		"engine:\n  version: "+fakeVersion+"\n  base_url: "+root+"\n")

	so, se, code := runLauncher(t, pub,
		map[string]string{"VENDKIT_CACHE_DIR": t.TempDir()}, "generate", "--check")
	if code != 0 || !strings.Contains(so, "FAKE-ENGINE generate --check") {
		t.Fatalf("engine.base_url not resolved from the pin file: code=%d\nstdout:\n%s\nstderr:\n%s",
			code, so, se)
	}
}

func TestVendkitwPublisherWithoutPinFileIsLoud(t *testing.T) {
	requireLauncherDeps(t)
	pub := publisherWithPin(t, "")

	so, se, code := runLauncher(t, pub,
		map[string]string{"VENDKIT_CACHE_DIR": t.TempDir()}, "generate")
	if code == 0 || strings.Contains(so, "FAKE-ENGINE") {
		t.Fatal("a publisher checkout without a pin file must fail loudly")
	}
	if !strings.Contains(se, "VENDKIT_VERSION") {
		t.Errorf("failure does not say how to proceed: %s", se)
	}
}

func TestVendkitwConsumerEngineBaseURL(t *testing.T) {
	requireLauncherDeps(t)
	root := fakeReleaseRoot(t)
	// The issue #15(B) shape: the slice's publisher is on Azure Repos, so the
	// asset URL cannot be derived from publisher.* — engine.base_url carries
	// the engine's own coordinates instead. No VENDKIT_BASE_URL.
	con := t.TempDir()
	mkdirAll(t, filepath.Join(con, ".vendkit", "consumer"))
	write(t, filepath.Join(con, ".vendkit", "consumer", "docs.yml"),
		"schema_version: 1\n"+
			"slice: docs\n"+
			"publisher:\n"+
			"  scm: azure-repos\n"+
			"  repo: org/project/pub\n"+
			"engine:\n"+
			"  version: "+fakeVersion+"\n"+
			"  base_url: "+root+"\n")

	so, se, code := runLauncher(t, con,
		map[string]string{"VENDKIT_CACHE_DIR": t.TempDir()}, "gate", "--all")
	if code != 0 || !strings.Contains(so, "FAKE-ENGINE gate --all") {
		t.Fatalf("engine.base_url not resolved from the slice config: code=%d\nstdout:\n%s\nstderr:\n%s",
			code, so, se)
	}
}

func TestVendkitwEngineBaseURLBeatsPublisherDerivation(t *testing.T) {
	requireLauncherDeps(t)
	root := fakeReleaseRoot(t)
	// A github publisher would derive https://github.com/... — the committed
	// engine.base_url must win over the derivation (it is more specific: the
	// engine's coordinates, not the slice publisher's).
	con := t.TempDir()
	mkdirAll(t, filepath.Join(con, ".vendkit", "consumer"))
	write(t, filepath.Join(con, ".vendkit", "consumer", "docs.yml"),
		"schema_version: 1\n"+
			"slice: docs\n"+
			"publisher:\n"+
			"  scm: github\n"+
			"  repo: example-org/pub\n"+
			"engine:\n"+
			"  version: "+fakeVersion+"\n"+
			"  base_url: "+root+"\n")

	so, se, code := runLauncher(t, con,
		map[string]string{"VENDKIT_CACHE_DIR": t.TempDir()}, "gate")
	if code != 0 || !strings.Contains(so, "FAKE-ENGINE gate") {
		t.Fatalf("engine.base_url did not take precedence: code=%d\nstdout:\n%s\nstderr:\n%s",
			code, so, se)
	}
}

func TestVendkitwAzureRepsPublisherWithoutBaseURLIsLoud(t *testing.T) {
	requireLauncherDeps(t)
	con := t.TempDir()
	mkdirAll(t, filepath.Join(con, ".vendkit", "consumer"))
	write(t, filepath.Join(con, ".vendkit", "consumer", "docs.yml"),
		"schema_version: 1\n"+
			"slice: docs\n"+
			"publisher:\n"+
			"  scm: azure-repos\n"+
			"  repo: org/project/pub\n"+
			"engine:\n"+
			"  version: "+fakeVersion+"\n")

	so, se, code := runLauncher(t, con,
		map[string]string{"VENDKIT_CACHE_DIR": t.TempDir()}, "gate")
	if code == 0 || strings.Contains(so, "FAKE-ENGINE") {
		t.Fatal("underivable asset URL must be fatal, never a guess")
	}
	if !strings.Contains(se, "engine.base_url") || !strings.Contains(se, "VENDKIT_BASE_URL") {
		t.Errorf("failure does not name both remedies: %s", se)
	}
}

func TestVendkitwConsumerPinShadowsPublisherPin(t *testing.T) {
	requireLauncherDeps(t)
	root := fakeReleaseRoot(t)
	// A checkout carrying BOTH halves (a repo that vendors and publishes) is
	// a consumer to the launcher: the consumer check runs first at every
	// level of the walk, so a publisher-side pin can never override a slice
	// config's. The publisher pin here is a decoy that resolves nothing.
	con := consumerWithPin(t, "docs", fakeVersion)
	mkdirAll(t, filepath.Join(con, ".vendkit", "publisher"))
	write(t, filepath.Join(con, ".vendkit", "publisher", "export-declaration.yml"),
		"schema_version: 1\nslice: docs\n")
	write(t, filepath.Join(con, ".vendkit", "publisher", "vendkit-engine.yml"),
		"engine:\n  version: v0.0.1\n  base_url: file:///nonexistent\n")

	so, se, code := runLauncher(t, con, map[string]string{
		"VENDKIT_BASE_URL": root + "/" + fakeVersion, "VENDKIT_CACHE_DIR": t.TempDir()}, "gate")
	if code != 0 || !strings.Contains(so, "FAKE-ENGINE gate") {
		t.Fatalf("consumer pin did not win: code=%d\nstdout:\n%s\nstderr:\n%s",
			code, so, se)
	}
}
