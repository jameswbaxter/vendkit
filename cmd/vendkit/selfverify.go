// `vendkit self-verify` re-asserts the running engine binary against the
// consumer-held checksum pin (DR-0016 §2). The scaffolded pipeline calls it
// right after fetching + checksum-verifying the binary, so a swapped-at-source
// or wrong-version engine fails loudly before it materialises anything. A
// platform whose engine.sha256 is blank is advisory (skipped) — the fetch step
// still verified the download against the release's own SHA256SUMS.txt.
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"runtime"

	"github.com/jameswbaxter/vendkit/internal/ci"
	"github.com/jameswbaxter/vendkit/internal/core"
)

func cmdSelfVerify(args []string, surface ci.Surface) (int, error) {
	fs := newFlagSet("self-verify")
	var c commonFlags
	slice := fs.String("slice", "", "")
	addCommon(fs, &c, false, true, false)
	if err := parseFlags(fs, args); err != nil {
		return 0, err
	}
	cfg, err := sliceOrOnly(c.ConsumerRoot, *slice)
	if err != nil {
		return 0, err
	}
	platform := runtime.GOOS + "/" + runtime.GOARCH
	if cfg.EngineVersion == "" {
		// No engine pin (e.g. ci: none) — nothing to assert.
		surface.EmitOutput("self-verify", "unpinned")
		return 0, nil
	}
	want := cfg.EngineSHA256[platform]
	if want == "" {
		surface.EmitError("no engine.sha256 recorded for " + platform +
			" — self-verify is advisory until it is filled (DR-0016)")
		surface.EmitOutput("self-verify", "unpinned")
		return 0, nil
	}
	exe, err := os.Executable()
	if err != nil {
		return 0, core.Errf("self-verify: locate running engine: %v", err)
	}
	sum, err := sha256File(exe)
	if err != nil {
		return 0, core.Errf("self-verify: %v", err)
	}
	if sum != want {
		// Loud infrastructure failure (exit >=4): the running engine is not
		// the pinned one.
		return 0, core.Errf("self-verify: running engine sha256 %s does not "+
			"match the pin %s for %s (engine %s) — refusing to proceed",
			sum, want, platform, cfg.EngineVersion)
	}
	surface.EmitOutput("self-verify", "ok")
	return 0, nil
}

func sha256File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", core.Errf("open %s: %v", path, err)
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", core.Errf("hash %s: %v", path, err)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
