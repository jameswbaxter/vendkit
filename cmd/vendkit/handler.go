// Reference delivery handlers, in-binary (DR-0016). `vendkit handler <scm>`
// reads a handler-protocol intent on stdin and dispatches on its `kind`
// (pr / handoff / fact-verify / push-hint), talking to the GitHub or Azure
// DevOps REST API, then writes `key=value` facts on stdout. Any protocol-honouring
// executable may replace it (handler-protocol spec §6); this is the built-in
// reference, ported from the former Python reference-handler modules.
package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/jameswbaxter/vendkit/internal/ci"
	"github.com/jameswbaxter/vendkit/internal/core"
)

// cmdHandler is the `vendkit handler <scm>` entrypoint. It ignores the CI
// surface (handlers speak the stdin/stdout protocol, not emit_output).
func cmdHandler(args []string, _ ci.Surface) (int, error) {
	if len(args) < 1 {
		return 0, core.Usagef("handler requires an scm argument (github|ado)")
	}
	scm := args[0]
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		return 0, core.Errf("handler: read stdin: %v", err)
	}
	var intent map[string]any
	if err := json.Unmarshal(data, &intent); err != nil {
		return 0, core.Errf("handler: stdin is not a JSON intent: %v", err)
	}
	if v, _ := intent["vendkit_handler_protocol"].(float64); int(v) != core.HandlerProtocolVersion {
		return 0, core.Errf("handler: unsupported protocol version %v",
			intent["vendkit_handler_protocol"])
	}
	kind, _ := intent["kind"].(string)
	if kind != "pr" && kind != "handoff" && kind != "fact-verify" && kind != "push-hint" {
		return 0, core.Errf("handler: this handler serves pr/handoff/fact-verify/push-hint, got %q", kind)
	}
	switch scm {
	case "github":
		return 0, githubHandler(kind, intent)
	case "ado":
		return 0, adoHandler(kind, intent)
	default:
		return 0, core.Usagef("unknown handler scm %q (expected github|ado)", scm)
	}
}

// -- shared handler plumbing (former Python _shared) ---------------------------

func emitFact(key, value string) { fmt.Printf("%s=%s\n", key, value) }

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// tokenFor: VENDKIT_TOKEN_<PURPOSE> wins, then the listed vendor conventions.
func tokenFor(purpose string, fallbackEnv ...string) string {
	if v := os.Getenv("VENDKIT_TOKEN_" + strings.ToUpper(purpose)); v != "" {
		return v
	}
	for _, e := range fallbackEnv {
		if v := os.Getenv(e); v != "" {
			return v
		}
	}
	return ""
}

func intentStr(intent map[string]any, key string) string {
	if v, ok := intent[key].(string); ok {
		return v
	}
	return ""
}

// numStr renders a JSON number (float64) or string identifier as text.
func numStr(v any) string {
	switch n := v.(type) {
	case float64:
		return strconv.FormatInt(int64(n), 10)
	case string:
		return n
	case nil:
		return ""
	default:
		return fmt.Sprint(v)
	}
}

// httpJSON performs a REST call and decodes the JSON body. A >=400 status or
// transport error becomes a loud failure (nonzero exit, per the protocol).
func httpJSON(method, url, token, authScheme string, body any, contentType string) (any, error) {
	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return nil, core.Errf("marshal request body: %v", err)
		}
		reader = bytes.NewReader(encoded)
	}
	req, err := http.NewRequest(method, url, reader)
	if err != nil {
		return nil, core.Errf("%s %s: %v", method, url, err)
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		if contentType == "" {
			contentType = "application/json"
		}
		req.Header.Set("Content-Type", contentType)
	}
	if token != "" {
		if authScheme == "Basic" {
			cred := base64.StdEncoding.EncodeToString([]byte(":" + token))
			req.Header.Set("Authorization", "Basic "+cred)
		} else {
			req.Header.Set("Authorization", "Bearer "+token)
		}
	}
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, core.Errf("%s %s failed: %v", method, url, err)
	}
	defer resp.Body.Close()
	payload, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, core.Errf("%s %s: read response: %v", method, url, err)
	}
	if resp.StatusCode >= 400 {
		detail := string(payload)
		if len(detail) > 500 {
			detail = detail[:500]
		}
		return nil, core.Errf("%s %s -> HTTP %d: %s", method, url, resp.StatusCode, detail)
	}
	if len(payload) == 0 {
		return map[string]any{}, nil
	}
	var out any
	if err := json.Unmarshal(payload, &out); err != nil {
		return nil, core.Errf("%s %s: response is not JSON: %v", method, url, err)
	}
	return out, nil
}

