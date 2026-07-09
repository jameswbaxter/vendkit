// Normalised content hashing (DR-0004, manifest spec §2).

package core

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"unicode/utf8"
)

const Recipe = "utf8;lf;strip-trailing-ws;single-final-newline;sha256"

func sha256hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// NormaliseHash returns (sha256 hexdigest, raw) per normalisation recipe v1.
// raw=true means the bytes are not valid UTF-8; the hash is then over the
// raw bytes. Pinned by tests/vectors/normalisation.json.
func NormaliseHash(data []byte) (string, bool) {
	if !utf8.Valid(data) {
		return sha256hex(data), true
	}
	text := string(data)
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimRight(line, " \t")
	}
	body := strings.Join(lines, "\n")
	body = strings.TrimRight(body, "\n") + "\n"
	return sha256hex([]byte(body)), false
}

// HashAsRecorded hashes honouring a recorded raw flag (never re-guess).
func HashAsRecorded(data []byte, raw bool) string {
	if raw {
		return sha256hex(data)
	}
	digest, nowRaw := NormaliseHash(data)
	if nowRaw {
		// Recorded as text but no longer decodable — hash raw so the
		// comparison fails as `changed` rather than crashing.
		return sha256hex(data)
	}
	return digest
}
