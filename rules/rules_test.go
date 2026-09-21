package rules

import (
	"reflect"
	"strings"
	"testing"

	"github.com/open-rpc/openrpc-linter/functions"
	"github.com/open-rpc/openrpc-linter/types"

	"gopkg.in/yaml.v3"
)

type givenPathCaptureRule struct{}

func (r *givenPathCaptureRule) RunRule(value interface{}, context types.RuleFunctionContext) []types.RuleFunctionResult {
	if context.GivenPath == nil {
		return []types.RuleFunctionResult{{Message: "missing given path"}}
	}
	return nil
}

func TestDefaultRulesRecommended(t *testing.T) {
	w, err := LoadRulesFile(GetRuleDefaultsFS(), "recommended.yaml")
	if err != nil {
		t.Fatalf("load recommended: %v", err)
	}
	for _, name := range []string{"info-description", "method-description", "method-errors", "method-examples"} {
		if _, ok := w.Rules[name]; !ok {
			t.Errorf("missing rule %q", name)
		}
	}
}

func TestRulesYAMLEmptyRulesWithExtends(t *testing.T) {
	// rules.yml-style document with only `extends:` and no `rules:` map.
	// CheckRules must accept it and ResolvedRules must return the inherited set.
	src := []byte(`extends:
  - recommended
`)

	var rw RulesWrapper
	if err := yaml.Unmarshal(src, &rw); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if rw.Rules != nil {
		t.Fatalf("expected nil Rules map from yaml, got %#v", rw.Rules)
	}
	if err := rw.CheckRules(); err != nil {
		t.Fatalf("CheckRules should accept empty rules when extends is set: %v", err)
	}

	merged, err := rw.ResolvedRules()
	if err != nil {
		t.Fatalf("ResolvedRules: %v", err)
	}
	for _, name := range []string{"info-description", "method-description", "method-errors", "method-examples"} {
		if _, ok := merged[name]; !ok {
			t.Errorf("expected inherited rule %q from recommended extension", name)
		}
	}
}

func TestRulesYAMLEmptyRulesAndExtendsRejected(t *testing.T) {
	// rules.yml-style document with neither `rules:` nor `extends:`.
	src := []byte(`description: "empty"
`)

	var rw RulesWrapper
	if err := yaml.Unmarshal(src, &rw); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	err := rw.CheckRules()
	if err == nil {
		t.Fatal("expected CheckRules to reject empty rules and extends")
	}
	if !strings.Contains(err.Error(), "no rules to merge") {
		t.Errorf("expected 'no rules to merge' error, got: %v", err)
	}
}

func TestResolvedRulesExtends(t *testing.T) {
	rw := &RulesWrapper{
		Extends: []types.RuleDefaults{types.RuleExtensionRecommended},
		Rules: map[string]types.Rule{
			"info-title": {
				Description: "override",
				Given:       "$.info.title",
				Then: &types.RuleAction{
					Function: "truthy",
				},
			},
		},
	}
	merged, err := rw.ResolvedRules()
	if err != nil {
		t.Fatal(err)
	}
	if merged["info-title"].Given != "$.info.title" {
		t.Errorf("user rule should override recommended, got %+v", merged["info-title"])
	}
	if _, ok := merged["method-errors"]; !ok {
		t.Errorf("expected recommended rule method-errors to be merged in")
	}
}

