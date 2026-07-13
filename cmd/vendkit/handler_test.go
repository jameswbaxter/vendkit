// REST-fixture contract tests for the reference delivery handlers
// (`vendkit handler github|ado`). testing.md §3 "still owed" listed the
// handlers' REST paths as untested; these close that gap without live network.
//
// Each of the four network flows (github pr/handoff, ado pr/handoff) is driven
// against an httptest.Server that ASSERTS method + path + query + request-body
// fields + auth scheme, then replies with a recorded fixture body. The base-URL
// env (VENDKIT_GITHUB_API / VENDKIT_ADO_ORG_URL) is pointed at the test server,
// so no production refactor is needed. Both branches of each flow — "existing
// found → PATCH/comment" and "none → POST create" — are covered, plus the two
// negative paths the docs call out (GITHUB_TOKEN PR-token refusal; a >=400 API
// response surfacing loudly). Facts are asserted by capturing os.Stdout, the
// least-invasive seam (emitFact writes there via fmt.Printf; no prod change).
//
// Tests mutate env via t.Setenv, so none call t.Parallel().

package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

// captureFacts swaps os.Stdout for a pipe, runs fn, and returns everything the
// handler printed via emitFact along with fn's error. This is the least-invasive
// seam for asserting emitted facts: no production code changes.
func captureFacts(t *testing.T, fn func() error) (string, error) {
	t.Helper()
	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stdout = w
	drained := make(chan string, 1)
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		drained <- buf.String()
	}()
	runErr := fn()
	os.Stdout = orig
	_ = w.Close()
	out := <-drained
	_ = r.Close()
	return out, runErr
}

// parseFacts turns emitted `key=value` lines into a map for assertions.
func parseFacts(out string) map[string]string {
	facts := map[string]string{}
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		if k, v, ok := strings.Cut(line, "="); ok {
			facts[k] = v
		}
	}
	return facts
}

func wantFact(t *testing.T, facts map[string]string, key, want string) {
	t.Helper()
	if got := facts[key]; got != want {
		t.Errorf("fact %q = %q, want %q (all facts: %v)", key, got, want, facts)
	}
}

// decodeBody reads and JSON-decodes a request body into v, failing the test on
// error. The server handlers use it to assert on request-body fields.
func decodeBody(t *testing.T, r *http.Request, v any) {
	t.Helper()
	body, err := io.ReadAll(r.Body)
	if err != nil {
		t.Fatalf("read request body: %v", err)
	}
	if err := json.Unmarshal(body, v); err != nil {
		t.Fatalf("request body is not JSON: %v (body=%s)", err, body)
	}
}

func wantBearer(t *testing.T, r *http.Request, token string) {
	t.Helper()
	if got := r.Header.Get("Authorization"); got != "Bearer "+token {
		t.Errorf("Authorization = %q, want Bearer %s", got, token)
	}
}

func wantBasic(t *testing.T, r *http.Request, token string) {
	t.Helper()
	cred := base64.StdEncoding.EncodeToString([]byte(":" + token))
	if got := r.Header.Get("Authorization"); got != "Basic "+cred {
		t.Errorf("Authorization = %q, want Basic (:%s)", got, token)
	}
}

// -- GitHub PR -----------------------------------------------------------------

func TestGithubPR_UpdatesExistingPR(t *testing.T) {
	const token = "pat-open-pr"
	t.Setenv("VENDKIT_TOKEN_OPEN_PR", token)

	var sawGET, sawPATCH bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		wantBearer(t, r, token)
		switch {
		case r.Method == "GET" && r.URL.Path == "/repos/octo/demo/pulls":
			sawGET = true
			q := r.URL.Query()
			if q.Get("state") != "open" {
				t.Errorf("GET query state = %q, want open", q.Get("state"))
			}
			if q.Get("head") != "octo:feature-x" {
				t.Errorf("GET query head = %q, want octo:feature-x", q.Get("head"))
			}
			_, _ = w.Write([]byte(`[{"number":42,"html_url":"https://github.com/octo/demo/pull/42"}]`))
		case r.Method == "PATCH" && r.URL.Path == "/repos/octo/demo/pulls/42":
			sawPATCH = true
			var body map[string]any
			decodeBody(t, r, &body)
			if body["title"] != "sync(docs)" {
				t.Errorf("PATCH title = %v, want sync(docs)", body["title"])
			}
			if body["body"] != "updated body" {
				t.Errorf("PATCH body = %v, want updated body", body["body"])
			}
			_, _ = w.Write([]byte(`{}`))
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			http.Error(w, "unexpected", http.StatusInternalServerError)
		}
	}))
	defer srv.Close()

	intent := map[string]any{
		"repo": "octo/demo", "head_branch": "feature-x", "base_branch": "main",
		"title": "sync(docs)", "body_md": "updated body",
	}
	out, err := captureFacts(t, func() error { return githubPR(srv.URL, intent) })
	if err != nil {
		t.Fatalf("githubPR: %v", err)
	}
	if !sawGET || !sawPATCH {
		t.Fatalf("expected GET+PATCH, got GET=%v PATCH=%v", sawGET, sawPATCH)
	}
	facts := parseFacts(out)
	wantFact(t, facts, "url", "https://github.com/octo/demo/pull/42")
	wantFact(t, facts, "number", "42")
}

