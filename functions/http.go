package functions

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// BackendError is a non-2xx response. The raw body is kept because Jev's
// error shapes vary.
type BackendError struct {
	StatusCode int
	Body       string
}

func (e *BackendError) Error() string {
	body := strings.TrimSpace(e.Body)
	if len(body) > 300 {
		body = body[:300] + "…"
	}
	return fmt.Sprintf("classifier: backend returned %d: %s", e.StatusCode, body)
}

func postSystemOne(ctx context.Context, client *http.Client, url, apiKey string, req Request) (Response, error) {
	var out Response
	if len(req.Questions) == 0 {
		return out, fmt.Errorf("classifier: at least one question is required")
	}
	if client == nil {
		client = &http.Client{Timeout: 60 * time.Second}
	}

	body, err := json.Marshal(req)
	if err != nil {
		return out, fmt.Errorf("classifier: marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return out, fmt.Errorf("classifier: build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+apiKey)
	}

	resp, err := client.Do(httpReq)
	if err != nil {
		return out, fmt.Errorf("classifier: request failed: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return out, fmt.Errorf("classifier: read response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return out, &BackendError{StatusCode: resp.StatusCode, Body: string(raw)}
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return out, fmt.Errorf("classifier: decode response: %w", err)
	}
	return out, nil
}