func asObj(v any) map[string]any {
	m, _ := v.(map[string]any)
	return m
}

func asArr(v any) []any {
	a, _ := v.([]any)
	return a
}

// -- GitHub (former Python github handler) -------------------------------------

func githubRepo(intent map[string]any) (string, error) {
	repo := intentStr(intent, "repo")
	if repo == "" {
		repo = os.Getenv("GITHUB_REPOSITORY")
	}
	if repo == "" {
		return "", core.Errf("no target repo: set intent.repo or GITHUB_REPOSITORY")
	}
	return repo, nil
}

func githubHandler(kind string, intent map[string]any) error {
	api := envOr("VENDKIT_GITHUB_API", "https://api.github.com")
	switch kind {
	case "pr":
		return githubPR(api, intent)
	case "handoff":
		return githubHandoff(api, intent)
	case "push-hint":
		return githubPushHint(api, intent)
	default: // fact-verify
		// API verification of branch protection / required checks: post-1.0.
		// "unknown" keeps the conformance status at `attested` (never a fail).
		emitFact("verdict", "unknown")
		return nil
	}
}

func githubPR(api string, intent map[string]any) error {
	token := tokenFor("open_pr")
	if token == "" {
		token = os.Getenv("VENDKIT_PR_TOKEN")
	}
	if token == "" {
		return core.Errf("PR delivery needs VENDKIT_TOKEN_OPEN_PR or " +
			"VENDKIT_PR_TOKEN (a PAT/App token — GITHUB_TOKEN-opened PRs do " +
			"not trigger workflows, so the sync PR would skip its own gate)")
	}
	repo, err := githubRepo(intent)
	if err != nil {
		return err
	}
	head := intentStr(intent, "head_branch")
	owner := strings.SplitN(repo, "/", 2)[0]
	openList, err := httpJSON("GET",
		fmt.Sprintf("%s/repos/%s/pulls?state=open&head=%s:%s", api, repo, owner, head),
		token, "Bearer", nil, "")
	if err != nil {
		return err
	}
	var pr map[string]any
	if existing := asArr(openList); len(existing) > 0 {
		pr = asObj(existing[0])
		if _, err := httpJSON("PATCH",
			fmt.Sprintf("%s/repos/%s/pulls/%s", api, repo, numStr(pr["number"])),
			token, "Bearer",
			map[string]any{"title": intentStr(intent, "title"), "body": intentStr(intent, "body_md")},
			""); err != nil {
			return err
		}
	} else {
		created, err := httpJSON("POST",
			fmt.Sprintf("%s/repos/%s/pulls", api, repo), token, "Bearer",
			map[string]any{
				"title": intentStr(intent, "title"), "body": intentStr(intent, "body_md"),
				"head": head, "base": intentStr(intent, "base_branch"),
			}, "")
		if err != nil {
			return err
		}
		pr = asObj(created)
	}
	emitFact("url", numStr(pr["html_url"]))
	emitFact("number", numStr(pr["number"]))
	return nil
}

func githubHandoff(api string, intent map[string]any) error {
	token := tokenFor("work_items", "GITHUB_TOKEN", "GH_TOKEN")
	if token == "" {
		return core.Errf("no credential for issues (set GITHUB_TOKEN)")
	}
	repo, err := githubRepo(intent)
	if err != nil {
		return err
	}
	key := intentStr(intent, "dedup_key")
	// Idempotency (handler-protocol spec §3): one open item per dedup_key —
	// find by label and comment, else create labelled.
	found, err := httpJSON("GET",
		fmt.Sprintf("%s/repos/%s/issues?state=open&labels=%s", api, repo, key),
		token, "Bearer", nil, "")
	if err != nil {
		return err
	}
	var issue map[string]any
	if existing := asArr(found); len(existing) > 0 {
		issue = asObj(existing[0])
		if _, err := httpJSON("POST",
			fmt.Sprintf("%s/repos/%s/issues/%s/comments", api, repo, numStr(issue["number"])),
			token, "Bearer", map[string]any{"body": intentStr(intent, "body_md")}, ""); err != nil {
			return err
		}
	} else {
		created, err := httpJSON("POST",
			fmt.Sprintf("%s/repos/%s/issues", api, repo), token, "Bearer",
			map[string]any{
				"title": intentStr(intent, "title"), "body": intentStr(intent, "body_md"),
				"labels": []string{key},
			}, "")
		if err != nil {
			return err
		}
		issue = asObj(created)
	}
	emitFact("url", numStr(issue["html_url"]))
	return nil
}

