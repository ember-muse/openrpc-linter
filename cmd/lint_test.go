package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestRunLint(t *testing.T) {
	// Create a temporary OpenRPC file without description
	openrpcContent := map[string]interface{}{
		"info": map[string]interface{}{
			"title":   "Test API",
			"version": "1.0.0",
			// no description field
		},
	}

	openrpcData, err := json.Marshal(openrpcContent)
	if err != nil {
		t.Fatalf("Failed to create test OpenRPC content: %v", err)
	}

	tempOpenRPC, err := os.CreateTemp("", "test-openrpc-*.json")
	if err != nil {
		t.Fatalf("Failed to create temp OpenRPC file: %v", err)
	}
	defer os.Remove(tempOpenRPC.Name())

	if _, err := tempOpenRPC.Write(openrpcData); err != nil {
		t.Fatalf("Failed to write test OpenRPC file: %v", err)
	}
	tempOpenRPC.Close()

	// Create a temporary rules file
	rulesContent := `description: "Test rules"
rules:
  info-description:
    description: "Info must have description"
    given: "$.info.description"
    then:
      function: "truthy"
`

	tempRules, err := os.CreateTemp("", "test-rules-*.yml")
	if err != nil {
		t.Fatalf("Failed to create temp rules file: %v", err)
	}
	defer os.Remove(tempRules.Name())

	if _, err := tempRules.WriteString(rulesContent); err != nil {
		t.Fatalf("Failed to write test rules file: %v", err)
	}
	tempRules.Close()

	// Test the RunLint function directly
	var output bytes.Buffer
	opts := LintOptions{
		OpenRPCFile: tempOpenRPC.Name(),
		RulesFile:   tempRules.Name(),
		Output:      &output,
	}

	err = RunLint(opts)
	if err == nil {
		t.Fatalf("Expected RunLint to return error for linting violations, but got nil")
	}

	// Should be a linting error, not a technical error
	if !strings.Contains(err.Error(), "linting error(s)") {
		t.Fatalf("Expected linting error, but got: %v", err)
	}

	outputStr := output.String()
	t.Logf("Lint output:\n%s", outputStr)

	if !strings.Contains(outputStr, "error") {
		t.Errorf("Expected severity label 'error' in output, but got: %s", outputStr)
	}

	if !strings.Contains(outputStr, "info.description") {
		t.Errorf("Expected friendly path info.description in output, but got: %s", outputStr)
	}
	if !strings.Contains(outputStr, "missing field 'description'") {
		t.Errorf("Expected missing field message in output, but got: %s", outputStr)
	}

	if !strings.Contains(outputStr, "1 error found") {
		t.Errorf("Expected error summary in output, but got: %s", outputStr)
	}
}

func TestRunLintSuccess(t *testing.T) {
	// Create a temporary OpenRPC file WITH description
	openrpcContent := map[string]interface{}{
		"info": map[string]interface{}{
			"title":       "Test API",
			"version":     "1.0.0",
			"description": "A test API description",
		},
	}

	openrpcData, err := json.Marshal(openrpcContent)
	if err != nil {
		t.Fatalf("Failed to create test OpenRPC content: %v", err)
	}

	tempOpenRPC, err := os.CreateTemp("", "test-openrpc-*.json")
	if err != nil {
		t.Fatalf("Failed to create temp OpenRPC file: %v", err)
	}
	defer os.Remove(tempOpenRPC.Name())

	if _, err := tempOpenRPC.Write(openrpcData); err != nil {
		t.Fatalf("Failed to write test OpenRPC file: %v", err)
	}
	tempOpenRPC.Close()

	// Create a temporary rules file
	rulesContent := `description: "Test rules"
rules:
  info-description:
    description: "Info must have description"
    given: "$.info.description"
    then:
      function: "truthy"
`

	tempRules, err := os.CreateTemp("", "test-rules-*.yml")
	if err != nil {
		t.Fatalf("Failed to create temp rules file: %v", err)
	}
	defer os.Remove(tempRules.Name())

	if _, err := tempRules.WriteString(rulesContent); err != nil {
		t.Fatalf("Failed to write test rules file: %v", err)
	}
	tempRules.Close()

	// Test the RunLint function directly
	var output bytes.Buffer
	opts := LintOptions{
		OpenRPCFile: tempOpenRPC.Name(),
		RulesFile:   tempRules.Name(),
		Output:      &output,
	}

	err = RunLint(opts)
	if err != nil {
		t.Fatalf("RunLint should succeed when no linting violations, but got: %v", err)
	}

	outputStr := output.String()
	t.Logf("Lint output:\n%s", outputStr)

	if !strings.Contains(outputStr, "All 1 rules passed") {
		t.Errorf("Expected 'All 1 rules passed' in output, but got: %s", outputStr)
	}
}

