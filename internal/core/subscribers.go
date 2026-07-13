// Publisher-held subscribers file for GHA push hints (platform-integration
// spec §4, DR-0006). Pull remains the source of truth; this file only tells
// the publisher's release workflow which consumers to *nudge* early. It is
// maintained by consumer PRs to the publisher repo (opt-in, self-service,
// auditable) — the one place "the publisher knows no downstream" is relaxed.
//
// The file is publisher-side config, not a consumer slice config: it is read
// only by `vendkit push-hint`, never by the gate/sync/watch lanes.

package core

import (
	"path/filepath"
	"strconv"
	"strings"
)

// DefaultSubscribersPath is where `vendkit push-hint` looks by default.
var DefaultSubscribersPath = filepath.Join(VendkitDir, "subscribers.yml")

// DefaultPushHintEventType is the repository_dispatch event type the
// scaffolded GHA receiver subscribes to.
const DefaultPushHintEventType = "vendkit-release"

// Subscriber is one downstream consumer to hint on release.
type Subscriber struct {
	// Repo is the consumer's SCM coordinate. For github-actions this is the
	// `owner/name` repository the repository_dispatch is POSTed to.
	Repo string
	// EventType is the repository_dispatch event type; defaults to
	// "vendkit-release" (the type the scaffolded receiver listens for).
	EventType string
	// TokenSecret optionally names the environment variable holding this
	// subscriber's dispatch-scoped token. Empty => the ambient
	// VENDKIT_TOKEN_PUSH_HINT is used (the handler resolves it).
	TokenSecret string
	// Platform is the consumer's CI (github-actions | azure-pipelines | none).
	// Defaults to github-actions. Only github-actions needs a sender; ADO push
	// is pull-based via the consumer's own resources.pipelines trigger, and
	// `none` has no push target — non-GHA entries are skipped by the dispatch
	// step with a logged note.
	Platform string
}

// IsGHA reports whether this subscriber needs the (GHA-only) sender.
func (s Subscriber) IsGHA() bool { return s.Platform == "github-actions" }

// LoadSubscribers reads and validates the publisher subscribers file. It fails
// loudly (usage error) on any malformed entry rather than silently skipping —
// the same strictness the .vendkit/ namespace uses (DR-0012).
func LoadSubscribers(path string) ([]Subscriber, error) {
	data, err := LoadYAML(path)
	if err != nil {
		return nil, err
	}
	if !schemaVersionIs(data, 1) {
		return nil, Usagef("%s: schema_version must be 1", path)
	}
	raw := getList(data, "subscribers")
	if raw == nil {
		return nil, Usagef("%s: a 'subscribers' list is required", path)
	}
	var out []Subscriber
	var errs []string
	for i, item := range raw {
		entry, ok := item.(map[string]any)
		if !ok {
			errs = append(errs, itemErr(i, "must be a mapping"))
			continue
		}
		sub := Subscriber{
			Repo:        strings.TrimSpace(getStr(entry, "repo")),
			EventType:   getStr(entry, "event_type"),
			TokenSecret: getStr(entry, "token_secret"),
			Platform:    getStr(entry, "platform"),
		}
		if sub.Platform == "" {
			sub.Platform = "github-actions"
		}
		if sub.EventType == "" {
			sub.EventType = DefaultPushHintEventType
		}
		if !contains(CIValues, sub.Platform) {
			errs = append(errs, itemErr(i,
				"platform must be one of "+strings.Join(CIValues, "|")))
			continue
		}
		// A GHA subscriber must name an owner/name repo — that is the POST
		// target. Non-GHA entries are pull-based, so repo is advisory there.
		if sub.IsGHA() {
			if sub.Repo == "" {
				errs = append(errs, itemErr(i, "repo is required for github-actions subscribers"))
				continue
			}
			if o, n, _ := strings.Cut(sub.Repo, "/"); o == "" || n == "" || strings.Contains(n, "/") {
				errs = append(errs, itemErr(i, "repo must be 'owner/name', got "+sub.Repo))
				continue
			}
		}
		out = append(out, sub)
	}
	if len(errs) > 0 {
		return nil, Usagef("%s: %s", path, strings.Join(errs, "; "))
	}
	return out, nil
}

func itemErr(i int, msg string) string {
	return "subscribers[" + strconv.Itoa(i) + "]: " + msg
}