func TestGithubPR_CreatesNewPR(t *testing.T) {
	const token = "pat-open-pr"
	t.Setenv("VENDKIT_TOKEN_OPEN_PR", token)

	var sawGET, sawPOST bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		wantBearer(t, r, token)
		switch {
		case r.Method == "GET" && r.URL.Path == "/repos/octo/demo/pulls":
			sawGET = true
			_, _ = w.Write([]byte(`[]`)) // none open → create
		case r.Method == "POST" && r.URL.Path == "/repos/octo/demo/pulls":
			sawPOST = true
			var body map[string]any
			decodeBody(t, r, &body)
			if body["head"] != "feature-x" {
				t.Errorf("POST head = %v, want feature-x", body["head"])
			}
			if body["base"] != "main" {
				t.Errorf("POST base = %v, want main", body["base"])
			}
			if body["title"] != "sync(docs)" {
				t.Errorf("POST title = %v, want sync(docs)", body["title"])
			}
			_, _ = w.Write([]byte(`{"number":7,"html_url":"https://github.com/octo/demo/pull/7"}`))
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			http.Error(w, "unexpected", http.StatusInternalServerError)
		}
	}))
	defer srv.Close()

	intent := map[string]any{
		"repo": "octo/demo", "head_branch": "feature-x", "base_branch": "main",
		"title": "sync(docs)", "body_md": "new body",
	}
	out, err := captureFacts(t, func() error { return githubPR(srv.URL, intent) })
	if err != nil {
		t.Fatalf("githubPR: %v", err)
	}
	if !sawGET || !sawPOST {
		t.Fatalf("expected GET+POST, got GET=%v POST=%v", sawGET, sawPOST)
	}
	facts := parseFacts(out)
	wantFact(t, facts, "url", "https://github.com/octo/demo/pull/7")
	wantFact(t, facts, "number", "7")
}

// TestGithubPR_RefusesGithubTokenFallback covers the differences-ledger #2
// refusal (handler.go ~204): a PR must not be opened with GITHUB_TOKEN, so with
// only GITHUB_TOKEN in scope the handler errors before any network call.
func TestGithubPR_RefusesGithubTokenFallback(t *testing.T) {
	// Force the PR-token slots empty (env may carry them on a dev machine).
	t.Setenv("VENDKIT_TOKEN_OPEN_PR", "")
	t.Setenv("VENDKIT_PR_TOKEN", "")
	t.Setenv("GITHUB_TOKEN", "ghs-workflow-token") // present but must be refused

	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		t.Errorf("handler must not call the API when refusing the token: %s %s", r.Method, r.URL.Path)
	}))
	defer srv.Close()

	intent := map[string]any{"repo": "octo/demo", "head_branch": "feature-x"}
	_, err := captureFacts(t, func() error { return githubPR(srv.URL, intent) })
	if err == nil {
		t.Fatal("expected refusal error, got nil")
	}
	if !strings.Contains(err.Error(), "VENDKIT_TOKEN_OPEN_PR") {
		t.Errorf("refusal error must name VENDKIT_TOKEN_OPEN_PR, got: %v", err)
	}
	if called {
		t.Error("refusal must happen before any network call")
	}
}

