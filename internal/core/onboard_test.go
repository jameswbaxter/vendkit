// Scaffolder outputs for the bootstrap launcher and the reusable engine
// fetch template (DR-0016 amendment: the bootstrap is engine-owned).

package core

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	vendkitassets "github.com/jameswbaxter/vendkit"
)

// gitInitCommit turns a fixture tree into a one-commit git repo — Onboard's
// materialise step records provenance from `git rev-parse HEAD`.
func gitInitCommit(t *testing.T, root string) {
	t.Helper()
	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		cmd.Env = append(os.Environ(),
			"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null",
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@invalid",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@invalid")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-q", "-b", "main")
	run("add", "-A")
	run("commit", "-q", "-m", "fixture")
}

func onboardFixture(t *testing.T) (*ExportDecl, string) {
	t.Helper()
	decl := declFixture(t, "")
	root := rootOf(decl)
	gitInitCommit(t, root)
	return decl, root
}

func onboardParams(ci string) OnboardParams {
	return OnboardParams{CI: ci, SCM: "github", Version: "v0.1.0",
		Mode: "primary", BaseBranch: "main", PRTokenSecret: "VENDKIT_PR_TOKEN"}
}

func TestOnboardWritesLauncherAndFetchTemplate(t *testing.T) {
	decl, pub := onboardFixture(t)
	con := t.TempDir()
	if _, err := Onboard(pub, con, decl, onboardParams("github-actions"),
		vendkitassets.FS); err != nil {
		t.Fatal(err)
	}

	// The launcher: present, executable, byte-identical to the embedded
	// (and therefore released) copy — no placeholders, by design.
	launcher := filepath.Join(con, LauncherName)
	fi, err := os.Stat(launcher)
	if err != nil {
		t.Fatalf("launcher not written: %v", err)
	}
	if fi.Mode()&0o111 == 0 {
		t.Error("launcher is not executable")
	}
	got, err := os.ReadFile(launcher)
	if err != nil {
		t.Fatal(err)
	}
	want, err := vendkitassets.FS.ReadFile("scaffold/" + LauncherName)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(want) {
		t.Error("scaffolded launcher differs from the embedded release copy")
	}

	// The reusable fetch template: rendered, pin line present, listed as a
	// pin file so the sync PR advances it in lockstep.
	action := filepath.Join(con, ".github", "actions", "vendkit-fetch", "action.yml")
	data, err := os.ReadFile(action)
	if err != nil {
		t.Fatalf("fetch template not written: %v", err)
	}
	if !strings.Contains(string(data), "refs/tags/v0.1.0") {
		t.Errorf("fetch template carries no engine pin line:\n%s", data)
	}
	cfg, err := os.ReadFile(filepath.Join(con, VendkitDir, ConsumerSubdir, "docs.yml"))
	if err != nil {
		t.Fatal(err)
	}
	for _, pin := range []string{
		"- .github/workflows/docs-sync.yml",
		"- .github/actions/vendkit-fetch/action.yml",
		"- .github/workflows/vendkit-gate.yml",
	} {
		if !strings.Contains(string(cfg), pin) {
			t.Errorf("slice config pin files missing %q:\n%s", pin, cfg)
		}
	}
}

func TestOnboardNeverClobbersAnExistingLauncher(t *testing.T) {
	decl, pub := onboardFixture(t)
	con := t.TempDir()
	custom := filepath.Join(con, LauncherName)
	if err := os.WriteFile(custom, []byte("#!/bin/sh\n# mine\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	result, err := Onboard(pub, con, decl, onboardParams("github-actions"),
		vendkitassets.FS)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(custom)
	if string(data) != "#!/bin/sh\n# mine\n" {
		t.Error("Onboard overwrote a pre-existing launcher")
	}
	for _, w := range result.Written {
		if w == custom {
			t.Error("pre-existing launcher reported as written")
		}
	}
}

func TestOnboardEmitsEngineBaseURL(t *testing.T) {
	// issue #15: engine.base_url gives the engine pin its own coordinates —
	// scaffolded blank by default (github derivation covers it), carrying the
	// captured value when the cmd layer supplies one, and readable back
	// through the parser either way.
	decl, pub := onboardFixture(t)
	con := t.TempDir()
	p := onboardParams("github-actions")
	p.EngineBaseURL = "https://mirror.example/vendkit"
	if _, err := Onboard(pub, con, decl, p, vendkitassets.FS); err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(con, VendkitDir, ConsumerSubdir, "docs.yml")
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `base_url: "https://mirror.example/vendkit"`) {
		t.Errorf("engine.base_url not scaffolded:\n%s", data)
	}
	cfg, err := LoadSliceConfig(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.EngineBaseURL != "https://mirror.example/vendkit" {
		t.Errorf("EngineBaseURL = %q, want the scaffolded value", cfg.EngineBaseURL)
	}

	con2 := t.TempDir()
	if _, err := Onboard(pub, con2, decl, onboardParams("github-actions"),
		vendkitassets.FS); err != nil {
		t.Fatal(err)
	}
	data2, err := os.ReadFile(filepath.Join(con2, VendkitDir, ConsumerSubdir, "docs.yml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data2), `base_url: ""`) {
		t.Errorf("blank engine.base_url key not scaffolded:\n%s", data2)
	}
}

func TestOnboardCINoneStillWritesLauncher(t *testing.T) {
	// ci: none forgoes pipelines, not the pinned-engine trust path — local
	// gate/conformance runs bootstrap through the launcher.
	decl, pub := onboardFixture(t)
	con := t.TempDir()
	if _, err := Onboard(pub, con, decl, onboardParams("none"),
		vendkitassets.FS); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(con, LauncherName)); err != nil {
		t.Errorf("ci none: launcher not written: %v", err)
	}
	if _, err := os.Stat(filepath.Join(con, ".github")); !os.IsNotExist(err) {
		t.Error("ci none scaffolded pipelines")
	}
}
