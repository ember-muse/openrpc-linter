package cmd

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

type schemaTransport struct {
	body   string
	status int
	err    error
}

func (s schemaTransport) RoundTrip(*http.Request) (*http.Response, error) {
	if s.err != nil {
		return nil, s.err
	}
	return &http.Response{StatusCode: s.status, Status: http.StatusText(s.status), Body: io.NopCloser(strings.NewReader(s.body)), Header: make(http.Header)}, nil
}

func TestValidateCommand(t *testing.T) {
	const schema = `{"type":"object","required":["openrpc"]}`
	for _, tt := range []struct {
		name           string
		document       string
		schema         string
		status         int
		transportError error
		wantError      string
	}{
		{name: "valid default file", document: `{"openrpc":"1.2.6"}`, schema: schema, status: 200},
		{name: "invalid document", document: `{}`, schema: schema, status: 200, wantError: "Validation failed"},
		{name: "malformed document", document: `{`, schema: schema, status: 200, wantError: "Error parsing JSON"},
		{name: "missing file", schema: schema, status: 200, wantError: "Error reading openrpc.json"},
		{name: "malformed schema", schema: `{`, status: 200, wantError: "Error parsing schema JSON"},
		{name: "invalid schema", schema: `{"type":"bogus"}`, status: 200, wantError: "Error compiling schema"},
		{name: "HTTP failure", status: 503, wantError: "cannot load OpenRPC schema"},
		{name: "network failure", transportError: errors.New("offline"), wantError: "offline"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Chdir(t.TempDir())
			if tt.document != "" {
				err := os.WriteFile("openrpc.json", []byte(tt.document), 0600)
				if err != nil {
					t.Fatal(err)
				}
			}
			original := http.DefaultClient
			http.DefaultClient = &http.Client{Transport: schemaTransport{body: tt.schema, status: tt.status, err: tt.transportError}}
			t.Cleanup(func() { http.DefaultClient = original })
			// Execute through Cobra so returning an error reaches the CLI exit handler.
			command := &cobra.Command{Use: "openrpc-linter"}
			validation := *validateCmd
			command.AddCommand(&validation)
			var output bytes.Buffer
			command.SetOut(&output)
			command.SetErr(&output)
			command.SetArgs([]string{"validate"})
			err := command.Execute()
			if tt.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantError) {
					t.Fatalf("expected %q error, got %v", tt.wantError, err)
				}
				if strings.Contains(output.String(), "✅") {
					t.Fatalf("failure reports success: %s", &output)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(output.String(), "✅ OpenRPC document is valid!") {
				t.Fatalf("missing success output: %s", &output)
			}
		})
	}
}