// TestGithubPR_APIErrorSurfacesLoudly covers handler.go ~143: a >=400 response
// becomes a nonzero (loud) error rather than a silent misdelivery.
func TestGithubPR_APIErrorSurfacesLoudly(t *testing.T) {
	t.Setenv("VENDKIT_TOKEN_OPEN_PR", "pat-open-pr")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"message":"boom"}`, http.StatusInternalServerError)
	}))
	defer srv.Close()

	intent := map[string]any{"repo": "octo/demo", "head_branch": "feature-x"}
	out, err := captureFacts(t, func() error { return githubPR(srv.URL, intent) })
	if err == nil {
		t.Fatal("expected loud error on HTTP 500, got nil")
	}
	if !strings.Contains(err.Error(), "HTTP 500") {
		t.Errorf("error must surface the status, got: %v", err)
	}
	if strings.Contains(out, "url=") {
		t.Errorf("no facts must be emitted on failure, got: %q", out)
	}
}

// -- GitHub handoff ------------------------------------------------------------

func TestGithubHandoff_CommentsOnExistingIssue(t *testing.T) {
	const token = "gh-token"
	t.Setenv("GITHUB_TOKEN", token) // handoff accepts GITHUB_TOKEN (unlike pr)

	var sawGET, sawComment bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		wantBearer(t, r, token)
		switch {
		case r.Method == "GET" && r.URL.Path == "/repos/octo/demo/issues":
			sawGET = true
			if got := r.URL.Query().Get("labels"); got != "vendkit-watch-docs" {
				t.Errorf("GET labels = %q, want vendkit-watch-docs", got)
			}
			if got := r.URL.Query().Get("state"); got != "open" {
				t.Errorf("GET state = %q, want open", got)
			}
			_, _ = w.Write([]byte(`[{"number":9,"html_url":"https://github.com/octo/demo/issues/9"}]`))
		case r.Method == "POST" && r.URL.Path == "/repos/octo/demo/issues/9/comments":
			sawComment = true
			var body map[string]any
			decodeBody(t, r, &body)
			if body["body"] != "the report" {
				t.Errorf("comment body = %v, want the report", body["body"])
			}
			_, _ = w.Write([]byte(`{}`))
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			http.Error(w, "unexpected", http.StatusInternalServerError)
		}
	}))
	defer srv.Close()

	intent := map[string]any{
		"repo": "octo/demo", "dedup_key": "vendkit-watch-docs", "body_md": "the report",
	}
	out, err := captureFacts(t, func() error { return githubHandoff(srv.URL, intent) })
	if err != nil {
		t.Fatalf("githubHandoff: %v", err)
	}
	if !sawGET || !sawComment {
		t.Fatalf("expected GET+comment, got GET=%v comment=%v", sawGET, sawComment)
	}
	wantFact(t, parseFacts(out), "url", "https://github.com/octo/demo/issues/9")
}

func TestGithubHandoff_CreatesNewIssue(t *testing.T) {
	const token = "gh-token"
	t.Setenv("GITHUB_TOKEN", token)

	var sawGET, sawCreate bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		wantBearer(t, r, token)
		switch {
		case r.Method == "GET" && r.URL.Path == "/repos/octo/demo/issues":
			sawGET = true
			_, _ = w.Write([]byte(`[]`)) // none open → create
		case r.Method == "POST" && r.URL.Path == "/repos/octo/demo/issues":
			sawCreate = true
			var body map[string]any
			decodeBody(t, r, &body)
			if body["title"] != "update available" {
				t.Errorf("create title = %v, want update available", body["title"])
			}
			labels, _ := body["labels"].([]any)
			if len(labels) != 1 || labels[0] != "vendkit-watch-docs" {
				t.Errorf("create labels = %v, want [vendkit-watch-docs]", body["labels"])
			}
			_, _ = w.Write([]byte(`{"number":11,"html_url":"https://github.com/octo/demo/issues/11"}`))
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			http.Error(w, "unexpected", http.StatusInternalServerError)
		}
	}))
	defer srv.Close()

	intent := map[string]any{
		"repo": "octo/demo", "dedup_key": "vendkit-watch-docs",
		"title": "update available", "body_md": "the report",
	}
	out, err := captureFacts(t, func() error { return githubHandoff(srv.URL, intent) })
	if err != nil {
		t.Fatalf("githubHandoff: %v", err)
	}
	if !sawGET || !sawCreate {
		t.Fatalf("expected GET+create, got GET=%v create=%v", sawGET, sawCreate)
	}
	wantFact(t, parseFacts(out), "url", "https://github.com/octo/demo/issues/11")
}

// -- Azure DevOps PR -----------------------------------------------------------

func TestAdoPR_UpdatesExistingPR(t *testing.T) {
	const token = "ado-pat"
	t.Setenv("VENDKIT_TOKEN_OPEN_PR", token)

	var sawGET, sawPATCH bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		wantBasic(t, r, token)
		base := "/proj/_apis/git/repositories/repo"
		switch {
		case r.Method == "GET" && r.URL.Path == base+"/pullrequests":
			sawGET = true
			q := r.URL.Query()
			if q.Get("searchCriteria.status") != "active" {
				t.Errorf("GET status = %q, want active", q.Get("searchCriteria.status"))
			}
			if q.Get("searchCriteria.sourceRefName") != "refs/heads/feature-x" {
				t.Errorf("GET sourceRefName = %q, want refs/heads/feature-x", q.Get("searchCriteria.sourceRefName"))
			}
			if q.Get("api-version") != "7.1" {
				t.Errorf("GET api-version = %q, want 7.1", q.Get("api-version"))
			}
			_, _ = w.Write([]byte(`{"value":[{"pullRequestId":55}]}`))
		case r.Method == "PATCH" && r.URL.Path == base+"/pullrequests/55":
			sawPATCH = true
			var body map[string]any
			decodeBody(t, r, &body)
			if body["title"] != "sync(docs)" {
				t.Errorf("PATCH title = %v, want sync(docs)", body["title"])
			}
			if body["description"] != "updated body" {
				t.Errorf("PATCH description = %v, want updated body", body["description"])
			}
			_, _ = w.Write([]byte(`{}`))
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			http.Error(w, "unexpected", http.StatusInternalServerError)
		}
	}))
	defer srv.Close()

	t.Setenv("VENDKIT_ADO_ORG_URL", srv.URL)
	intent := map[string]any{
		"repo": "proj/repo", "head_branch": "feature-x", "base_branch": "main",
		"title": "sync(docs)", "body_md": "updated body",
	}
	out, err := captureFacts(t, func() error { return adoPR(intent) })
	if err != nil {
		t.Fatalf("adoPR: %v", err)
	}
	if !sawGET || !sawPATCH {
		t.Fatalf("expected GET+PATCH, got GET=%v PATCH=%v", sawGET, sawPATCH)
	}
	facts := parseFacts(out)
	wantFact(t, facts, "url", srv.URL+"/proj/_git/repo/pullrequest/55")
	wantFact(t, facts, "number", "55")
}

func TestAdoPR_CreatesNewPR(t *testing.T) {
	const token = "ado-pat"
	t.Setenv("VENDKIT_TOKEN_OPEN_PR", token)

	var sawGET, sawPOST bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		wantBasic(t, r, token)
		base := "/proj/_apis/git/repositories/repo"
		switch {
		case r.Method == "GET" && r.URL.Path == base+"/pullrequests":
			sawGET = true
			_, _ = w.Write([]byte(`{"value":[]}`)) // none active → create
		case r.Method == "POST" && r.URL.Path == base+"/pullrequests":
			sawPOST = true
			var body map[string]any
			decodeBody(t, r, &body)
			if body["sourceRefName"] != "refs/heads/feature-x" {
				t.Errorf("POST sourceRefName = %v, want refs/heads/feature-x", body["sourceRefName"])
			}
			if body["targetRefName"] != "refs/heads/main" {
				t.Errorf("POST targetRefName = %v, want refs/heads/main", body["targetRefName"])
			}
			if body["title"] != "sync(docs)" {
				t.Errorf("POST title = %v, want sync(docs)", body["title"])
			}
			_, _ = w.Write([]byte(`{"pullRequestId":88}`))
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			http.Error(w, "unexpected", http.StatusInternalServerError)
		}
	}))
	defer srv.Close()

	t.Setenv("VENDKIT_ADO_ORG_URL", srv.URL)
	intent := map[string]any{
		"repo": "proj/repo", "head_branch": "feature-x", "base_branch": "main",
		"title": "sync(docs)", "body_md": "new body",
	}
	out, err := captureFacts(t, func() error { return adoPR(intent) })
	if err != nil {
		t.Fatalf("adoPR: %v", err)
	}
	if !sawGET || !sawPOST {
		t.Fatalf("expected GET+POST, got GET=%v POST=%v", sawGET, sawPOST)
	}
	facts := parseFacts(out)
	wantFact(t, facts, "url", srv.URL+"/proj/_git/repo/pullrequest/88")
	wantFact(t, facts, "number", "88")
}

// -- Azure DevOps handoff ------------------------------------------------------

func TestAdoHandoff_CommentsOnExistingWorkItem(t *testing.T) {
	const token = "ado-pat"
	t.Setenv("VENDKIT_TOKEN_OPEN_PR", "") // ensure work_items purpose falls through
	t.Setenv("SYSTEM_ACCESSTOKEN", token)

	var sawWIQL, sawComment bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		wantBasic(t, r, token)
		switch {
		case r.Method == "POST" && r.URL.Path == "/proj/_apis/wit/wiql":
			sawWIQL = true
			var body map[string]any
			decodeBody(t, r, &body)
			q, _ := body["query"].(string)
			if !strings.Contains(q, "vendkit-watch-docs") {
				t.Errorf("wiql query must reference the dedup key, got: %q", q)
			}
			_, _ = w.Write([]byte(`{"workItems":[{"id":123}]}`))
		case r.Method == "POST" && r.URL.Path == "/proj/_apis/wit/workItems/123/comments":
			sawComment = true
			var body map[string]any
			decodeBody(t, r, &body)
			if body["text"] != "the report" {
				t.Errorf("comment text = %v, want the report", body["text"])
			}
			_, _ = w.Write([]byte(`{}`))
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			http.Error(w, "unexpected", http.StatusInternalServerError)
		}
	}))
	defer srv.Close()

	t.Setenv("VENDKIT_ADO_ORG_URL", srv.URL)
	intent := map[string]any{
		"project": "proj", "dedup_key": "vendkit-watch-docs", "body_md": "the report",
	}
	out, err := captureFacts(t, func() error { return adoHandoff(intent) })
	if err != nil {
		t.Fatalf("adoHandoff: %v", err)
	}
	if !sawWIQL || !sawComment {
		t.Fatalf("expected WIQL+comment, got WIQL=%v comment=%v", sawWIQL, sawComment)
	}
	wantFact(t, parseFacts(out), "url", srv.URL+"/proj/_workitems/edit/123")
}

func TestAdoHandoff_CreatesNewWorkItem(t *testing.T) {
	const token = "ado-pat"
	t.Setenv("VENDKIT_TOKEN_OPEN_PR", "")
	t.Setenv("SYSTEM_ACCESSTOKEN", token)

	var sawWIQL, sawCreate bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		wantBasic(t, r, token)
		switch {
		case r.Method == "POST" && r.URL.Path == "/proj/_apis/wit/wiql":
			sawWIQL = true
			_, _ = w.Write([]byte(`{"workItems":[]}`)) // none → create
		case r.Method == "POST" && r.URL.Path == "/proj/_apis/wit/workitems/$Issue":
			sawCreate = true
			// The json-patch content-type is load-bearing for ADO workitem create.
			if ct := r.Header.Get("Content-Type"); ct != "application/json-patch+json" {
				t.Errorf("create Content-Type = %q, want application/json-patch+json", ct)
			}
			var patch []map[string]any
			decodeBody(t, r, &patch)
			if len(patch) != 3 {
				t.Fatalf("json-patch must have 3 ops, got %d: %v", len(patch), patch)
			}
			byPath := map[string]any{}
			for _, op := range patch {
				byPath[op["path"].(string)] = op["value"]
			}
			if byPath["/fields/System.Title"] != "update available" {
				t.Errorf("patch title = %v, want update available", byPath["/fields/System.Title"])
			}
			if byPath["/fields/System.Tags"] != "vendkit-watch-docs" {
				t.Errorf("patch tags = %v, want vendkit-watch-docs", byPath["/fields/System.Tags"])
			}
			_, _ = w.Write([]byte(`{"id":456}`))
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			http.Error(w, "unexpected", http.StatusInternalServerError)
		}
	}))
	defer srv.Close()

	t.Setenv("VENDKIT_ADO_ORG_URL", srv.URL)
	intent := map[string]any{
		"project": "proj", "dedup_key": "vendkit-watch-docs",
		"title": "update available", "body_md": "the report",
	}
	out, err := captureFacts(t, func() error { return adoHandoff(intent) })
	if err != nil {
		t.Fatalf("adoHandoff: %v", err)
	}
	if !sawWIQL || !sawCreate {
		t.Fatalf("expected WIQL+create, got WIQL=%v create=%v", sawWIQL, sawCreate)
	}
	wantFact(t, parseFacts(out), "url", srv.URL+"/proj/_workitems/edit/456")
}

// -- GitHub push-hint (repository_dispatch) ------------------------------------

func TestGithubPushHint_Dispatches(t *testing.T) {
	const token = "dispatch-token"
	t.Setenv("VENDKIT_TOKEN_PUSH_HINT", token)

	var sawPOST bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		wantBearer(t, r, token)
		switch {
		case r.Method == "POST" && r.URL.Path == "/repos/acme/leaf/dispatches":
			sawPOST = true
			var body map[string]any
			decodeBody(t, r, &body)
			if body["event_type"] != "vendkit-release" {
				t.Errorf("event_type = %v, want vendkit-release", body["event_type"])
			}
			payload, _ := body["client_payload"].(map[string]any)
			if payload == nil {
				t.Fatalf("client_payload missing: %v", body)
			}
			if payload["version"] != "v1.2.3" || payload["tag"] != "v1.2.3" {
				t.Errorf("client_payload version/tag = %v", payload)
			}
			if payload["publisher"] != "acme/framework" {
				t.Errorf("client_payload publisher = %v, want acme/framework", payload["publisher"])
			}
			w.WriteHeader(http.StatusNoContent) // 204: the dispatch success shape
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			http.Error(w, "unexpected", http.StatusInternalServerError)
		}
	}))
	defer srv.Close()

	intent := map[string]any{
		"repo":       "acme/leaf",
		"event_type": "vendkit-release",
		"client_payload": map[string]any{
			"version": "v1.2.3", "tag": "v1.2.3", "publisher": "acme/framework",
		},
	}
	out, err := captureFacts(t, func() error { return githubPushHint(srv.URL, intent) })
	if err != nil {
		t.Fatalf("githubPushHint: %v", err)
	}
	if !sawPOST {
		t.Fatal("expected a POST to /dispatches")
	}
	facts := parseFacts(out)
	wantFact(t, facts, "dispatched", "true")
	wantFact(t, facts, "event_type", "vendkit-release")
	wantFact(t, facts, "repo", "acme/leaf")
}

// TestGithubPushHint_RefusesWithoutToken: the dispatch-scoped token is
// mandatory; with none in scope the handler errors before any network call.
func TestGithubPushHint_RefusesWithoutToken(t *testing.T) {
	t.Setenv("VENDKIT_TOKEN_PUSH_HINT", "")

	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		t.Errorf("handler must not call the API when refusing: %s %s", r.Method, r.URL.Path)
	}))
	defer srv.Close()

	intent := map[string]any{"repo": "acme/leaf"}
	_, err := captureFacts(t, func() error { return githubPushHint(srv.URL, intent) })
	if err == nil {
		t.Fatal("expected refusal error, got nil")
	}
	if !strings.Contains(err.Error(), "VENDKIT_TOKEN_PUSH_HINT") {
		t.Errorf("refusal must name VENDKIT_TOKEN_PUSH_HINT, got: %v", err)
	}
	if called {
		t.Error("refusal must happen before any network call")
	}
}

// TestGithubPushHint_APIErrorSurfacesLoudly: a >=400 dispatch response is a
// loud (nonzero) failure, not a silent no-op.
func TestGithubPushHint_APIErrorSurfacesLoudly(t *testing.T) {
	t.Setenv("VENDKIT_TOKEN_PUSH_HINT", "dispatch-token")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"message":"no such repo"}`, http.StatusNotFound)
	}))
	defer srv.Close()

	intent := map[string]any{"repo": "acme/leaf", "event_type": "vendkit-release"}
	out, err := captureFacts(t, func() error { return githubPushHint(srv.URL, intent) })
	if err == nil {
		t.Fatal("expected loud error on HTTP 404, got nil")
	}
	if !strings.Contains(err.Error(), "HTTP 404") {
		t.Errorf("error must surface the status, got: %v", err)
	}
	if strings.Contains(out, "dispatched=true") {
		t.Errorf("no success fact must be emitted on failure, got: %q", out)
	}
}