func TestExecuteRule(t *testing.T) {
	tests := []struct {
		name         string
		rule         *types.Rule
		document     interface{}
		context      types.RuleFunctionContext
		expectError  bool
		expectedMsg  string
		expectedPath []string
	}{
		{
			name: "truthy rule with missing field",
			rule: &types.Rule{
				Description: "Test missing field",
				Given:       "$.info.description",
				Then: &types.RuleAction{
					Function: "truthy",
				},
			},
			document: map[string]interface{}{
				"info": map[string]interface{}{
					"title":   "Test API",
					"version": "1.0.0",
					// no description field
				},
			},
			expectError: true,
			expectedMsg: "missing field 'description'",
		},
		{
			name: "truthy rule with missing field on selected method includes path",
			rule: &types.Rule{
				Description: "Test missing method description",
				Given:       "$.methods[*].description",
				Then: &types.RuleAction{
					Function: "truthy",
				},
			},
			document: map[string]interface{}{
				"methods": []interface{}{
					map[string]interface{}{
						"name": "ping",
					},
				},
			},
			expectError:  true,
			expectedMsg:  "missing field 'description'",
			expectedPath: []string{"$['methods'][0]['description']"},
		},
		{
			name: "truthy rule with present field",
			rule: &types.Rule{
				Description: "Test present field",
				Given:       "$.info.description",
				Then: &types.RuleAction{
					Function: "truthy",
				},
			},
			document: map[string]interface{}{
				"info": map[string]interface{}{
					"title":       "Test API",
					"version":     "1.0.0",
					"description": "A test API",
				},
			},
			expectError: false,
		},
		{
			name: "truthy rule ignores functionOptions field",
			rule: &types.Rule{
				Description: "Test ignored field option",
				Given:       "$.info",
				Then: &types.RuleAction{
					Function:        "truthy",
					FunctionOptions: map[string]interface{}{"field": "description"},
				},
			},
			document: map[string]interface{}{
				"info": map[string]interface{}{
					"title": "Test API",
				},
			},
			expectError: false,
		},
		{
			name: "truthy rule with falsey selected value",
			rule: &types.Rule{
				Description: "Test falsey field",
				Given:       "$.info.description",
				Then: &types.RuleAction{
					Function: "truthy",
				},
			},
			document: map[string]interface{}{
				"info": map[string]interface{}{
					"description": "",
				},
			},
			expectError:  true,
			expectedMsg:  "Field must have a truthy value",
			expectedPath: []string{"$['info']['description']"},
		},
		{
			name: "schema rule with invalid methods length",
			rule: &types.Rule{
				Description: "Test methods length",
				Given:       "$.methods",
				Then: &types.RuleAction{
					Function: "schema",
					FunctionOptions: map[string]interface{}{
						"type":     "array",
						"minItems": 1,
					},
				},
			},
			document: map[string]interface{}{
				"methods": []interface{}{},
			},
			expectError: true,
			expectedMsg: "Value does not match schema:",
		},
		{
			name: "unknown function",
			rule: &types.Rule{
				Description: "Test unknown function",
				Given:       "$.info",
				Then: &types.RuleAction{
					Function: "unknownFunction",
				},
			},
			document: map[string]interface{}{
				"info": map[string]interface{}{
					"title": "Test API",
				},
			},
			expectError: true,
			expectedMsg: "unknown function: unknownFunction",
		},
		{
			name: "jsonpath with no matches",
			rule: &types.Rule{
				Description: "Test missing path",
				Given:       "$.nonexistent[*]",
				Then: &types.RuleAction{
					Function: "truthy",
				},
			},
			document: map[string]interface{}{
				"info": map[string]interface{}{
					"title": "Test API",
				},
			},
			expectError: false,
		},
		{
			name: "invalid jsonpath",
			rule: &types.Rule{
				Description: "Test invalid path",
				Given:       "$.info[",
				Then: &types.RuleAction{
					Function: "truthy",
				},
			},
			document: map[string]interface{}{
				"info": map[string]interface{}{
					"title": "Test API",
				},
			},
			expectError: true,
			expectedMsg: "error parsing JSON path:",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			context := types.RuleFunctionContext{
				Rule:     tt.rule,
				RuleID:   "test-rule",
				Document: tt.document,
			}
			tt.context = context

			results, err := ExecuteRule(tt.rule, tt.context)

			if tt.expectError {
				if err == nil && (len(results) == 0 || (len(results) > 0 && (results[0].Message == "" || results[0].Message == "Result: <nil>"))) {
					t.Errorf("Expected error message, but got results: %+v, err: %+v", results, err)
				}
				if tt.expectedMsg != "" && err != nil && !strings.HasPrefix(err.Error(), tt.expectedMsg) {
					t.Errorf("Expected error message prefix %q, got %q", tt.expectedMsg, err.Error())
				}
				if tt.expectedMsg != "" && err == nil && len(results) > 0 && !strings.HasPrefix(results[0].Message, tt.expectedMsg) {
					t.Errorf("Expected result message prefix %q, got %q", tt.expectedMsg, results[0].Message)
				}
				if tt.expectedPath != nil {
					if len(results) != 1 {
						t.Fatalf("Expected one result, got %+v", results)
					}
					if !reflect.DeepEqual(results[0].Path, tt.expectedPath) {
						t.Fatalf("Expected path %+v, got %+v", tt.expectedPath, results[0].Path)
					}
				}
			} else {
				if err != nil {
					t.Errorf("Expected success, but got error: %v", err)
				}
				// For success cases, we expect either no results or results with "Result:" messages
				for _, result := range results {
					if result.Message != "" && !strings.HasPrefix(result.Message, "Result:") {
						t.Errorf("Expected success (empty or Result: message), but got: %q", result.Message)
					}
				}
			}
		})
	}
}