// githubPushHint POSTs a repository_dispatch to a subscriber, nudging its sync
// workflow to run early (platform-integration spec §4, DR-0006). The token
// purpose is the dispatch-scoped push-hint token — distinct from the PR token
// and the one publisher-held-credential relaxation (security-model spec §4).
func githubPushHint(api string, intent map[string]any) error {
	token := tokenFor("push_hint")
	if token == "" {
		return core.Errf("push-hint dispatch needs VENDKIT_TOKEN_PUSH_HINT " +
			"(a dispatch-scoped token; the one publisher-held credential the " +
			"model permits — security-model spec §4). Push is a best-effort " +
			"hint; the consumer's schedule remains the reconciler")
	}
	repo, err := githubRepo(intent)
	if err != nil {
		return err
	}
	eventType := intentStr(intent, "event_type")
	if eventType == "" {
		eventType = "vendkit-release"
	}
	body := map[string]any{"event_type": eventType}
	if payload := asObj(intent["client_payload"]); payload != nil {
		body["client_payload"] = payload
	}
	// A successful dispatch is HTTP 204 with no body; httpJSON treats <400 as
	// success and returns {} for the empty body.
	if _, err := httpJSON("POST",
		fmt.Sprintf("%s/repos/%s/dispatches", api, repo),
		token, "Bearer", body, ""); err != nil {
		return err
	}
	emitFact("dispatched", "true")
	emitFact("event_type", eventType)
	emitFact("repo", repo)
	return nil
}

// -- Azure DevOps (former Python ado handler) ----------------------------------

func adoOrg() (string, error) {
	org := strings.TrimRight(os.Getenv("VENDKIT_ADO_ORG_URL"), "/")
	if org == "" {
		return "", core.Errf("VENDKIT_ADO_ORG_URL is not set " +
			"(e.g. https://dev.azure.com/example-org)")
	}
	return org, nil
}

func adoProjectRepo(intent map[string]any) (string, string, error) {
	repo := intentStr(intent, "repo")
	if repo == "" {
		repo = os.Getenv("SYSTEM_TEAMPROJECT") + "/" + os.Getenv("BUILD_REPOSITORY_NAME")
	}
	project, repository, _ := strings.Cut(repo, "/")
	if project == "" || repository == "" {
		return "", "", core.Errf("target repo must be '<project>/<repository>': %q", repo)
	}
	return project, repository, nil
}

func adoToken(purpose string) (string, error) {
	token := tokenFor(purpose, "SYSTEM_ACCESSTOKEN", "ADO_PAT")
	if token == "" {
		return "", core.Errf("no credential for %s (set VENDKIT_TOKEN_%s, "+
			"SYSTEM_ACCESSTOKEN, or ADO_PAT)", purpose, strings.ToUpper(purpose))
	}
	return token, nil
}

func adoHandler(kind string, intent map[string]any) error {
	switch kind {
	case "pr":
		return adoPR(intent)
	case "handoff":
		return adoHandoff(intent)
	case "push-hint":
		return adoPushHint(intent)
	default: // fact-verify
		emitFact("verdict", "unknown") // API verification: post-1.0
		return nil
	}
}