// TestAdoPushHint_IsPullTriggerNoop: on ADO the push hint is the consumer's own
// resources.pipelines trigger; the publisher does nothing and says so.
func TestAdoPushHint_IsPullTriggerNoop(t *testing.T) {
	out, err := captureFacts(t, func() error { return adoPushHint(map[string]any{"repo": "proj/repo"}) })
	if err != nil {
		t.Fatalf("adoPushHint: %v", err)
	}
	facts := parseFacts(out)
	wantFact(t, facts, "dispatched", "false")
	wantFact(t, facts, "skipped", "ado-pull-trigger")
}

// -- GitHub fact-verify (branch-protection API) --------------------------------
//
// These assert the --verify-attestations verification path: the handler maps a
// STABLE fact key to a concrete branch-protection API check and emits
// verdict=true|false|unknown. Method + path + Bearer auth are asserted, then a
// recorded protection fixture drives the verdict.

func TestGithubFactVerify_RequiredCheckEnforced(t *testing.T) {
	t.Setenv("VENDKIT_TOKEN_FACT_VERIFY", "verify-tok")

	var sawGET bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		wantBearer(t, r, "verify-tok")
		switch {
		case r.Method == "GET" && r.URL.Path == "/repos/octo/demo/branches/main/protection":
			sawGET = true
			_, _ = w.Write([]byte(`{"required_status_checks":{"strict":true,"contexts":["vendkit-gate"]}}`))
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			http.Error(w, "unexpected", http.StatusInternalServerError)
		}
	}))
	defer srv.Close()

	intent := map[string]any{
		"fact": "required_check_enforced", "repo": "octo/demo", "branch": "main",
	}
	out, err := captureFacts(t, func() error { return githubFactVerify(srv.URL, intent) })
	if err != nil {
		t.Fatalf("githubFactVerify: %v", err)
	}
	if !sawGET {
		t.Fatal("expected a GET to the branch-protection endpoint")
	}
	wantFact(t, parseFacts(out), "verdict", "true")
}