func TestExecuteRuleTruthyReportsMissingFieldsUnderWildcardParent(t *testing.T) {
	rule := &types.Rule{
		Description: "Method descriptions",
		Given:       "$.methods[*].description",
		Then: &types.RuleAction{
			Function: "truthy",
		},
	}
	document := map[string]interface{}{
		"methods": []interface{}{
			map[string]interface{}{"name": "first", "description": "First method"},
			map[string]interface{}{"name": "second"},
			map[string]interface{}{"name": "third", "description": ""},
		},
	}

	results, err := ExecuteRule(rule, types.RuleFunctionContext{
		Rule:     rule,
		Document: document,
	})
	if err != nil {
		t.Fatalf("expected truthy rule to execute successfully, got: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected missing and falsey description results, got %+v", results)
	}
	if results[0].Message != "missing field 'description'" {
		t.Fatalf("unexpected missing-field result: %+v", results[0])
	}
	if !reflect.DeepEqual(results[0].Path, []string{"$['methods'][1]['description']"}) {
		t.Fatalf("expected missing field path, got %+v", results[0].Path)
	}
	if results[1].Message != "Field must have a truthy value" {
		t.Fatalf("unexpected falsey result: %+v", results[1])
	}
	if !reflect.DeepEqual(results[1].Path, []string{"$['methods'][2]['description']"}) {
		t.Fatalf("expected falsey field path, got %+v", results[1].Path)
	}
}

