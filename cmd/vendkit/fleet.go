// Fleet audit (conformance spec §5): read-only aggregation over consumer
// conformance documents. The clone-and-run over many repos is a scheduled
// external job (Layer 3, optional); this command's job is the AGGREGATION —
// fold N `conformance --json` documents into one fleet report. It clones
// nothing, fetches nothing, and calls no network/SCM API (the audit never
// inverts the trust model).
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"

	"github.com/jameswbaxter/vendkit/internal/ci"
	"github.com/jameswbaxter/vendkit/internal/core"
)

// fleetStatusOrder lists worst-status buckets worst-first for the human census.
var fleetStatusOrder = []string{"error", "fail", "attested", "skipped", "waived", "pass"}

// cmdFleet aggregates conformance documents into one fleet report.
//
// Input contract: each positional arg is a file (one document, or a JSON array
// of documents) or a directory (every *.json inside it, sorted). With no args,
// documents are read from stdin — a JSON array, or one-or-more JSON objects
// (newline-delimited / concatenated). A file that is not valid conformance JSON
// fails loudly, named.
//
// Output: a human summary always; `--json` additionally emits the fleet-level
// interchange document. Machine facts `consumers=` and `total-gaps=` go to the
// CI surface. Read-only: exit 0 (advisory), never a network call.
func cmdFleet(args []string, surface ci.Surface) (int, error) {
	fs := newFlagSet("fleet")
	var c commonFlags
	addCommon(fs, &c, false, false, false) // registers only --json
	// Positional paths may be interspersed with flags: the stdlib flag parser
	// stops at the first operand, so drain flags and operands in a loop.
	var paths []string
	rest := args
	for len(rest) > 0 {
		if err := parseFlags(fs, rest); err != nil {
			return 0, err
		}
		rest = fs.Args()
		if len(rest) == 0 {
			break
		}
		paths = append(paths, rest[0])
		rest = rest[1:]
	}
	docs, err := loadFleetDocs(paths, os.Stdin)
	if err != nil {
		return 0, err
	}
	if len(docs) == 0 {
		return 0, core.Usagef("no conformance documents provided " +
			"(pass file/directory args, or pipe conformance --json on stdin)")
	}
	report := core.AggregateFleet(docs)

	printFleetHuman(report)
	surface.EmitOutput("consumers", fmt.Sprint(report.TotalConsumers))
	surface.EmitOutput("total-gaps", fmt.Sprint(report.TotalGaps))
	if c.JSON {
		out, _ := json.Marshal(report)
		fmt.Println(string(out))
	}
	return 0, nil
}

func printFleetHuman(r *core.FleetReport) {
	fmt.Printf("fleet: %d consumer(s), %d total gap(s)\n",
		r.TotalConsumers, r.TotalGaps)
	fmt.Print("by worst status:")
	for _, st := range fleetStatusOrder {
		if n := r.ByWorstStatus[st]; n > 0 {
			fmt.Printf(" %s=%d", st, n)
		}
	}
	fmt.Print("\n\n")
	fmt.Printf("%-9s %-16s %-12s %-4s %-5s %s\n",
		"WORST", "SLICE", "PIN", "LAG", "GAPS", "PROFILE")
	for _, row := range r.Consumers {
		pin := "-"
		if row.Pin != nil && row.Pin.Version != "" {
			pin = row.Pin.Version
		}
		lag := "-" // null offline: no network to learn latest (spec §5)
		if row.PinLag != nil {
			lag = fmt.Sprint(*row.PinLag)
		}
		fmt.Printf("%-9s %-16s %-12s %-4s %-5d %s\n",
			row.WorstStatus, row.Slice, pin, lag, row.GapCount, row.Profile)
	}
}

// loadFleetDocs resolves the input contract into a flat list of documents.
func loadFleetDocs(paths []string, stdin io.Reader) ([]*core.ConformanceDoc, error) {
	if len(paths) == 0 {
		data, err := io.ReadAll(stdin)
		if err != nil {
			return nil, core.Errf("read stdin: %v", err)
		}
		return decodeFleetDocs("<stdin>", data)
	}
	var out []*core.ConformanceDoc
	for _, p := range paths {
		fi, err := os.Stat(p)
		if err != nil {
			return nil, core.Usagef("cannot read %s: %v", p, err)
		}
		var files []string
		if fi.IsDir() {
			hits, _ := filepath.Glob(filepath.Join(p, "*.json"))
			sort.Strings(hits)
			if len(hits) == 0 {
				return nil, core.Usagef(
					"%s: no *.json conformance documents in directory", p)
			}
			files = hits
		} else {
			files = []string{p}
		}
		for _, f := range files {
			data, err := os.ReadFile(f)
			if err != nil {
				return nil, core.Usagef("cannot read %s: %v", f, err)
			}
			docs, err := decodeFleetDocs(f, data)
			if err != nil {
				return nil, err
			}
			out = append(out, docs...)
		}
	}
	return out, nil
}

// decodeFleetDocs parses one source's bytes: a JSON array of documents, or one
// or more JSON objects (newline-delimited / concatenated). A source that does
// not decode, or yields an object with no "slice", is a named usage error.
func decodeFleetDocs(name string, data []byte) ([]*core.ConformanceDoc, error) {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return nil, nil
	}
	var docs []*core.ConformanceDoc
	if trimmed[0] == '[' {
		if err := json.Unmarshal(trimmed, &docs); err != nil {
			return nil, core.Usagef("%s: not valid conformance JSON: %v", name, err)
		}
	} else {
		dec := json.NewDecoder(bytes.NewReader(trimmed))
		for {
			var d core.ConformanceDoc
			err := dec.Decode(&d)
			if err == io.EOF {
				break
			}
			if err != nil {
				return nil, core.Usagef("%s: not valid conformance JSON: %v", name, err)
			}
			docs = append(docs, &d)
		}
	}
	for _, d := range docs {
		if d == nil || d.Slice == "" {
			return nil, core.Usagef(
				"%s: not a conformance document (missing \"slice\")", name)
		}
	}
	return docs, nil
}
