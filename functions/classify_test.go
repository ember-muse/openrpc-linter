package functions

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/open-rpc/openrpc-linter/selector"
	"github.com/open-rpc/openrpc-linter/types"
)

func classifyRuleContext(opts map[string]interface{}) types.RuleFunctionContext {
	return types.RuleFunctionContext{
		Rule: &types.Rule{
			Then: &types.RuleAction{Function: "classify", FunctionOptions: opts},
		},
		RuleID: "test-classify",
		Path:   "$.methods[0].description",
	}
}

// scriptedJevServer replays one canned Jev response for every request and
// counts hits. Tests exercise the real HTTP path against it — no key, no
// network — and the hit count lets the caching test assert behavior.
func scriptedJevServer(t *testing.T, answers string) (*httptest.Server, *atomic.Int64) {
	t.Helper()
	var calls atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(fmt.Sprintf(
			`{"model":"jev-test","answers":{%s},"usage":{"input_tokens":1,"output_tokens":0}}`,
			answers,
		)))
	}))
	t.Cleanup(srv.Close)
	return srv, &calls
}

func scoreOptions(endpoint string, score float64, below float64) map[string]interface{} {
	return map[string]interface{}{
		"endpoint": endpoint,
		"apiKey":   "test-key",
		"questions": map[string]interface{}{
			"quality": map[string]interface{}{
				"type":         "score",
				"instructions": "Rate the quality of this description",
				"criteria":     []interface{}{"missing", "vague", "adequate", "clear", "excellent"},
			},
		},
		"assert": map[string]interface{}{
			"quality": map[string]interface{}{
				"below":   below,
				"message": "Description reads like filler (score {{.Score}})",
			},
		},
	}
}

func scoreServer(t *testing.T, score float64) (*httptest.Server, *atomic.Int64) {
	t.Helper()
	return scriptedJevServer(t, fmt.Sprintf(`"quality":{"type":"score","score":%v,"confidence":0.9}`, score))
}

func TestClassifyRuleFlagsLowScore(t *testing.T) {
	srv, _ := scoreServer(t, 0.4)
	r := &ClassifyRule{}
	results := r.RunRule("Does the thing", classifyRuleContext(scoreOptions(srv.URL, 0.4, 1.5)))
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d (%v)", len(results), results)
	}
	if !strings.Contains(results[0].Message, "0.4") {
		t.Fatalf("message should render the score, got %q", results[0].Message)
	}
}

func TestClassifyRulePassesHighScore(t *testing.T) {
	srv, _ := scoreServer(t, 3.8)
	r := &ClassifyRule{}
	results := r.RunRule("Returns the user's balance in cents, or an error if the account is closed.", classifyRuleContext(scoreOptions(srv.URL, 3.8, 1.5)))
	if len(results) != 0 {
		t.Fatalf("expected no results, got %v", results)
	}
}

func TestClassifyRuleChoiceAssertion(t *testing.T) {
	srv, _ := scriptedJevServer(t, `"bucket":{"type":"choice","choice":"placeholder","confidence":0.95}`)
	opts := map[string]interface{}{
		"endpoint": srv.URL,
		"apiKey":   "test-key",
		"questions": map[string]interface{}{
			"bucket": map[string]interface{}{
				"type":         "choice",
				"instructions": "What kind of example is this?",
				"criteria":     map[string]interface{}{"realistic": "a plausible real value", "placeholder": "foo/bar/test"},
			},
		},
		"assert": map[string]interface{}{
			"bucket": map[string]interface{}{"in": []interface{}{"placeholder"}},
		},
	}
	r := &ClassifyRule{}
	results := r.RunRule("test", classifyRuleContext(opts))
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %v", results)
	}
	if !strings.Contains(results[0].Message, `choice="placeholder"`) {
		t.Fatalf("default message should name the choice, got %q", results[0].Message)
	}
}

