// Package assets embeds the release-shipped data files the engine needs at
// runtime: the scaffold template packs (Layer 3) and the core conformance
// rules. Embedding keeps the binary self-contained (DR-0016/DR-0017).
package assets

import "embed"

//go:embed scaffold conformance/core-rules.yml
var FS embed.FS