func TestRunLintWarningSeverityDoesNotFail(t *testing.T) {
	// Create a temporary OpenRPC file without description to trigger a truthy violation.
	openrpcContent := map[string]interface{}{
		"info": map[string]interface{}{
			"title":   "Test API",
			"version": "1.0.0",
		},
	}

	openrpcData, err := json.Marshal(openrpcContent)
	if err != nil {
		t.Fatalf("Failed to create test OpenRPC content: %v", err)
	}

	tempOpenRPC, err := os.CreateTemp("", "test-openrpc-*.json")
	if err != nil {
		t.Fatalf("Failed to create temp OpenRPC file: %v", err)
	}
	defer os.Remove(tempOpenRPC.Name())

	if _, err := tempOpenRPC.Write(openrpcData); err != nil {
		t.Fatalf("Failed to write test OpenRPC file: %v", err)
	}
	tempOpenRPC.Close()

	// severity: warn should not cause RunLint to return an error.
	rulesContent := `description: "Test rules"
rules:
  info-description:
    description: "Info must have description"
    given: "$.info.description"
    severity: "warn"
    then:
      function: "truthy"
`

	tempRules, err := os.CreateTemp("", "test-rules-*.yml")
	if err != nil {
		t.Fatalf("Failed to create temp rules file: %v", err)
	}
	defer os.Remove(tempRules.Name())

	if _, err := tempRules.WriteString(rulesContent); err != nil {
		t.Fatalf("Failed to write test rules file: %v", err)
	}
	tempRules.Close()

	var output bytes.Buffer
	opts := LintOptions{
		OpenRPCFile: tempOpenRPC.Name(),
		RulesFile:   tempRules.Name(),
		Output:      &output,
	}

	err = RunLint(opts)
	if err != nil {
		t.Fatalf("RunLint should not fail for warn severity violations, but got: %v\nOutput:\n%s", err, output.String())
	}

	outputStr := output.String()
	if !strings.Contains(outputStr, "info.description") {
		t.Fatalf("Expected warning with friendly path, got:\n%s", outputStr)
	}
	if !strings.Contains(outputStr, "missing field 'description'") {
		t.Fatalf("Expected warning violation message, got:\n%s", outputStr)
	}
	if !strings.Contains(outputStr, "warning") {
		t.Fatalf("Expected 'warning' label in output, got:\n%s", outputStr)
	}
	if !strings.Contains(outputStr, "1 warning found") {
		t.Fatalf("Expected warning summary in output, got:\n%s", outputStr)
	}
}