func TestClassifyRuleNoulAssertion(t *testing.T) {
	srv, _ := scriptedJevServer(t, `"sensitive":{"type":"noul","noul":0.97}`)
	opts := map[string]interface{}{
		"endpoint": srv.URL,
		"apiKey":   "test-key",
		"questions": map[string]interface{}{
			"sensitive": map[string]interface{}{
				"type":         "noul",
				"instructions": "Does this method handle sensitive data?",
			},
		},
		"assert": map[string]interface{}{
			"sensitive": map[string]interface{}{
				"above":   0.8,
				"message": "Handles sensitive data (p={{.Noul}}) but has no security docs",
			},
		},
	}
	r := &ClassifyRule{}
	results := r.RunRule("Transfers funds between accounts", classifyRuleContext(opts))
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %v", results)
	}
	if !strings.Contains(results[0].Message, "0.97") {
		t.Fatalf("got %q", results[0].Message)
	}
}

func TestClassifyRuleSkipsEmptyState(t *testing.T) {
	srv, _ := scoreServer(t, 0.1)
	r := &ClassifyRule{}
	for _, v := range []interface{}{"", "   ", nil} {
		if results := r.RunRule(v, classifyRuleContext(scoreOptions(srv.URL, 0.1, 1.5))); len(results) != 0 {
			t.Fatalf("expected no results for empty state %v, got %v", v, results)
		}
	}
}

func TestClassifyRuleStateField(t *testing.T) {
	srv, _ := scoreServer(t, 0.2)
	opts := scoreOptions(srv.URL, 0.2, 1.5)
	opts["stateField"] = "description"
	r := &ClassifyRule{}
	value := map[string]interface{}{"name": "doThing", "description": "Does the thing"}
	results := r.RunRule(value, classifyRuleContext(opts))
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %v", results)
	}
}

func TestClassifyRuleRequiresQuestions(t *testing.T) {
	opts := map[string]interface{}{"endpoint": "http://127.0.0.1:1", "apiKey": "test-key"}
	r := &ClassifyRule{}
	results := r.RunRule("Does the thing", classifyRuleContext(opts))
	if len(results) != 1 || !strings.Contains(results[0].Message, "at least one question") {
		t.Fatalf("expected config error result, got %v", results)
	}
}

func TestClassifyRuleCachesRepeatedStates(t *testing.T) {
	srv, calls := scoreServer(t, 0.4)
	r := &ClassifyRule{}
	ctx := classifyRuleContext(scoreOptions(srv.URL, 0.4, 1.5))

	r.RunRule("Does the thing", ctx)
	r.RunRule("Does the thing", ctx) // identical state: cache hit
	r.RunRule("Does stuff", ctx)     // different state: new call
	if got := calls.Load(); got != 2 {
		t.Fatalf("backend calls = %d, want 2 (one cached)", got)
	}
}

func TestClassifyCacheKeyStable(t *testing.T) {
	q := map[string]Question{"q": {Type: QuestionNoul, Instructions: "Urgent?"}}
	k1, err := classifyCacheKey("m", "same state", q)
	if err != nil {
		t.Fatal(err)
	}
	k2, err := classifyCacheKey("m", "same state", q)
	if err != nil {
		t.Fatal(err)
	}
	k3, err := classifyCacheKey("m", "different state", q)
	if err != nil {
		t.Fatal(err)
	}
	if k1 != k2 || k1 == k3 {
		t.Fatal("cache keys not stable/distinct")
	}
}

func TestClassifyRuleSkipsMissingField(t *testing.T) {
	srv, _ := scoreServer(t, 0.1)
	ctx := classifyRuleContext(scoreOptions(srv.URL, 0.1, 1.5))
	ctx.Target = &selector.Target{Field: "description", Exists: false}
	r := &ClassifyRule{}
	if results := r.RunRule("whatever", ctx); len(results) != 0 {
		t.Fatalf("expected no results for missing field, got %v", results)
	}
}

func TestClassifyRuleRequiresFunctionOptions(t *testing.T) {
	r := &ClassifyRule{}
	ctx := classifyRuleContext(scoreOptions("http://127.0.0.1:1", 0.1, 1.5))
	ctx.Rule.Then = nil
	if results := r.RunRule("x", ctx); len(results) != 1 || !strings.Contains(results[0].Message, "functionOptions") {
		t.Fatalf("expected config error for nil Then, got %v", results)
	}
	ctx = classifyRuleContext(scoreOptions("http://127.0.0.1:1", 0.1, 1.5))
	ctx.Rule.Then = &types.RuleAction{}
	if results := r.RunRule("x", ctx); len(results) != 1 || !strings.Contains(results[0].Message, "functionOptions") {
		t.Fatalf("expected config error for empty options, got %v", results)
	}
}