// A named check must be a member of the required contexts for a true verdict.
func TestGithubFactVerify_NamedCheckMembership(t *testing.T) {
	t.Setenv("VENDKIT_TOKEN_FACT_VERIFY", "verify-tok")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"required_status_checks":{"checks":[{"context":"other"}]}}`))
	}))
	defer srv.Close()

	intent := map[string]any{
		"fact": "required_check_enforced", "repo": "octo/demo", "branch": "main",
		"check": "vendkit-gate",
	}
	out, err := captureFacts(t, func() error { return githubFactVerify(srv.URL, intent) })
	if err != nil {
		t.Fatalf("githubFactVerify: %v", err)
	}
	wantFact(t, parseFacts(out), "verdict", "false") // required check present, but not the named one
}

// Protection exists but requires no status checks → the control is not enforced.
func TestGithubFactVerify_NotEnforced(t *testing.T) {
	t.Setenv("VENDKIT_TOKEN_FACT_VERIFY", "verify-tok")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"required_pull_request_reviews":{"require_code_owner_reviews":true}}`))
	}))
	defer srv.Close()

	intent := map[string]any{"fact": "required_check_enforced", "repo": "octo/demo", "branch": "main"}
	out, err := captureFacts(t, func() error { return githubFactVerify(srv.URL, intent) })
	if err != nil {
		t.Fatalf("githubFactVerify: %v", err)
	}
	wantFact(t, parseFacts(out), "verdict", "false")
}

