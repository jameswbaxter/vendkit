// `vendkit push-hint` — the publisher-side dispatch step (platform-integration
// spec §4, DR-0006). After a release is published, it nudges the subscribers
// listed in the publisher-held subscribers file so their sync pipelines run
// early instead of waiting for the next scheduled poll. Pull remains the
// source of truth; a lost hint costs latency, not correctness — so this
// command is best-effort: one subscriber failing never aborts the rest, and
// dispatch failures are warnings, not a red exit.
//
// Core reads the subscribers file and composes ONE `push-hint` intent per GHA
// subscriber; the actual repository_dispatch POST lives in the handler
// (DR-0014: core calls no vendor API itself). Non-GHA subscribers are skipped
// with a logged note (ADO push is the consumer's own resources.pipelines
// trigger; `none` has no push target).
package main

import (
	"fmt"
	"os"

	"github.com/jameswbaxter/vendkit/internal/ci"
	"github.com/jameswbaxter/vendkit/internal/core"
)

func cmdPushHint(args []string, surface ci.Surface) (int, error) {
	fs := newFlagSet("push-hint")
	var c commonFlags
	subscribersPath := fs.String("subscribers", core.DefaultSubscribersPath, "")
	version := fs.String("version", "", "")
	publisherRepo := fs.String("publisher-repo", "", "")
	addCommon(fs, &c, false, false, true)
	if err := parseFlags(fs, args); err != nil {
		return 0, err
	}

	// Missing subscribers file = push hints not configured. This is the
	// opt-out, not an error: emit a note and exit 0 so a release workflow can
	// run the step unconditionally (it stays a no-op until a consumer PR adds
	// the file).
	if _, err := os.Stat(*subscribersPath); os.IsNotExist(err) {
		surface.EmitOutput("subscribers", "0")
		surface.EmitOutput("dispatched", "0")
		return 0, nil
	}

	subs, err := core.LoadSubscribers(*subscribersPath)
	if err != nil {
		return 0, err // usage error (exit 2): a malformed file is loud
	}

	// The release being announced: explicit --version, else the publisher
	// checkout's exact tag (the release workflow passes the tag directly).
	resolved := *version
	if resolved == "" {
		resolved, err = publisherTag(c.PublisherRoot)
		if err != nil {
			return 0, core.Usagef("--version is required (the release tag to " +
				"announce) when the publisher checkout is not at a release tag")
		}
	}

	// The publisher's own coordinate travels in the payload so tier-chain hops
	// know who nudged them. Default to the ambient GitHub coordinate.
	pubRepo := *publisherRepo
	if pubRepo == "" {
		pubRepo = os.Getenv("GITHUB_REPOSITORY")
	}

	command := pushHintHandler()

	var dispatched, skipped, failed int
	for _, sub := range subs {
		if !sub.IsGHA() {
			// ADO push is pull-based (its own resources.pipelines trigger);
			// `none` has no push target. Skip with a visible note.
			skipped++
			// Expected, not a failure: route through the summary channel so it
			// does not surface as a red ::error:: annotation on the release run.
			surface.EmitSummary("skip " + sub.Repo + " (" + sub.Platform +
				"): non-GHA subscribers are nudged by their own pull trigger, " +
				"not the publisher")
			continue
		}
		// Per-subscriber dispatch-scoped token: the named env var is handed to
		// the handler as VENDKIT_TOKEN_PUSH_HINT. Empty token_secret falls
		// through to the ambient VENDKIT_TOKEN_PUSH_HINT (the handler resolves
		// it). A named-but-empty secret is a per-subscriber warning, not fatal.
		var extraEnv []string
		if sub.TokenSecret != "" {
			val := os.Getenv(sub.TokenSecret)
			if val == "" {
				failed++
				surface.EmitError("skip " + sub.Repo + ": token secret " +
					sub.TokenSecret + " is unset in the environment")
				continue
			}
			extraEnv = []string{"VENDKIT_TOKEN_PUSH_HINT=" + val}
		}
		intent := map[string]any{
			"repo":       sub.Repo,
			"event_type": sub.EventType,
			"client_payload": map[string]any{
				"version":   resolved,
				"tag":       resolved,
				"publisher": pubRepo,
			},
		}
		if _, err := core.InvokeHandlerEnv(command, "push-hint", intent,
			c.PublisherRoot, extraEnv); err != nil {
			// Best-effort: a failed hint is a warning, never a red release.
			failed++
			surface.EmitError("push-hint to " + sub.Repo + " failed: " + err.Error())
			continue
		}
		dispatched++
		surface.EmitOutput("dispatched-"+sub.Repo, sub.EventType)
	}

	surface.EmitOutput("subscribers", fmt.Sprint(len(subs)))
	surface.EmitOutput("dispatched", fmt.Sprint(dispatched))
	surface.EmitOutput("skipped", fmt.Sprint(skipped))
	surface.EmitOutput("failed", fmt.Sprint(failed))
	// Exit 0 even when hints failed: push is a latency optimisation, not the
	// reconciler (DR-0006). Failures are surfaced as warnings above.
	return 0, nil
}

// pushHintHandler resolves the push-hint handler command. Unlike consumer-side
// kinds there is no slice config on the publisher, so resolution is: the
// VENDKIT_HANDLER_PUSH_HINT env override, else the built-in reference GitHub
// handler (this same binary — DR-0016).
func pushHintHandler() []string {
	if cmd := core.ResolveHandler("push-hint", nil); cmd != nil {
		return cmd
	}
	exe, err := os.Executable()
	if err != nil || exe == "" {
		exe = "vendkit"
	}
	return []string{exe, "handler", "github"}
}
