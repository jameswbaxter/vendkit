// Handler invocation — the core side of the handler protocol (DR-0014).

package core

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"strings"
)

const HandlerProtocolVersion = 1

// ResolveHandler: the command for a handler kind, or nil when unwired.
// Resolution: VENDKIT_HANDLER_<KIND> env override (shell-word split), then
// the slice config's handlers.<kind>.exec.
func ResolveHandler(kind string, cfg *SliceConfig) []string {
	envKey := "VENDKIT_HANDLER_" + strings.ToUpper(strings.ReplaceAll(kind, "-", "_"))
	if env := os.Getenv(envKey); env != "" {
		return strings.Fields(env)
	}
	if cfg != nil {
		if spec, ok := cfg.Handlers[kind]; ok {
			return spec.Exec
		}
	}
	return nil
}

// InvokeHandler runs a handler: intent JSON on stdin, `key=value` facts on
// stdout. Exit 0 = delivered; nonzero = infrastructure failure (loud).
func InvokeHandler(command []string, kind string, payload map[string]any,
	cwd string) (map[string]string, error) {
	document := map[string]any{
		"vendkit_handler_protocol": HandlerProtocolVersion,
		"kind":                     kind,
	}
	for k, v := range payload {
		document[k] = v
	}
	input, err := json.Marshal(document)
	if err != nil {
		return nil, Errf("marshal handler intent: %v", err)
	}
	cmd := exec.Command(command[0], command[1:]...)
	cmd.Dir = cwd
	cmd.Stdin = bytes.NewReader(input)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		detail := strings.TrimSpace(stderr.String())
		if len(detail) > 500 {
			detail = detail[:500]
		}
		return nil, Errf("handler %s (%s) failed: %s", command[0], kind, detail)
	}
	facts := map[string]string{}
	for _, line := range strings.Split(stdout.String(), "\n") {
		key, value, found := strings.Cut(line, "=")
		if found && key != "" && !strings.Contains(key, " ") {
			facts[key] = value
		}
	}
	return facts, nil
}
