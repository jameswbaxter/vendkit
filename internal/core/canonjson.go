// Canonical JSON serialisation (manifest spec §1): sorted keys, 2-space
// indent, ensure-ASCII escaping, no HTML escaping, single trailing newline —
// byte-identical to the Python reference (json.dumps(..., indent=2,
// sort_keys=True)); pinned by tests/vectors/canonical-manifest.*.

package core

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
)

func CanonicalJSON(v any) []byte {
	var b strings.Builder
	writeCanon(&b, v, 0)
	b.WriteString("\n")
	return []byte(b.String())
}

func writeCanon(b *strings.Builder, v any, depth int) {
	pad := strings.Repeat("  ", depth)
	childPad := strings.Repeat("  ", depth+1)
	switch t := v.(type) {
	case nil:
		b.WriteString("null")
	case bool:
		if t {
			b.WriteString("true")
		} else {
			b.WriteString("false")
		}
	case string:
		writeJSONString(b, t)
	case int:
		b.WriteString(strconv.Itoa(t))
	case int64:
		b.WriteString(strconv.FormatInt(t, 10))
	case float64:
		if t == math.Trunc(t) && math.Abs(t) < 1e15 {
			b.WriteString(strconv.FormatInt(int64(t), 10))
		} else {
			b.WriteString(strconv.FormatFloat(t, 'g', -1, 64))
		}
	case map[string]any:
		if len(t) == 0 {
			b.WriteString("{}")
			return
		}
		keys := make([]string, 0, len(t))
		for k := range t {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		b.WriteString("{\n")
		for i, k := range keys {
			b.WriteString(childPad)
			writeJSONString(b, k)
			b.WriteString(": ")
			writeCanon(b, t[k], depth+1)
			if i < len(keys)-1 {
				b.WriteString(",")
			}
			b.WriteString("\n")
		}
		b.WriteString(pad + "}")
	case []any:
		if len(t) == 0 {
			b.WriteString("[]")
			return
		}
		b.WriteString("[\n")
		for i, item := range t {
			b.WriteString(childPad)
			writeCanon(b, item, depth+1)
			if i < len(t)-1 {
				b.WriteString(",")
			}
			b.WriteString("\n")
		}
		b.WriteString(pad + "]")
	default:
		panic(fmt.Sprintf("canonical JSON: unsupported type %T", v))
	}
}

// writeJSONString mirrors Python's ensure_ascii encoder: \" \\ \n \r \t
// \b \f, other control chars and ALL non-ASCII as \uXXXX (surrogate pairs
// beyond the BMP), forward slash NOT escaped, <>& NOT escaped.
func writeJSONString(b *strings.Builder, s string) {
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		case '\b':
			b.WriteString(`\b`)
		case '\f':
			b.WriteString(`\f`)
		default:
			switch {
			case r < 0x20:
				fmt.Fprintf(b, `\u%04x`, r)
			case r < 0x7f:
				b.WriteRune(r)
			case r <= 0xffff:
				fmt.Fprintf(b, `\u%04x`, r)
			default:
				r -= 0x10000
				fmt.Fprintf(b, `\u%04x\u%04x`, 0xd800+(r>>10), 0xdc00+(r&0x3ff))
			}
		}
	}
	b.WriteByte('"')
}
