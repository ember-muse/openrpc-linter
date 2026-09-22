package cmd

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestValidateCommand(t *testing.T) {
	for _, tt := range []struct {
		name      string
		document  string
		wantError string
	}{
		{name: "valid v1.3 default file", document: minimumOpenRPCDocument("1.3.2")},
		{name: "valid v1.4 default file", document: minimumOpenRPCDocument("1.4.0")},
		{name: "invalid document", document: `{"zzz": "xxx", "openrpc": "1.4.0"}`, wantError: "Validation failed"},
		{name: "invalid document", document: `{}`, wantError: "document has no openrpc version"},
		{name: "malformed document", document: `{`, wantError: "Error parsing JSON"},
		{name: "missing file", wantError: "Error reading openrpc.json"},
		{name: "unsupported version", document: minimumOpenRPCDocument("1.5.0"), wantError: `unsupported OpenRPC version "1.5.0"`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Chdir(t.TempDir())
			if tt.document != "" {
				err := os.WriteFile("openrpc.json", []byte(tt.document), 0600)
				if err != nil {
					t.Fatal(err)
				}
			}

			output, err := executeValidate(t)
			if tt.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantError) {
					t.Fatalf("expected %q error, got %v", tt.wantError, err)
				}
				if strings.Contains(output.String(), "✅") {
					t.Fatalf("failure reports success: %s", output)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(output.String(), "✅ OpenRPC document is valid!") {
				t.Fatalf("missing success output: %s", output)
			}
		})
	}
}

func executeValidate(t *testing.T, args ...string) (*bytes.Buffer, error) {
	t.Helper()

	command := &cobra.Command{Use: "openrpc-linter"}
	validation := *validateCmd
	command.AddCommand(&validation)
	output := &bytes.Buffer{}
	command.SetOut(output)
	command.SetErr(output)
	command.SetArgs(append([]string{"validate"}, args...))
	return output, command.Execute()
}

func minimumOpenRPCDocument(version string) string {
	return fmt.Sprintf(`{
		"openrpc": %q,
		"info": {"title": "Test API", "version": "1.0.0"},
		"methods": []
	}`, version)
}
