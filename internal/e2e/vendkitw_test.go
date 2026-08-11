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

// fakeRelease lays out a release-asset directory: a stub engine for the host
// platform plus a SHA256SUMS.txt covering it, and returns its file:// base.
func fakeRelease(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	asset := fmt.Sprintf("vendkit_%s_%s_%s", fakeVersion, runtime.GOOS, runtime.GOARCH)
	stub := "#!/usr/bin/env bash\necho \"FAKE-ENGINE $*\"\n"
	write(t, filepath.Join(dir, asset), stub)
	sum := sha256.Sum256([]byte(stub))
	write(t, filepath.Join(dir, "SHA256SUMS.txt"),
		fmt.Sprintf("%x  %s\n", sum, asset))
	return "file://" + dir
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
