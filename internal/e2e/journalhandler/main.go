// journalhandler is the neutral reference PR/handoff/fact-verify handler for
// the scenario kit — the Go analogue of the deleted vendkit.handlers.journal
// Python module. It records every intent to VENDKIT_NEUTRAL_JOURNAL (one JSON
// object per line) and reports a synthetic URL, so the e2e tests have a
// no-network delivery assertion point (handler-protocol spec §6, DR-0014).
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
)

func main() {
	os.Exit(run())
}

func run() int {
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		fmt.Fprintf(os.Stderr, "journal handler: read stdin: %v\n", err)
		return 1
	}
	var intent map[string]any
	if err := json.Unmarshal(data, &intent); err != nil {
		fmt.Fprintf(os.Stderr, "journal handler: stdin is not a JSON intent: %v\n", err)
		return 1
	}
	if v, _ := intent["vendkit_handler_protocol"].(float64); int(v) != 1 {
		fmt.Fprintf(os.Stderr, "journal handler: unsupported protocol %v\n",
			intent["vendkit_handler_protocol"])
		return 1
	}
	if path := os.Getenv("VENDKIT_NEUTRAL_JOURNAL"); path != "" {
		fh, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			fmt.Fprintf(os.Stderr, "journal handler: open journal: %v\n", err)
			return 1
		}
		// Re-marshalling a map yields keys in sorted order (as Python's
		// json.dumps(sort_keys=True) did); the fields are unchanged.
		line, _ := json.Marshal(intent)
		if _, err := fh.Write(append(line, '\n')); err != nil {
			fh.Close()
			fmt.Fprintf(os.Stderr, "journal handler: write journal: %v\n", err)
			return 1
		}
		fh.Close()
	}
	switch intent["kind"] {
	case "pr":
		hb, _ := intent["head_branch"].(string)
		fmt.Printf("url=neutral://pr/%s\n", hb)
		fmt.Println("number=0")
	case "handoff":
		dk, _ := intent["dedup_key"].(string)
		fmt.Printf("url=neutral://item/%s\n", dk)
	default:
		fmt.Println("verdict=unknown")
	}
	return 0
}
