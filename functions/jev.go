package functions

import (
	"context"
	"fmt"
	"net/http"
	"strings"
)

const (
	DefaultJevBaseURL = "https://api.typesafe.ai"
	DefaultJevModel   = "jev-latest"
	JevAPIKeyEnv      = "TYPESAFE_API_KEY"
)

// JevBackend POSTs to {BaseURL}/v1/systemone with bearer auth.
// BaseURL defaults to the hosted API; point it at any Jev-shaped server.
type JevBackend struct {
	APIKey  string
	BaseURL string
	Model   string
	Client  *http.Client
}

func (b *JevBackend) Classify(ctx context.Context, req Request) (Response, error) {
	if b.APIKey == "" {
		return Response{}, fmt.Errorf("classifier: jev backend needs an API key (set %s)", JevAPIKeyEnv)
	}
	base := b.BaseURL
	if base == "" {
		base = DefaultJevBaseURL
	}
	if req.Model == "" {
		req.Model = b.Model
	}
	if req.Model == "" {
		req.Model = DefaultJevModel
	}
	return postSystemOne(ctx, b.Client, strings.TrimSuffix(base, "/")+"/v1/systemone", b.APIKey, req)
}