// A 404 from the protection endpoint means the branch is unprotected → false.
func TestGithubFactVerify_Unprotected404(t *testing.T) {
	t.Setenv("VENDKIT_TOKEN_FACT_VERIFY", "verify-tok")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"message":"Branch not protected"}`, http.StatusNotFound)
	}))
	defer srv.Close()

	intent := map[string]any{"fact": "required_check_enforced", "repo": "octo/demo", "branch": "main"}
	out, err := captureFacts(t, func() error { return githubFactVerify(srv.URL, intent) })
	if err != nil {
		t.Fatalf("githubFactVerify: %v", err)
	}
	wantFact(t, parseFacts(out), "verdict", "false")
}

// A 403 (insufficient scope) is unknown — NOT false — so the rule stays attested.
func TestGithubFactVerify_ForbiddenIsUnknown(t *testing.T) {
	t.Setenv("VENDKIT_TOKEN_FACT_VERIFY", "verify-tok")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"message":"Resource not accessible"}`, http.StatusForbidden)
	}))
	defer srv.Close()

	intent := map[string]any{"fact": "required_check_enforced", "repo": "octo/demo", "branch": "main"}
	out, err := captureFacts(t, func() error { return githubFactVerify(srv.URL, intent) })
	if err != nil {
		t.Fatalf("githubFactVerify: %v", err)
	}
	wantFact(t, parseFacts(out), "verdict", "unknown")
}

