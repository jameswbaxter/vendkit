// The vendkit CLI (cli spec). One entrypoint; uniform conventions.
// Exit codes: 0 ok · 1 strict findings · 2 usage/config · 3 refusal ·
// >=4 infrastructure. Contractually identical to the reference CLI
// (DR-0017): same flags, same key=value outputs, same refusal tokens.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/jameswbaxter/vendkit/internal/ci"
	"github.com/jameswbaxter/vendkit/internal/core"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	platform := ""
	for len(args) > 0 {
		if args[0] == "--platform" && len(args) > 1 {
			platform = args[1]
			args = args[2:]
			continue
		}
		if v, ok := strings.CutPrefix(args[0], "--platform="); ok {
			platform = v
			args = args[1:]
			continue
		}
		break
	}
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage error: a command is required "+
			"(generate, gate, sync, sync-pipeline, release, watch, migrations, "+
			"migrations-verify, conformance, fleet, self-verify, handler, "+
			"push-hint, init, status, diff, update, explain)")
		return 2
	}
	cmd, rest := args[0], args[1:]

	surface, err := ci.GetSurface(platform)
	if err != nil {
		fmt.Fprintf(os.Stderr, "usage error: %v\n", err)
		return 2
	}

	var code int
	switch cmd {
	case "generate":
		code, err = cmdGenerate(rest, surface)
	case "gate":
		code, err = cmdGate(rest, surface)
	case "sync":
		code, err = cmdSync(rest, surface)
	case "sync-pipeline":
		code, err = cmdSyncPipeline(rest, surface)
	case "release":
		code, err = cmdRelease(rest, surface)
	case "watch":
		code, err = cmdWatch(rest, surface)
	case "migrations":
		code, err = cmdMigrations(rest, surface)
	case "migrations-verify":
		code, err = cmdMigrationsVerify(rest, surface)
	case "conformance":
		code, err = cmdConformance(rest, surface)
	case "fleet":
		code, err = cmdFleet(rest, surface)
	case "self-verify":
		code, err = cmdSelfVerify(rest, surface)
	case "handler":
		code, err = cmdHandler(rest, surface)
	case "push-hint":
		code, err = cmdPushHint(rest, surface)
	case "init", "onboard":
		code, err = cmdInit(rest, surface)
	case "status":
		code, err = cmdStatus(rest, surface)
	case "diff":
		code, err = cmdDiff(rest, surface)
	case "update":
		code, err = cmdUpdate(rest, surface)
	case "explain":
		code, err = cmdExplain(rest, surface)
	default:
		fmt.Fprintf(os.Stderr, "usage error: unknown command %q\n", cmd)
		return 2
	}
	if err != nil {
		var usage *core.UsageError
		var refusal *core.Refusal
		var vendkit *core.VendkitError
		switch {
		case errors.As(err, &refusal):
			fmt.Printf("refused=%s\n", refusal.Reason)
			fmt.Fprintf(os.Stderr, "REFUSED: %s\n", refusal.Msg)
			return 3
		case errors.As(err, &usage):
			fmt.Fprintf(os.Stderr, "usage error: %s\n", usage.Msg)
			return 2
		case errors.As(err, &vendkit):
			fmt.Fprintf(os.Stderr, "error: %s\n", vendkit.Msg)
			return 4
		default:
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			return 4
		}
	}
	return code
}

// -- flag plumbing ------------------------------------------------------------

type commonFlags struct {
	ExportDecl    string
	ConsumerRoot  string
	PublisherRoot string
	JSON          bool
}

func newFlagSet(name string) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	return fs
}

func addCommon(fs *flag.FlagSet, c *commonFlags, decl, consumer, publisher bool) {
	if decl {
		fs.StringVar(&c.ExportDecl, "export-decl", core.DefaultDecl, "")
	}
	if consumer {
		fs.StringVar(&c.ConsumerRoot, "consumer-root", ".", "")
	}
	if publisher {
		fs.StringVar(&c.PublisherRoot, "publisher-root", ".", "")
	}
	fs.BoolVar(&c.JSON, "json", false, "")
}

func parseFlags(fs *flag.FlagSet, args []string) error {
	if err := fs.Parse(args); err != nil {
		return core.Usagef("%s: %v", fs.Name(), err)
	}
	return nil
}

// loadDeclFrom resolves the export declaration like the reference CLI: it
// lives in the publisher checkout unless --export-decl points elsewhere.
func loadDeclFrom(publisherRoot, exportDecl string) (*core.ExportDecl, error) {
	path := filepath.Join(publisherRoot, core.DefaultDecl)
	if exportDecl != "" && exportDecl != core.DefaultDecl {
		path = exportDecl
	}
	return core.LoadExportDecl(path)
}
