package metaschema

import (
	"errors"
	"strings"
	"testing"
)

func TestForVersion(t *testing.T) {
	tests := []struct {
		version string
		family  string
	}{
		{version: "1.0.0-rc0", family: "1.0.x-1.3.x"},
		{version: "1.2.6", family: "1.0.x-1.3.x"},
		{version: "1.3.2", family: "1.0.x-1.3.x"},
		{version: "1.4.0", family: "1.4.x"},
		{version: "1.4.7", family: "1.4.x"},
	}
	for _, tt := range tests {
		t.Run(tt.version, func(t *testing.T) {
			meta, err := ForVersion(tt.version)
			if err != nil {
				t.Fatal(err)
			}
			if meta.Version != tt.family {
				t.Fatalf("got family %q, want %q", meta.Version, tt.family)
			}
		})
	}

	for _, version := range []string{"1.5.0", "2.0.0", "abc", ""} {
		t.Run("unsupported_"+version, func(t *testing.T) {
			_, err := ForVersion(version)
			if !errors.Is(err, ErrUnsupportedVersion) {
				t.Fatalf("got %v, want ErrUnsupportedVersion", err)
			}
		})
	}
}

func TestForDocument(t *testing.T) {
	tests := []struct {
		name    string
		doc     any
		family  string
		wantErr bool
	}{
		{name: "v1.3", doc: map[string]any{"openrpc": "1.3.2"}, family: "1.0.x-1.3.x"},
		{name: "missing version", doc: map[string]any{}, family: "1.4.x"},
		{name: "non-string version", doc: map[string]any{"openrpc": 1.4}, family: "1.4.x"},
		{name: "unsupported", doc: map[string]any{"openrpc": "9.9.9"}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			meta, err := For(tt.doc)
			if tt.wantErr {
				if !errors.Is(err, ErrUnsupportedVersion) {
					t.Fatalf("got %v, want ErrUnsupportedVersion", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if meta.Version != tt.family {
				t.Fatalf("got family %q, want %q", meta.Version, tt.family)
			}
		})
	}
}

func TestSchemaNavigation(t *testing.T) {
	for _, version := range []string{"1.3.2", "1.4.0"} {
		t.Run(version, func(t *testing.T) {
			meta, err := ForVersion(version)
			if err != nil {
				t.Fatal(err)
			}
			if meta.Root() == nil {
				t.Fatal("Root returned nil")
			}
			if meta.Resolve("#/definitions/methodObject") == nil {
				t.Fatal("methodObject did not resolve")
			}
			if meta.Resolve("https://meta.json-schema.tools") != nil {
				t.Fatal("external reference unexpectedly resolved")
			}
		})
	}
}

func TestExternalJSONSchemaRefsAreRewrittenToDraft7(t *testing.T) {
	for _, version := range []string{"1.3.2", "1.4.0"} {
		t.Run(version, func(t *testing.T) {
			meta, err := ForVersion(version)
			if err != nil {
				t.Fatal(err)
			}
			refs := collectSchemaRefs(meta.Root())
			for _, ref := range refs {
				if strings.Contains(ref, "meta.json-schema.tools") {
					t.Fatalf("external JSON Schema reference was not rewritten: %s", ref)
				}
			}
			if !containsString(refs, draft7URL) {
				t.Fatalf("missing draft-07 schema reference: %v", refs)
			}
			if !containsString(refs, draft7RefURL) {
				t.Fatalf("missing draft-07 $ref definition reference: %v", refs)
			}
		})
	}
}

func TestCompileAndValidateMatchingVersion(t *testing.T) {
	v13, err := ForVersion("1.3.2")
	if err != nil {
		t.Fatal(err)
	}
	v14, err := ForVersion("1.4.0")
	if err != nil {
		t.Fatal(err)
	}
	schema13, err := v13.Compile()
	if err != nil {
		t.Fatalf("compile v1.3: %v", err)
	}
	schema14, err := v14.Compile()
	if err != nil {
		t.Fatalf("compile v1.4: %v", err)
	}

	doc13 := minimumDocument("1.3.2")
	doc14 := minimumDocument("1.4.0")
	if err := schema13.Validate(doc13); err != nil {
		t.Fatalf("v1.3 schema rejected v1.3 document: %v", err)
	}
	if err := schema14.Validate(doc14); err != nil {
		t.Fatalf("v1.4 schema rejected v1.4 document: %v", err)
	}
	if err := schema13.Validate(doc14); err == nil {
		t.Fatal("v1.3 schema accepted v1.4 document")
	}
	if err := schema14.Validate(doc13); err == nil {
		t.Fatal("v1.4 schema accepted v1.3 document")
	}
}

func minimumDocument(version string) map[string]any {
	return map[string]any{
		"openrpc": version,
		"info": map[string]any{
			"title":   "Test API",
			"version": "1.0.0",
		},
		"methods": []any{},
	}
}

func collectSchemaRefs(value any) []string {
	var refs []string
	var walk func(any)
	walk = func(value any) {
		switch node := value.(type) {
		case map[string]any:
			for _, key := range []string{"$schema", "$ref"} {
				if ref, ok := node[key].(string); ok {
					refs = append(refs, ref)
				}
			}
			for _, child := range node {
				walk(child)
			}
		case []any:
			for _, child := range node {
				walk(child)
			}
		}
	}
	walk(value)
	return refs
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