// With no --base-branch, the handler resolves the repo's default branch first.
func TestGithubFactVerify_ResolvesDefaultBranch(t *testing.T) {
	t.Setenv("VENDKIT_TOKEN_FACT_VERIFY", "verify-tok")
	var sawRepoGET, sawProtGET bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "GET" && r.URL.Path == "/repos/octo/demo":
			sawRepoGET = true
			_, _ = w.Write([]byte(`{"default_branch":"trunk"}`))
		case r.Method == "GET" && r.URL.Path == "/repos/octo/demo/branches/trunk/protection":
			sawProtGET = true
			_, _ = w.Write([]byte(`{"required_status_checks":{"contexts":["vendkit-gate"]}}`))
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			http.Error(w, "unexpected", http.StatusInternalServerError)
		}
	}))
	defer srv.Close()

	intent := map[string]any{"fact": "required_check_enforced", "repo": "octo/demo"}
	out, err := captureFacts(t, func() error { return githubFactVerify(srv.URL, intent) })
	if err != nil {
		t.Fatalf("githubFactVerify: %v", err)
	}
	if !sawRepoGET || !sawProtGET {
		t.Fatalf("expected repo + protection GETs, got repo=%v prot=%v", sawRepoGET, sawProtGET)
	}
	wantFact(t, parseFacts(out), "verdict", "true")
}

// An unrecognised fact key is unknown and makes NO network call (forward-compat).
func TestGithubFactVerify_UnknownFactKey(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("no network call expected for an unrecognised fact: %s %s", r.Method, r.URL.Path)
	}))
	defer srv.Close()

	intent := map[string]any{"fact": "some_future_fact", "repo": "octo/demo", "branch": "main"}
	out, err := captureFacts(t, func() error { return githubFactVerify(srv.URL, intent) })
	if err != nil {
		t.Fatalf("githubFactVerify: %v", err)
	}
	wantFact(t, parseFacts(out), "verdict", "unknown")
}

// -- Azure DevOps fact-verify (branch-policy API) ------------------------------

func TestAdoFactVerify_RequiredReviewersPresent(t *testing.T) {
	const token = "ado-verify"
	t.Setenv("VENDKIT_TOKEN_FACT_VERIFY", token)

	var sawGET bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		wantBasic(t, r, token)
		switch {
		case r.Method == "GET" && r.URL.Path == "/proj/_apis/policy/configurations":
			sawGET = true
			if got := r.URL.Query().Get("api-version"); got != "7.1" {
				t.Errorf("api-version = %q, want 7.1", got)
			}
			_, _ = w.Write([]byte(`{"value":[{"isEnabled":true,"isBlocking":true,` +
				`"type":{"displayName":"Minimum number of reviewers"},` +
				`"settings":{"scope":[{"refName":"refs/heads/main","matchKind":"Exact"}]}}]}`))
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			http.Error(w, "unexpected", http.StatusInternalServerError)
		}
	}))
	defer srv.Close()

	t.Setenv("VENDKIT_ADO_ORG_URL", srv.URL)
	intent := map[string]any{"fact": "required_reviewers_policy", "repo": "proj/repo", "branch": "main"}
	out, err := captureFacts(t, func() error { return adoFactVerify(intent) })
	if err != nil {
		t.Fatalf("adoFactVerify: %v", err)
	}
	if !sawGET {
		t.Fatal("expected a GET to the policy-configurations endpoint")
	}
	wantFact(t, parseFacts(out), "verdict", "true")
}