func TestExecuteRulePassesGivenPathToRuleFunctions(t *testing.T) {
	const functionName = "captureGivenPath"
	previous := functions.FunctionRegistry[functionName]
	functions.FunctionRegistry[functionName] = func() types.RuleFunction { return &givenPathCaptureRule{} }
	defer func() {
		if previous == nil {
			delete(functions.FunctionRegistry, functionName)
			return
		}
		functions.FunctionRegistry[functionName] = previous
	}()

	rule := &types.Rule{
		Description: "Capture path",
		Given:       "$.info.title",
		Then: &types.RuleAction{
			Function: functionName,
		},
	}
	document := map[string]interface{}{
		"info": map[string]interface{}{"title": "Test API"},
	}

	results, err := ExecuteRule(rule, types.RuleFunctionContext{
		Rule:     rule,
		Document: document,
	})
	if err != nil {
		t.Fatalf("expected capture rule to execute successfully, got: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("expected GivenPath to be available, got %+v", results)
	}
}

func TestExecuteRuleUniqueUsesResolvedDocumentAndAddsFieldPath(t *testing.T) {
	rule := &types.Rule{
		Description: "Unique resolved method names",
		Given:       "$.methods[*].name",
		Then: &types.RuleAction{
			Function: "unique",
		},
	}
	originalDocument := map[string]interface{}{
		"methods": []interface{}{
			map[string]interface{}{"name": "ping"},
			map[string]interface{}{"name": "pong"},
		},
	}
	resolvedDocument := map[string]interface{}{
		"methods": []interface{}{
			map[string]interface{}{"name": "ping"},
			map[string]interface{}{"name": "ping"},
		},
	}

	results, err := ExecuteRule(rule, types.RuleFunctionContext{
		Rule:             rule,
		Document:         originalDocument,
		ResolvedDocument: resolvedDocument,
	})
	if err != nil {
		t.Fatalf("expected unique rule to execute successfully, got: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected one duplicate result from resolved document, got %+v", results)
	}
	if results[0].Message != `Duplicate value "ping" (first seen at $['methods'][0]['name'])` {
		t.Fatalf("unexpected duplicate message: %+v", results)
	}
	if !reflect.DeepEqual(results[0].Path, []string{"$['methods'][1]['name']"}) {
		t.Fatalf("expected duplicate field path on unique result, got %+v", results[0].Path)
	}
}

func TestExecuteRuleUniqueScopesNestedWildcardCollections(t *testing.T) {
	rule := &types.Rule{
		Description: "Unique param names per method",
		Given:       "$.methods[*].params[*].name",
		Then: &types.RuleAction{
			Function: "unique",
			FunctionOptions: map[string]interface{}{
				"scope": "$.methods[*]",
			},
		},
	}
	document := map[string]interface{}{
		"methods": []interface{}{
			map[string]interface{}{
				"name": "first",
				"params": []interface{}{
					map[string]interface{}{"name": "id"},
				},
			},
			map[string]interface{}{
				"name": "second",
				"params": []interface{}{
					map[string]interface{}{"name": "id"},
					map[string]interface{}{"name": "id"},
				},
			},
		},
	}

	results, err := ExecuteRule(rule, types.RuleFunctionContext{
		Rule:     rule,
		Document: document,
	})
	if err != nil {
		t.Fatalf("expected unique rule to execute successfully, got: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected one duplicate result scoped to the second method, got %+v", results)
	}
	if !reflect.DeepEqual(results[0].Path, []string{"$['methods'][1]['params'][1]['name']"}) {
		t.Fatalf("expected nested duplicate field path, got %+v", results[0].Path)
	}
}

func TestExecuteRuleUniqueGlobalAcrossMethodsWithoutScope(t *testing.T) {
	rule := &types.Rule{
		Description: "Globally unique param names without scope",
		Given:       "$.methods[*].params[*].name",
		Then: &types.RuleAction{
			Function: "unique",
		},
	}
	document := map[string]interface{}{
		"methods": []interface{}{
			map[string]interface{}{
				"name": "first",
				"params": []interface{}{
					map[string]interface{}{"name": "id"},
				},
			},
			map[string]interface{}{
				"name": "second",
				"params": []interface{}{
					map[string]interface{}{"name": "id"},
					map[string]interface{}{"name": "id"},
				},
			},
		},
	}

	results, err := ExecuteRule(rule, types.RuleFunctionContext{
		Rule:     rule,
		Document: document,
	})
	if err != nil {
		t.Fatalf("expected unique rule to execute successfully, got: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected two duplicate results across methods without scope, got %+v", results)
	}
	expectedPaths := [][]string{
		{"$['methods'][1]['params'][0]['name']"},
		{"$['methods'][1]['params'][1]['name']"},
	}
	for i, want := range expectedPaths {
		if !reflect.DeepEqual(results[i].Path, want) {
			t.Fatalf("expected path %+v at index %d, got %+v", want, i, results[i].Path)
		}
	}
}

func TestExecuteRuleUniqueHandlesMapBackedCollections(t *testing.T) {
	rule := &types.Rule{
		Description: "Unique schema titles",
		Given:       "$.components.schemas[*].title",
		Then: &types.RuleAction{
			Function: "unique",
		},
	}
	document := map[string]interface{}{
		"components": map[string]interface{}{
			"schemas": map[string]interface{}{
				"Balance": map[string]interface{}{"title": "Shared"},
				"Amount":  map[string]interface{}{"title": "Shared"},
			},
		},
	}

	results, err := ExecuteRule(rule, types.RuleFunctionContext{
		Rule:     rule,
		Document: document,
	})
	if err != nil {
		t.Fatalf("expected unique rule to execute successfully, got: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected one duplicate result for map-backed collection, got %+v", results)
	}
	if results[0].Message == "" {
		t.Fatalf("expected non-empty duplicate message: %+v", results)
	}
	if len(results[0].Path) != 1 {
		t.Fatalf("expected map duplicate to include a path, got %+v", results[0].Path)
	}
}

func TestExecuteRuleUniqueNoMatchesReturnsNoResults(t *testing.T) {
	rule := &types.Rule{
		Description: "Unique missing schemas",
		Given:       "$.components.schemas[*].title",
		Then: &types.RuleAction{
			Function: "unique",
		},
	}
	document := map[string]interface{}{
		"components": map[string]interface{}{},
	}

	results, err := ExecuteRule(rule, types.RuleFunctionContext{
		Rule:     rule,
		Document: document,
	})
	if err != nil {
		t.Fatalf("expected no-match unique rule to execute successfully, got: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("expected no results for no-match unique rule, got %+v", results)
	}
}

func TestGetFieldFromNode(t *testing.T) {
	tests := []struct {
		name     string
		node     *yaml.Node
		field    string
		expected *yaml.Node
	}{
		{
			name: "field found",
			node: &yaml.Node{
				Kind: yaml.MappingNode,
				Content: []*yaml.Node{
					{Kind: yaml.ScalarNode, Value: "key1"},
					{Kind: yaml.ScalarNode, Value: "value1"},
					{Kind: yaml.ScalarNode, Value: "key2"},
					{Kind: yaml.ScalarNode, Value: "value2"},
				},
			},
			field:    "key2",
			expected: &yaml.Node{Kind: yaml.ScalarNode, Value: "value2"},
		},
		{
			name: "field not found",
			node: &yaml.Node{
				Kind: yaml.MappingNode,
				Content: []*yaml.Node{
					{Kind: yaml.ScalarNode, Value: "key1"},
					{Kind: yaml.ScalarNode, Value: "value1"},
				},
			},
			field:    "nonexistent",
			expected: nil,
		},
		{
			name: "empty node",
			node: &yaml.Node{
				Kind:    yaml.MappingNode,
				Content: []*yaml.Node{},
			},
			field:    "anykey",
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetFieldFromNode(tt.node, tt.field)

			if tt.expected == nil {
				if result != nil {
					t.Errorf("GetFieldFromNode() returned %v, expected nil", result)
				}
			} else {
				if result == nil {
					t.Errorf("GetFieldFromNode() returned nil, expected %v", tt.expected)
				} else if result.Value != tt.expected.Value {
					t.Errorf("GetFieldFromNode() returned value %q, expected %q", result.Value, tt.expected.Value)
				}
			}
		})
	}
}

// Benchmark test for ExecuteRule
func BenchmarkExecuteRule(b *testing.B) {
	rule := &types.Rule{
		Description: "Benchmark rule",
		Given:       "$.info.description",
		Then: &types.RuleAction{
			Function: "truthy",
		},
	}

	document := map[string]interface{}{
		"info": map[string]interface{}{
			"title":   "Test API",
			"version": "1.0.0",
		},
	}

	context := types.RuleFunctionContext{
		Rule:     rule,
		RuleID:   "benchmark-rule",
		Document: document,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := ExecuteRule(rule, context)
		if err != nil {
			b.Fatalf("Execute rule: %v", err)
		}
	}
}
