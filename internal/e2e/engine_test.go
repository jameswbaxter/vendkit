// Coverage for the DR-0016 consumer surface built into the binary: the
// `vendkit self-verify` engine-pin check and the `vendkit handler <scm>`
// reference delivery handlers (handler-protocol dispatch, no network path).
package e2e

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// vkStdin drives the vendkit binary with an intent document on stdin.
func vkStdin(t *testing.T, dir string, env map[string]string, stdin string, args ...string) (string, string, int) {
	t.Helper()
	cmd := exec.Command(vendkitBin, args...)
	cmd.Dir = dir
	cmd.Env = mergedEnv(env)
	cmd.Stdin = strings.NewReader(stdin)
	var so, se bytes.Buffer
	cmd.Stdout, cmd.Stderr = &so, &se
	err := cmd.Run()
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			return so.String(), se.String(), ee.ExitCode()
		}
		t.Fatalf("run vendkit %v: %v", args, err)
	}
	return so.String(), so.String() + se.String(), 0
}

func sha256Hex(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func TestSelfVerifyEnforcesTheEnginePin(t *testing.T) {
	_, con := world(t)
	cfgPath := filepath.Join(con, ".vendkit", "docs.yml")
	platform := runtime.GOOS + "/" + runtime.GOARCH
	if !strings.Contains(read(t, cfgPath), platform+`: ""`) {
		t.Skipf("host platform %s has no scaffolded engine.sha256 slot", platform)
	}

	// Onboard leaves engine.sha256 blank — self-verify is advisory, not a fail.
	so, _, code := vk(t, con, nil, true, "self-verify", "--slice", "docs")
	if code != 0 {
		t.Fatalf("blank pin should pass advisory, exit = %d", code)
	}
	mustContain(t, so, "self-verify=unpinned")

	// Record the running engine's real checksum → self-verify passes cleanly.
	fill := func(hash string) {
		cfg := read(t, cfgPath)
		out := strings.Replace(cfg, platform+`: ""`, platform+`: "`+hash+`"`, 1)
		if out == cfg {
			t.Fatalf("no sha256 slot for %s in config:\n%s", platform, cfg)
		}
		write(t, cfgPath, out)
	}
	fill(sha256Hex(t, vendkitBin))
	so, _, _ = vk(t, con, nil, true, "self-verify", "--slice", "docs")
	mustContain(t, so, "self-verify=ok")

	// A wrong checksum is a loud infrastructure failure (exit >=4).
	write(t, cfgPath, strings.Replace(read(t, cfgPath),
		platform+`: "`+sha256Hex(t, vendkitBin)+`"`,
		platform+`: "`+strings.Repeat("0", 64)+`"`, 1))
	_, se, code := vk(t, con, nil, false, "self-verify", "--slice", "docs")
	if code < 4 {
		t.Fatalf("mismatched pin exit = %d, want >=4", code)
	}
	mustContain(t, se, "does not match")
}

func TestHandlerProtocolDispatch(t *testing.T) {
	dir := t.TempDir()
	// An UNRECOGNISED fact key needs no network or credentials and yields the
	// neutral verdict=unknown (forward-compatible non-failing path). Real
	// per-fact API verification is covered by the httptest contract tests in
	// handler_test.go (github required-check, ado branch policies).
	for _, scm := range []string{"github", "ado"} {
		intent := `{"vendkit_handler_protocol":1,"kind":"fact-verify","fact":"x","slice":"docs"}`
		so, _, code := vkStdin(t, dir, nil, intent, "handler", scm)
		if code != 0 {
			t.Fatalf("handler %s fact-verify exit = %d", scm, code)
		}
		mustContain(t, so, "verdict=unknown")
	}

	// Wrong protocol version is rejected loudly.
	if _, _, code := vkStdin(t, dir, nil,
		`{"vendkit_handler_protocol":2,"kind":"fact-verify"}`, "handler", "github"); code == 0 {
		t.Fatal("unsupported protocol version must fail")
	}

	// An unknown kind is refused before any delivery attempt.
	if _, _, code := vkStdin(t, dir, nil,
		`{"vendkit_handler_protocol":1,"kind":"bogus"}`, "handler", "github"); code == 0 {
		t.Fatal("unknown kind must fail")
	}

	// An unknown scm is a usage error (exit 2).
	if _, _, code := vkStdin(t, dir, nil,
		`{"vendkit_handler_protocol":1,"kind":"fact-verify"}`, "handler", "bogus"); code != 2 {
		t.Fatalf("unknown scm exit = %d, want 2", code)
	}
}