func TestClassifyRuleBackendErrorFallsBackToContextPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	url := srv.URL
	srv.Close() // refused: surfaces the backend-error path without network
	r := &ClassifyRule{}
	results := r.RunRule("Does the thing", classifyRuleContext(scoreOptions(url, 0.1, 1.5)))
	if len(results) != 1 || !strings.Contains(results[0].Message, "backend error") {
		t.Fatalf("expected backend error result, got %v", results)
	}
	if len(results[0].Path) != 1 || results[0].Path[0] != "$.methods[0].description" {
		t.Fatalf("expected context path fallback, got %v", results[0].Path)
	}

	// With a target set, the result path comes from the target.
	ctx := classifyRuleContext(scoreOptions(url, 0.1, 1.5))
	ctx.Target = &selector.Target{}
	results = (&ClassifyRule{}).RunRule("Does the thing", ctx)
	if len(results) != 1 || len(results[0].Path) != 1 || results[0].Path[0] != "$" {
		t.Fatalf("expected target path, got %v", results)
	}
}

func TestBuildClassifySetupRejectsBadOptions(t *testing.T) {
	cases := map[string]map[string]interface{}{
		"no questions":        {},
		"questions not a map": {"questions": "nope"},
		"unknown type": {"questions": map[string]interface{}{
			"q": map[string]interface{}{"type": "bogus"},
		}},
		"unencodable": {"questions": map[string]interface{}{
			"q": map[string]interface{}{"type": "noul"},
		}, "fn": func() {}},
	}
	for name, opts := range cases {
		if _, err := buildClassifySetup(opts); err == nil {
			t.Errorf("%s: expected error, got nil", name)
		}
	}
}