func TestRunLintSeverityIgnoreDisablesRecommendedRule(t *testing.T) {
	openrpcContent := map[string]interface{}{
		"info": map[string]interface{}{
			"title":       "Test API",
			"version":     "1.0.0",
			"description": "A test API description",
		},
		"methods": []interface{}{
			map[string]interface{}{
				"name": "test_method",
				"errors": []interface{}{
					map[string]interface{}{
						"code":    -32000,
						"message": "Test error",
					},
				},
				"examples": []interface{}{
					map[string]interface{}{
						"name":   "test example",
						"params": []interface{}{},
						"result": map[string]interface{}{},
					},
				},
			},
		},
	}

	openrpcData, err := json.Marshal(openrpcContent)
	if err != nil {
		t.Fatalf("Failed to create test OpenRPC content: %v", err)
	}

	tempOpenRPC, err := os.CreateTemp("", "test-openrpc-*.json")
	if err != nil {
		t.Fatalf("Failed to create temp OpenRPC file: %v", err)
	}
	defer os.Remove(tempOpenRPC.Name())

	if _, err := tempOpenRPC.Write(openrpcData); err != nil {
		t.Fatalf("Failed to write test OpenRPC file: %v", err)
	}
	tempOpenRPC.Close()

	rulesContent := `description: "Test rules"
extends:
  - recommended
rules:
  info-license:
    severity: "ignore"
  method-summary:
    severity: "ignore"
  method-description:
    severity: "ignore"
  example-description:
    severity: "ignore"
`

	tempRules, err := os.CreateTemp("", "test-rules-*.yml")
	if err != nil {
		t.Fatalf("Failed to create temp rules file: %v", err)
	}
	defer os.Remove(tempRules.Name())

	if _, err := tempRules.WriteString(rulesContent); err != nil {
		t.Fatalf("Failed to write test rules file: %v", err)
	}
	tempRules.Close()

	var output bytes.Buffer
	opts := LintOptions{
		OpenRPCFile: tempOpenRPC.Name(),
		RulesFile:   tempRules.Name(),
		Output:      &output,
	}

	err = RunLint(opts)
	if err != nil {
		t.Fatalf("RunLint should not fail when a recommended rule is disabled, but got: %v\nOutput:\n%s", err, output.String())
	}

	outputStr := output.String()
	if strings.Contains(outputStr, "method-description") {
		t.Fatalf("Expected disabled recommended rule to be omitted from output, got:\n%s", outputStr)
	}
	if strings.Contains(outputStr, "Missing field 'description'") {
		t.Fatalf("Expected method description violation to be skipped, got:\n%s", outputStr)
	}
}

