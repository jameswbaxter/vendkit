// Package core is the engine: a faithful port of the Python reference
// (DR-0017). Behavioural contracts — CLI outputs, exit codes, glob dialects,
// normalisation, canonical JSON — are pinned by tests/vectors/ and the
// scenario kit; where this file comments "fnmatch semantics" etc., the
// vectors are the authority.
package core

import (
	"fmt"
	"os/exec"
	"regexp"
	"strings"
)

// -- error model (cli spec exit codes) ---------------------------------------

// VendkitError: infrastructure failure. CLI maps to exit >= 4.
type VendkitError struct{ Msg string }

func (e *VendkitError) Error() string { return e.Msg }

// UsageError: bad arguments or config. CLI maps to exit 2.
type UsageError struct{ Msg string }

func (e *UsageError) Error() string { return e.Msg }

// Refusal: deliberate refusal (retracted target, tag moved…). Exit 3;
// Reason is the stable machine token emitted as `refused=<reason>`.
type Refusal struct {
	Reason string
	Msg    string
}

func (e *Refusal) Error() string { return e.Msg }

func Usagef(format string, a ...any) error {
	return &UsageError{Msg: fmt.Sprintf(format, a...)}
}

func Errf(format string, a ...any) error {
	return &VendkitError{Msg: fmt.Sprintf(format, a...)}
}

// -- THE fnmatch glob matcher --------------------------------------------------
// Python fnmatch.fnmatchcase semantics, pinned by tests/vectors/
// fnmatch-globs.json: '*' and '?' cross '/', dotfiles match, case-sensitive,
// [seq] classes with '!' negation.

var fnmatchCache = map[string]*regexp.Regexp{}

func fnmatchTranslate(pattern string) *regexp.Regexp {
	if rx, ok := fnmatchCache[pattern]; ok {
		return rx
	}
	var b strings.Builder
	b.WriteString(`(?s)\A`)
	runes := []rune(pattern)
	for i := 0; i < len(runes); i++ {
		c := runes[i]
		switch c {
		case '*':
			b.WriteString(`.*`)
		case '?':
			b.WriteString(`.`)
		case '[':
			j := i + 1
			if j < len(runes) && runes[j] == '!' {
				j++
			}
			if j < len(runes) && runes[j] == ']' {
				j++
			}
			for j < len(runes) && runes[j] != ']' {
				j++
			}
			if j >= len(runes) {
				b.WriteString(`\[`)
			} else {
				inner := string(runes[i+1 : j])
				inner = strings.ReplaceAll(inner, `\`, `\\`)
				if strings.HasPrefix(inner, "!") {
					inner = "^" + inner[1:]
				} else if strings.HasPrefix(inner, "^") {
					inner = `\` + inner
				}
				b.WriteString("[" + inner + "]")
				i = j
			}
		default:
			b.WriteString(regexp.QuoteMeta(string(c)))
		}
	}
	b.WriteString(`\z`)
	rx := regexp.MustCompile(b.String())
	fnmatchCache[pattern] = rx
	return rx
}

// PathMatch is the one glob matcher (util.path_match): resolver, migration
// verifier and gate must all use this implementation.
func PathMatch(path, pattern string) bool {
	return fnmatchTranslate(pattern).MatchString(path)
}

func MatchAny(path string, patterns []string) bool {
	for _, p := range patterns {
		if PathMatch(path, p) {
			return true
		}
	}
	return false
}

// -- git ----------------------------------------------------------------------

// RunGit runs git, returning trimmed stdout; *VendkitError on failure.
func RunGit(args []string, cwd string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = cwd
	out, err := cmd.Output()
	if err != nil {
		detail := ""
		if ee, ok := err.(*exec.ExitError); ok {
			detail = strings.TrimSpace(string(ee.Stderr))
		} else {
			detail = err.Error()
		}
		return "", Errf("git %s failed in %s: %s", strings.Join(args, " "), cwd, detail)
	}
	return strings.TrimSpace(string(out)), nil
}