func TestClassifyRuleAPIKeyFromEnv(t *testing.T) {
	t.Setenv(JevAPIKeyEnv, "env-key")
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"model":"jev-test","answers":{},"usage":{"input_tokens":1,"output_tokens":0}}`))
	}))
	t.Cleanup(srv.Close)
	opts := scoreOptions(srv.URL, 0.4, 1.5)
	delete(opts, "apiKey")
	r := &ClassifyRule{}
	r.RunRule("Does the thing", classifyRuleContext(opts))
	if gotAuth != "Bearer env-key" {
		t.Fatalf("Authorization = %q, want Bearer env-key", gotAuth)
	}
}

func TestClassifyRuleStateFieldMissing(t *testing.T) {
	srv, calls := scoreServer(t, 0.1)
	opts := scoreOptions(srv.URL, 0.1, 1.5)
	opts["stateField"] = "description"
	r := &ClassifyRule{}
	ctx := classifyRuleContext(opts)
	if results := r.RunRule(map[string]interface{}{"name": "doThing"}, ctx); len(results) != 0 {
		t.Fatalf("expected no results when state field is absent, got %v", results)
	}
	if results := r.RunRule("not a map", ctx); len(results) != 0 {
		t.Fatalf("expected no results for non-map value with stateField, got %v", results)
	}
	if got := calls.Load(); got != 0 {
		t.Fatalf("backend calls = %d, want 0", got)
	}
}

func TestClassifyRuleUnmarshalableState(t *testing.T) {
	srv, _ := scoreServer(t, 0.1)
	r := &ClassifyRule{}
	results := r.RunRule(map[string]interface{}{"f": func() {}}, classifyRuleContext(scoreOptions(srv.URL, 0.1, 1.5)))
	if len(results) != 1 || !strings.Contains(results[0].Message, "backend error") {
		t.Fatalf("expected backend error for unmarshalable state, got %v", results)
	}
}

func TestClassifyRuleBadMessageTemplate(t *testing.T) {
	newOpts := func(message string) map[string]interface{} {
		srv, _ := scriptedJevServer(t, `"bucket":{"type":"choice","choice":"placeholder","confidence":0.95}`)
		return map[string]interface{}{
			"endpoint": srv.URL,
			"apiKey":   "test-key",
			"questions": map[string]interface{}{
				"bucket": map[string]interface{}{"type": "choice", "instructions": "Pick one"},
			},
			"assert": map[string]interface{}{
				"bucket": map[string]interface{}{"in": []interface{}{"placeholder"}, "message": message},
			},
		}
	}
	r := &ClassifyRule{}
	results := r.RunRule("test", classifyRuleContext(newOpts("{{.Unclosed")))
	if len(results) != 1 || !strings.Contains(results[0].Message, "{{.Unclosed") {
		t.Fatalf("parse error should return the raw message, got %v", results)
	}
	r = &ClassifyRule{} // setup is cached per instance
	results = r.RunRule("test", classifyRuleContext(newOpts("{{.Choice.nope}}")))
	if len(results) != 1 || !strings.Contains(results[0].Message, "{{.Choice.nope}}") {
		t.Fatalf("execute error should return the raw message, got %v", results)
	}
}

func TestShortAnswer(t *testing.T) {
	s, n := 0.5, 0.9
	cases := []struct {
		ans  Answer
		want string
	}{
		{Answer{Type: QuestionChoice, Choice: "x"}, `choice="x"`},
		{Answer{Type: QuestionScore, Score: &s}, "score=0.5"},
		{Answer{Type: QuestionScore}, "score=0"},
		{Answer{Type: QuestionNoul, Noul: &n}, "noul=0.9"},
		{Answer{Type: "weird"}, "weird"},
	}
	for _, c := range cases {
		if got := shortAnswer(c.ans); got != c.want {
			t.Errorf("shortAnswer(%+v) = %q, want %q", c.ans, got, c.want)
		}
	}
}

func TestClassifyRuleIgnoresAnswerWithoutValue(t *testing.T) {
	srv, _ := scriptedJevServer(t, `"sensitive":{"type":"noul"}`)
	opts := map[string]interface{}{
		"endpoint": srv.URL,
		"apiKey":   "test-key",
		"questions": map[string]interface{}{
			"sensitive": map[string]interface{}{"type": "noul", "instructions": "Sensitive?"},
		},
		"assert": map[string]interface{}{
			"sensitive": map[string]interface{}{"above": 0.8},
		},
	}
	r := &ClassifyRule{}
	if results := r.RunRule("Transfers funds", classifyRuleContext(opts)); len(results) != 0 {
		t.Fatalf("expected no results for valueless answer, got %v", results)
	}
}

// Jev backend and HTTP transport tests.

// recordedRequest is what the test server saw.
type recordedRequest struct {
	path   string
	header http.Header
	body   map[string]any
}

// scriptedJev replays a canned JSON response and records the request.
func scriptedJev(t *testing.T, status int, response string) (*httptest.Server, *recordedRequest) {
	t.Helper()
	rec := &recordedRequest{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec.path = r.URL.Path
		rec.header = r.Header.Clone()
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &rec.body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(response))
	}))
	t.Cleanup(srv.Close)
	return srv, rec
}

func TestJevBackendPostsJevShape(t *testing.T) {
	srv, rec := scriptedJev(t, 200, `{
		"model": "jev-1.13.0",
		"answers": {"q": {"type": "noul", "noul": 0.99}},
		"usage": {"input_tokens": 10, "output_tokens": 0}
	}`)
	b := &JevBackend{APIKey: "sk-test", BaseURL: srv.URL, Client: srv.Client()}
	resp, err := b.Classify(context.Background(), Request{
		State:     "hello",
		Questions: map[string]Question{"q": {Type: QuestionNoul, Instructions: "Urgent?"}},
	})
	if err != nil {
		t.Fatal(err)
	}

	if rec.path != "/v1/systemone" {
		t.Fatalf("path = %q, want /v1/systemone", rec.path)
	}
	if auth := rec.header.Get("Authorization"); auth != "Bearer sk-test" {
		t.Fatalf("Authorization = %q", auth)
	}
	if rec.body["model"] != "jev-latest" || rec.body["state"] != "hello" {
		t.Fatalf("body = %v", rec.body)
	}
	if qs, ok := rec.body["questions"].(map[string]any); !ok || qs["q"] == nil {
		t.Fatalf("questions missing q: %v", rec.body)
	}
	if *resp.Answers["q"].Noul != 0.99 {
		t.Fatalf("noul = %v", *resp.Answers["q"].Noul)
	}
}

func TestJevBackendRequiresAPIKey(t *testing.T) {
	b := &JevBackend{}
	_, err := b.Classify(context.Background(), Request{
		Questions: map[string]Question{"q": {Type: QuestionNoul, Instructions: "x"}},
	})
	if err == nil || !strings.Contains(err.Error(), "API key") {
		t.Fatalf("expected API key error, got %v", err)
	}
}

func TestJevBackendSurfacesHTTPError(t *testing.T) {
	srv, _ := scriptedJev(t, 422, `{"detail": [{"msg": "bad"}]}`)
	b := &JevBackend{APIKey: "sk-test", BaseURL: srv.URL, Client: srv.Client()}
	_, err := b.Classify(context.Background(), Request{
		Questions: map[string]Question{"q": {Type: QuestionNoul, Instructions: "x"}},
	})
	be, ok := err.(*BackendError)
	if !ok || be.StatusCode != 422 {
		t.Fatalf("expected *BackendError{422}, got %T (%v)", err, err)
	}
}

// roundTripFunc adapts a func to http.RoundTripper for transport-level tests.
type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func cannedJevResponse() *http.Response {
	return &http.Response{
		StatusCode: 200,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(`{"model":"x","answers":{},"usage":{"input_tokens":0,"output_tokens":0}}`)),
	}
}

func noulRequest() Request {
	return Request{Questions: map[string]Question{"q": {Type: QuestionNoul, Instructions: "x"}}}
}

func TestJevBackendDefaultsBaseURL(t *testing.T) {
	var gotURL string
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		gotURL = r.URL.String()
		return cannedJevResponse(), nil
	})}
	b := &JevBackend{APIKey: "sk-test", Client: client}
	if _, err := b.Classify(context.Background(), noulRequest()); err != nil {
		t.Fatal(err)
	}
	if gotURL != "https://api.typesafe.ai/v1/systemone" {
		t.Fatalf("url = %q", gotURL)
	}
}

func TestBackendErrorMessage(t *testing.T) {
	e := &BackendError{StatusCode: 500, Body: "boom"}
	if got := e.Error(); !strings.Contains(got, "500") || !strings.Contains(got, "boom") {
		t.Fatalf("got %q", got)
	}
	long := &BackendError{StatusCode: 422, Body: strings.Repeat("x", 500)}
	if got := long.Error(); strings.Contains(got, strings.Repeat("x", 500)) || !strings.HasSuffix(got, "…") {
		t.Fatalf("long body not truncated: %.60q…", got)
	}
}

func TestPostSystemOneRequiresQuestions(t *testing.T) {
	_, err := postSystemOne(context.Background(), nil, "http://example.com", "k", Request{})
	if err == nil || !strings.Contains(err.Error(), "at least one question") {
		t.Fatalf("got %v", err)
	}
}

func TestPostSystemOneRejectsUnmarshalableRequest(t *testing.T) {
	_, err := postSystemOne(context.Background(), nil, "http://example.com", "k",
		Request{State: func() {}, Questions: noulRequest().Questions})
	if err == nil || !strings.Contains(err.Error(), "marshal") {
		t.Fatalf("got %v", err)
	}
}

func TestPostSystemOneSurfacesBadURL(t *testing.T) {
	_, err := postSystemOne(context.Background(), nil, "://bad", "k", noulRequest())
	if err == nil || !strings.Contains(err.Error(), "build request") {
		t.Fatalf("got %v", err)
	}
}

func TestPostSystemOneSurfacesRequestError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	url := srv.URL
	srv.Close() // refused
	_, err := postSystemOne(context.Background(), srv.Client(), url, "k", noulRequest())
	if err == nil || !strings.Contains(err.Error(), "request failed") {
		t.Fatalf("got %v", err)
	}
}

type errReader struct{}

func (errReader) Read([]byte) (int, error) { return 0, io.ErrUnexpectedEOF }

func TestPostSystemOneSurfacesReadError(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(errReader{})}, nil
	})}
	_, err := postSystemOne(context.Background(), client, "http://example.com", "k", noulRequest())
	if err == nil || !strings.Contains(err.Error(), "read response") {
		t.Fatalf("got %v", err)
	}
}

func TestPostSystemOneSurfacesBadJSON(t *testing.T) {
	srv, _ := scriptedJev(t, 200, `not json`)
	_, err := postSystemOne(context.Background(), srv.Client(), srv.URL, "k", noulRequest())
	if err == nil || !strings.Contains(err.Error(), "decode response") {
		t.Fatalf("got %v", err)
	}
}
