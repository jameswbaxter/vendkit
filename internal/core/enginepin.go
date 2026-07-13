// Engine-pin advancement (DR-0016 §3): the sync PR that advances the content
// pin also advances the engine pin — one reviewed change, no skew window. The
// version moves to the target; the per-platform sha256 values are blanked,
// because the sync runner cannot know the target binary's checksums offline.
// Blank is advisory (self-verify skips it) — the reviewer refills them from the
// target release's SHA256SUMS.txt as part of accepting the bump.
package core

import (
	"os"
	"regexp"
)

var (
	// engine.version is emitted at two-space indent; nothing else in the
	// slice config carries a `version:` key.
	engineVersionRx = regexp.MustCompile(`(?m)^(  version: ).*$`)
	// sha256 platform entries are `<goos>/<goarch>: "<hex>"` at four-space
	// indent under the engine.sha256 map.
	engineSHAEntryRx = regexp.MustCompile(`(?m)^(    [a-z0-9]+/[a-z0-9]+: )".*"$`)
)

// AdvanceEnginePin rewrites the slice config's engine block in place: version →
// target, sha256 values → blank. A no-op when the slice has no engine pin.
func AdvanceEnginePin(cfg *SliceConfig, target string) error {
	if cfg.EngineVersion == "" {
		return nil
	}
	data, err := os.ReadFile(cfg.Path)
	if err != nil {
		return Errf("advance engine pin: read %s: %v", cfg.Path, err)
	}
	out := engineVersionRx.ReplaceAllString(string(data), "${1}"+target)
	out = engineSHAEntryRx.ReplaceAllString(out, `${1}""`)
	if err := os.WriteFile(cfg.Path, []byte(out), 0o644); err != nil {
		return Errf("advance engine pin: write %s: %v", cfg.Path, err)
	}
	return nil
}
