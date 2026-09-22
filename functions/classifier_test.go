package functions

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

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