// No matching policy in the list → the control is definitively absent → false.
func TestAdoFactVerify_PolicyAbsent(t *testing.T) {
	t.Setenv("VENDKIT_TOKEN_FACT_VERIFY", "ado-verify")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"value":[]}`))
	}))
	defer srv.Close()

	t.Setenv("VENDKIT_ADO_ORG_URL", srv.URL)
	intent := map[string]any{"fact": "required_reviewers_policy", "repo": "proj/repo", "branch": "main"}
	out, err := captureFacts(t, func() error { return adoFactVerify(intent) })
	if err != nil {
		t.Fatalf("adoFactVerify: %v", err)
	}
	wantFact(t, parseFacts(out), "verdict", "false")
}

// required_check_enforced demands a *blocking* Build-validation policy: a
// non-blocking build policy is not enough → false.
func TestAdoFactVerify_RequiredCheckNeedsBlocking(t *testing.T) {
	t.Setenv("VENDKIT_TOKEN_FACT_VERIFY", "ado-verify")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"value":[{"isEnabled":true,"isBlocking":false,` +
			`"type":{"displayName":"Build"},"settings":{"scope":[]}}]}`))
	}))
	defer srv.Close()

	t.Setenv("VENDKIT_ADO_ORG_URL", srv.URL)
	intent := map[string]any{"fact": "required_check_enforced", "repo": "proj/repo"}
	out, err := captureFacts(t, func() error { return adoFactVerify(intent) })
	if err != nil {
		t.Fatalf("adoFactVerify: %v", err)
	}
	wantFact(t, parseFacts(out), "verdict", "false")
}

// A build-validation policy that IS blocking satisfies required_check_enforced.
func TestAdoFactVerify_BuildValidationBlocking(t *testing.T) {
	t.Setenv("VENDKIT_TOKEN_FACT_VERIFY", "ado-verify")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"value":[{"isEnabled":true,"isBlocking":true,` +
			`"type":{"displayName":"Build"},"settings":{"scope":[]}}]}`))
	}))
	defer srv.Close()

	t.Setenv("VENDKIT_ADO_ORG_URL", srv.URL)
	intent := map[string]any{"fact": "required_check_enforced", "repo": "proj/repo"}
	out, err := captureFacts(t, func() error { return adoFactVerify(intent) })
	if err != nil {
		t.Fatalf("adoFactVerify: %v", err)
	}
	wantFact(t, parseFacts(out), "verdict", "true")
}

// A 403 from the policy endpoint is unknown (scope), not false.
func TestAdoFactVerify_ForbiddenIsUnknown(t *testing.T) {
	t.Setenv("VENDKIT_TOKEN_FACT_VERIFY", "ado-verify")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"message":"TF401027"}`, http.StatusForbidden)
	}))
	defer srv.Close()

	t.Setenv("VENDKIT_ADO_ORG_URL", srv.URL)
	intent := map[string]any{"fact": "pull_request_enforcement", "repo": "proj/repo"}
	out, err := captureFacts(t, func() error { return adoFactVerify(intent) })
	if err != nil {
		t.Fatalf("adoFactVerify: %v", err)
	}
	wantFact(t, parseFacts(out), "verdict", "unknown")
}

// Missing org/token coordinate → unknown, with no network call.
func TestAdoFactVerify_MissingCoordinateIsUnknown(t *testing.T) {
	t.Setenv("VENDKIT_ADO_ORG_URL", "")
	t.Setenv("VENDKIT_TOKEN_FACT_VERIFY", "ado-verify")
	intent := map[string]any{"fact": "required_reviewers_policy", "repo": "proj/repo"}
	out, err := captureFacts(t, func() error { return adoFactVerify(intent) })
	if err != nil {
		t.Fatalf("adoFactVerify: %v", err)
	}
	wantFact(t, parseFacts(out), "verdict", "unknown")
}

// An unrecognised fact key is unknown for ADO too.
func TestAdoFactVerify_UnknownFactKey(t *testing.T) {
	t.Setenv("VENDKIT_ADO_ORG_URL", "https://example.invalid")
	intent := map[string]any{"fact": "some_future_fact", "repo": "proj/repo"}
	out, err := captureFacts(t, func() error { return adoFactVerify(intent) })
	if err != nil {
		t.Fatalf("adoFactVerify: %v", err)
	}
	wantFact(t, parseFacts(out), "verdict", "unknown")
}