func TestRunLintHandlesCyclicSchemaRef(t *testing.T) {
	openrpcContent := `{
  "openrpc": "1.4.0",
  "info": {
    "title": "Recursive Schema API",
    "version": "1.0.0"
  },
  "methods": [
    {
      "name": "getCategory",
      "params": [],
      "result": {
        "name": "category",
        "schema": {
          "$ref": "#/components/schemas/Category"
        }
      }
    }
  ],
  "components": {
    "schemas": {
      "Category": {
        "type": "object",
        "properties": {
          "name": {
            "type": "string"
          },
          "parent": {
            "$ref": "#/components/schemas/Category"
          }
        }
      }
    }
  }
}`

	rulesContent := `description: "Cyclic ref smoke test"
rules:
  info-title:
    description: "Info title must exist"
    given: "$.info.title"
    severity: "error"
    then:
      function: "truthy"
`

	results, err := runLintJSON(t, openrpcContent, rulesContent)
	if err != nil {
		t.Fatalf("RunLint should handle cyclic schema refs, got: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("expected no lint results, got %+v", results)
	}
}

// TestRunLintDescendantDescriptionReportsMissingCandidates exercises the
// schema-aware descendant path that the old truthy implementation could not
// satisfy: $..description must surface MISSING descriptions on every
// OpenRPC object the meta-schema says could legally hold one, not just the
// ones that already exist. The test relies on the full lint pipeline
// (resolveRefs -> selector.Build -> ExecuteRule) so a regression in any
// stage is caught.
func TestRunLintDescendantDescriptionReportsMissingCandidates(t *testing.T) {
	openrpcContent := `{
        "openrpc": "1.4.0",
        "info": {"title": "Demo", "version": "1.0.0"},
        "methods": [
          {"name": "foo", "params": [{"name": "p"}]}
        ]
      }`

	rulesContent := `description: "Descendant description rule"
rules:
  descendant-description:
    description: "Every OpenRPC object should have description"
    given: "$..description"
    severity: "error"
    then:
      function: "truthy"
`

	results, err := runLintJSON(t, openrpcContent, rulesContent)
	if err == nil {
		t.Fatalf("expected linting errors for missing descriptions")
	}

	// Three candidate parents per the v1.4 meta-schema: info, the method,
	// and the content descriptor. None of them have description.
	wantPaths := map[string]bool{
		"$['info']['description']":                    false,
		"$['methods'][0]['description']":              false,
		"$['methods'][0]['params'][0]['description']": false,
	}
	for _, r := range results {
		if r.RuleID != "descendant-description" {
			continue
		}
		for _, p := range r.Path {
			if _, ok := wantPaths[p]; ok {
				wantPaths[p] = true
			}
		}
	}
	for path, seen := range wantPaths {
		if !seen {
			t.Errorf("expected descendant-description to report %s; results: %+v", path, results)
		}
	}
}

// TestRunLintCompoundDescendantPath exercises a path with a descendant in
// the middle ($.methods..result.schema). The selector must peel the
// descendant ..result, then evaluate .schema as a parent-field check on
// each resolved result node — and report missing schema fields.
func TestRunLintCompoundDescendantPath(t *testing.T) {
	openrpcContent := `{
        "openrpc": "1.4.0",
        "info": {"title": "Demo", "version": "1.0.0"},
        "methods": [
          {"name": "foo", "params": [], "result": {"name": "r"}}
        ]
      }`

	rulesContent := `description: "Result schemas must exist"
rules:
  result-schema:
    description: "Each result must have a schema"
    given: "$.methods..result.schema"
    severity: "error"
    then:
      function: "truthy"
`

	results, err := runLintJSON(t, openrpcContent, rulesContent)
	if err == nil {
		t.Fatalf("expected linting errors for missing result.schema")
	}

	found := false
	for _, r := range results {
		if r.RuleID != "result-schema" {
			continue
		}
		for _, p := range r.Path {
			if p == "$['methods'][0]['result']['schema']" {
				found = true
			}
		}
	}
	if !found {
		t.Fatalf("expected missing-schema diagnostic at $['methods'][0]['result']['schema'], got: %+v", results)
	}
}

func TestRunLintInvalidSeverityFailsFast(t *testing.T) {
	openrpcContent := map[string]interface{}{
		"info": map[string]interface{}{
			"title":   "Test API",
			"version": "1.0.0",
		},
	}

	openrpcData, err := json.Marshal(openrpcContent)
	if err != nil {
		t.Fatalf("Failed to create test OpenRPC content: %v", err)
	}

	tempOpenRPC, err := os.CreateTemp("", "test-openrpc-*.json")
	if err != nil {
		t.Fatalf("Failed to create temp OpenRPC file: %v", err)
	}
	defer os.Remove(tempOpenRPC.Name())

	if _, err := tempOpenRPC.Write(openrpcData); err != nil {
		t.Fatalf("Failed to write test OpenRPC file: %v", err)
	}
	tempOpenRPC.Close()

	rulesContent := `description: "Test rules"
rules:
  info-description:
    description: "Info must have description"
    given: "$.info.description"
    severity: "critical"
    then:
      function: "truthy"
`

	tempRules, err := os.CreateTemp("", "test-rules-*.yml")
	if err != nil {
		t.Fatalf("Failed to create temp rules file: %v", err)
	}
	defer os.Remove(tempRules.Name())

	if _, err := tempRules.WriteString(rulesContent); err != nil {
		t.Fatalf("Failed to write test rules file: %v", err)
	}
	tempRules.Close()

	var output bytes.Buffer
	opts := LintOptions{
		OpenRPCFile: tempOpenRPC.Name(),
		RulesFile:   tempRules.Name(),
		Output:      &output,
	}

	err = RunLint(opts)
	if err == nil {
		t.Fatalf("Expected RunLint to fail for invalid severity")
	}

	if !strings.Contains(err.Error(), "invalid severity") {
		t.Fatalf("Expected invalid severity error, got: %v", err)
	}
}
