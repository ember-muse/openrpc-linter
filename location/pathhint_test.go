package location

import (
	"testing"

	"github.com/open-rpc/openrpc-linter/types"
)

func doc() map[string]any {
	return map[string]any{
		"openrpc": "1.3.2",
		"info": map[string]any{
			"title": "API",
		},
		"methods": []any{
			map[string]any{
				"name": "eth_getLogs",
				"params": []any{
					map[string]any{
						"name": "filter",
						"schema": map[string]any{
							"type": "object",
						},
					},
				},
				"errors": []any{},
			},
			map[string]any{
				"name": "eth_blockNumber",
			},
		},
		"components": map[string]any{
			"schemas": map[string]any{
				"Pet": map[string]any{
					"type": "object",
				},
				"Block": map[string]any{
					"type":  "object",
					"title": "BlockSchema",
				},
			},
			"contentDescriptors": []any{
				map[string]any{
					"name": "chainId",
				},
			},
		},
	}
}

func TestResolveMethodAndField(t *testing.T) {
	labels := Resolve(doc(), `$['methods'][0]['errors']`)
	if labels.Method != "eth_getLogs" {
		t.Fatalf("Method = %q, want eth_getLogs", labels.Method)
	}
}

func TestResolveMethodAndParam(t *testing.T) {
	labels := Resolve(doc(), `$['methods'][0]['params'][0]['schema']['title']`)
	if labels.Method != "eth_getLogs" {
		t.Fatalf("Method = %q, want eth_getLogs", labels.Method)
	}
	if labels.Param != "filter" {
		t.Fatalf("Param = %q, want filter", labels.Param)
	}
}

func TestResolveComponentsSchemaKey(t *testing.T) {
	labels := Resolve(doc(), `$['components']['schemas']['Pet']['title']`)
	if labels.Section != "components" {
		t.Fatalf("Section = %q, want components", labels.Section)
	}
	if labels.Schema != "Pet" {
		t.Fatalf("Schema = %q, want Pet", labels.Schema)
	}
}

func TestResolveComponentsSchemaTitle(t *testing.T) {
	labels := Resolve(doc(), `$['components']['schemas']['Block']['title']`)
	if labels.Schema != "Block" {
		t.Fatalf("Schema = %q, want Block (map key wins over title field)", labels.Schema)
	}
}

func TestResolveContentDescriptor(t *testing.T) {
	labels := Resolve(doc(), `$['components']['contentDescriptors'][0]['name']`)
	if labels.Descriptor != "chainId" {
		t.Fatalf("Descriptor = %q, want chainId", labels.Descriptor)
	}
}

func TestResolveInfoSection(t *testing.T) {
	labels := Resolve(doc(), `$['info']['description']`)
	if labels.Section != "info" {
		t.Fatalf("Section = %q, want info", labels.Section)
	}
}

func TestResolveEmptyForMissingPath(t *testing.T) {
	labels := Resolve(doc(), `$['methods'][99]['errors']`)
	if !labels.IsEmpty() {
		t.Fatalf("expected empty labels for out-of-range index, got %+v", labels)
	}
}

func TestEnrich(t *testing.T) {
	results := []types.RuleFunctionResult{
		{Path: []string{`$['methods'][0]['errors']`}, Message: "missing"},
		{Message: "no path"},
	}
	Enrich(results, doc())
	if results[0].PathLabels.Method != "eth_getLogs" {
		t.Fatalf("enriched Method = %q", results[0].PathLabels.Method)
	}
	if !results[1].PathLabels.IsEmpty() {
		t.Fatalf("expected empty labels for result without path")
	}
}

func TestFriendlyPath(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{`$['methods'][71]['errors']`, "methods[71].errors"},
		{`$['components']['schemas']['Pet']['title']`, "components.schemas.Pet.title"},
		{`$['info']['description']`, "info.description"},
		{"$", "$"},
	}
	for _, tt := range tests {
		if got := FriendlyPath(tt.in); got != tt.want {
			t.Errorf("FriendlyPath(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
