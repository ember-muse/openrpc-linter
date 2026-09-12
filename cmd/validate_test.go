package cmd

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestValidateCommand(t *testing.T) {
	tests := []struct {
		name           string
		filename       string
		fileContent    string
		expectedOutput string
		expectError    bool
	}{
		{
			name:     "valid openrpc document",
			filename: "test_valid.json",
			fileContent: `{
				"openrpc": "1.2.6",
				"info": {
					"title": "Test API",
					"version": "1.0.0"
				},
				"methods": [
					{
						"name": "test_method",
						"params": []
					}
				]
			}`,
			expectedOutput: "✅ OpenRPC document is valid!",
			expectError:    false,
		},
		{
			name:     "missing required openrpc field",
			filename: "test_invalid.json",
			fileContent: `{
				"info": {
					"title": "Test API",
					"version": "1.0.0"
				},
				"methods": []
			}`,
			expectedOutput: "❌ Validation failed:",
			expectError:    true,
		},
		{
			name:     "missing required info field",
			filename: "test_invalid2.json",
			fileContent: `{
				"openrpc": "1.2.6",
				"methods": []
			}`,
			expectedOutput: "❌ Validation failed:",
			expectError:    true,
		},
		{
			name:     "invalid json format",
			filename: "test_malformed.json",
			fileContent: `{
				"openrpc": "1.2.6",
				"info": {
					"title": "Test API"
					"version": "1.0.0"
				}
			}`,
			expectedOutput: "Error parsing JSON:",
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test file
			err := os.WriteFile(tt.filename, []byte(tt.fileContent), 0644)
			if err != nil {
				t.Fatalf("Failed to create test file: %v", err)
			}
			defer os.Remove(tt.filename) // Clean up

			// Capture stdout since the validate command uses fmt.Printf
			oldStdout := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			// Execute the validate command function directly
			validateCmd.Run(validateCmd, []string{tt.filename})

			// Restore stdout and read captured output
			w.Close()
			os.Stdout = oldStdout

			var buf bytes.Buffer
			_, err = buf.ReadFrom(r)
			if err != nil {
				t.Fatalf("Read captured output: %v", err)
			}
			output := buf.String()

			// Check if expected output is present
			if !strings.Contains(output, tt.expectedOutput) {
				t.Errorf("Expected output to contain '%s', got: '%s'", tt.expectedOutput, output)
			}
		})
	}
}

func TestValidateCommandDefaultFile(t *testing.T) {
	// Test that the command defaults to "openrpc.json" when no argument is provided
	// Capture stdout since the validate command uses fmt.Printf
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Execute without arguments (should use default openrpc.json)
	validateCmd.Run(validateCmd, []string{})

	// Restore stdout and read captured output
	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, err := buf.ReadFrom(r)
	if err != nil {
		t.Fatalf("Read captured output: %v", err)
	}
	output := buf.String()

	// Should show it's validating openrpc.json
	if !strings.Contains(output, "Validating OpenRPC document: openrpc.json") {
		t.Errorf("Expected output to show default file 'openrpc.json', got: '%s'", output)
	}
}

func TestValidateCommandNonExistentFile(t *testing.T) {
	// Capture stdout since the validate command uses fmt.Printf
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Execute with non-existent file
	validateCmd.Run(validateCmd, []string{"nonexistent.json"})

	// Restore stdout and read captured output
	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, err := buf.ReadFrom(r)
	if err != nil {
		t.Fatalf("Read captured output: %v", err)
	}
	output := buf.String()

	// Should show file read error
	if !strings.Contains(output, "Error reading nonexistent.json") {
		t.Errorf("Expected output to show file read error, got: '%s'", output)
	}
}