func adoPR(intent map[string]any) error {
	org, err := adoOrg()
	if err != nil {
		return err
	}
	project, repository, err := adoProjectRepo(intent)
	if err != nil {
		return err
	}
	token, err := adoToken("open_pr")
	if err != nil {
		return err
	}
	head := intentStr(intent, "head_branch")
	base := fmt.Sprintf("%s/%s/_apis/git/repositories/%s", org, project, repository)
	active, err := httpJSON("GET",
		fmt.Sprintf("%s/pullrequests?searchCriteria.status=active"+
			"&searchCriteria.sourceRefName=refs/heads/%s&api-version=7.1", base, head),
		token, "Basic", nil, "")
	if err != nil {
		return err
	}
	var prID string
	if items := asArr(asObj(active)["value"]); len(items) > 0 {
		prID = numStr(asObj(items[0])["pullRequestId"])
		if _, err := httpJSON("PATCH",
			fmt.Sprintf("%s/pullrequests/%s?api-version=7.1", base, prID),
			token, "Basic",
			map[string]any{"title": intentStr(intent, "title"), "description": intentStr(intent, "body_md")},
			""); err != nil {
			return err
		}
	} else {
		created, err := httpJSON("POST",
			fmt.Sprintf("%s/pullrequests?api-version=7.1", base), token, "Basic",
			map[string]any{
				"sourceRefName": "refs/heads/" + head,
				"targetRefName": "refs/heads/" + intentStr(intent, "base_branch"),
				"title":         intentStr(intent, "title"),
				"description":   intentStr(intent, "body_md"),
			}, "")
		if err != nil {
			return err
		}
		prID = numStr(asObj(created)["pullRequestId"])
	}
	emitFact("url", fmt.Sprintf("%s/%s/_git/%s/pullrequest/%s", org, project, repository, prID))
	emitFact("number", prID)
	return nil
}

func adoHandoff(intent map[string]any) error {
	org, err := adoOrg()
	if err != nil {
		return err
	}
	token, err := adoToken("work_items")
	if err != nil {
		return err
	}
	key := intentStr(intent, "dedup_key")
	project := intentStr(intent, "project")
	if project == "" {
		project = os.Getenv("SYSTEM_TEAMPROJECT")
	}
	if project == "" {
		return core.Errf("work items need intent.project or SYSTEM_TEAMPROJECT")
	}
	wiql := map[string]any{"query": "SELECT [System.Id] FROM WorkItems " +
		"WHERE [System.Tags] CONTAINS '" + key + "' " +
		"AND [System.State] <> 'Closed' AND [System.State] <> 'Removed' " +
		"ORDER BY [System.ChangedDate] DESC"}
	found, err := httpJSON("POST",
		fmt.Sprintf("%s/%s/_apis/wit/wiql?api-version=7.1", org, project),
		token, "Basic", wiql, "")
	if err != nil {
		return err
	}
	var wid string
	if items := asArr(asObj(found)["workItems"]); len(items) > 0 {
		wid = numStr(asObj(items[0])["id"])
		if _, err := httpJSON("POST",
			fmt.Sprintf("%s/%s/_apis/wit/workItems/%s/comments?api-version=7.1-preview.4",
				org, project, wid),
			token, "Basic", map[string]any{"text": intentStr(intent, "body_md")}, ""); err != nil {
			return err
		}
	} else {
		itemType := intentStr(intent, "item_type")
		if itemType == "" {
			itemType = "Issue"
		}
		patch := []map[string]any{
			{"op": "add", "path": "/fields/System.Title", "value": intentStr(intent, "title")},
			{"op": "add", "path": "/fields/System.Description", "value": intentStr(intent, "body_md")},
			{"op": "add", "path": "/fields/System.Tags", "value": key},
		}
		created, err := httpJSON("POST",
			fmt.Sprintf("%s/%s/_apis/wit/workitems/$%s?api-version=7.1", org, project, itemType),
			token, "Basic", patch, "application/json-patch+json")
		if err != nil {
			return err
		}
		wid = numStr(asObj(created)["id"])
	}
	emitFact("url", fmt.Sprintf("%s/%s/_workitems/edit/%s", org, project, wid))
	return nil
}

// adoPushHint is a deliberate no-op: on Azure DevOps the push hint is
// consumer-declared (the sync pipeline's own resources.pipelines trigger fires
// on the publisher's release-pipeline completion). The publisher keeps no
// registry and takes no sender action — so this kind only records that fact
// (platform-integration spec §4). Exit 0: a skip is not a failure.
func adoPushHint(_ map[string]any) error {
	emitFact("dispatched", "false")
	emitFact("skipped", "ado-pull-trigger")
	return nil
}
