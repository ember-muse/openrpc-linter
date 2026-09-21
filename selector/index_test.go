package selector

import (
	"encoding/json"
	"testing"

	"github.com/open-rpc/openrpc-linter/metaschema"
	"github.com/theory/jsonpath/spec"
)

func metaV14(t *testing.T) *metaschema.MetaSchema {
	t.Helper()
	meta, err := metaschema.ForVersion("1.4.0")
	if err != nil {
		t.Fatal(err)
	}
	return meta
}

// docFromJSON returns the minimum useful OpenRPC document for index tests.
// Decoded via json.Unmarshal so values match what the linter sees at runtime
// (map[string]any / []any / float64). Avoids any subtle map-typing skew
// between hand-built test fixtures and the production pipeline.
func docFromJSON(t *testing.T, raw string) any {
	t.Helper()
	var v any
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		t.Fatalf("unmarshal fixture: %v", err)
	}
	return v
}

const sampleDoc = `{
  "openrpc": "1.4.0",
  "info": {
    "title": "Demo",
    "version": "1.0.0"
  },
  "methods": [
    {
      "name": "foo",
      "params": [
        {"name": "p1", "schema": {"type": "string"}}
      ]
    }
  ],
  "components": {
    "schemas": {
      "MyType": {"type": "object", "properties": {"a": {"type": "string"}}}
    }
  }
}`

func TestIndexClassifiesKnownObjects(t *testing.T) {
	doc := docFromJSON(t, sampleDoc)
	idx := Build(doc, metaV14(t))

	// The infoObject has a "description" property — even though our fixture
	// document omits it, ByField["description"] must include $.info.
	cands := idx.ByField["description"]
	if !containsPath(cands, "$['info']") {
		t.Fatalf("expected ByField[description] to include $['info'], got: %s",
			renderPaths(cands))
	}
	// methodObject also declares description; the single method at $.methods[0]
	// must therefore appear as a candidate.
	if !containsPath(cands, "$['methods'][0]") {
		t.Fatalf("expected ByField[description] to include $['methods'][0], got: %s",
			renderPaths(cands))
	}
	// contentDescriptorObject (a param) also declares description.
	if !containsPath(cands, "$['methods'][0]['params'][0]") {
		t.Fatalf("expected ByField[description] to include params[0], got: %s",
			renderPaths(cands))
	}
}

func TestIndexStopsAtJSONSchemaBoundary(t *testing.T) {
	doc := docFromJSON(t, sampleDoc)
	idx := Build(doc, metaV14(t))

	// components.schemas/* dereferences to the draft-07 meta-schema, which is
	// an external ref. The walker must stop there: the inner
	// {type:"object", properties:{a:...}} is a JSON Schema, not an OpenRPC
	// object, and indexing it would invent missing-field warnings for
	// arbitrary JSON Schema instances.
	for _, c := range idx.ByField["description"] {
		path := c.Path.String()
		if startsWith(path, "$['components']['schemas']") {
			t.Fatalf("indexing should stop at JSON Schema boundary, but indexed %s", path)
		}
	}
	for _, c := range idx.ByField["type"] {
		path := c.Path.String()
		if startsWith(path, "$['components']['schemas']") {
			t.Fatalf("indexing should not descend into JSON Schema, but indexed %s for ByField[type]", path)
		}
	}
}

func TestIndexExtensionKeysAreNotCandidateFields(t *testing.T) {
	doc := docFromJSON(t, `{
        "openrpc": "1.4.0",
        "info": {"title": "Demo", "version": "1.0.0"},
        "methods": [
          {"name": "foo", "params": [], "x-internal": true}
        ]
      }`)
	idx := Build(doc, metaV14(t))

	// "x-internal" is matched by patternProperties "^x-" in the meta-schema,
	// but extensions are intentionally opaque (specificationExtension). We
	// must NOT register it as a candidate field — otherwise users would get
	// "missing x-internal" diagnostics on every method that omits it.
	if cands, ok := idx.ByField["x-internal"]; ok {
		t.Fatalf("extension keys must not appear in ByField, got %d candidates", len(cands))
	}
}

func TestIndexPathStringsMatchJSONPath(t *testing.T) {
	// The whole point of using spec.NormalizedPath internally is that the
	// strings produced by our index render *identically* to the strings
	// jsonpath.SelectLocated produces. Verify directly so a regression
	// (e.g. someone introduces a custom renderer) shows up immediately.
	path := spec.NormalizedPath{spec.Name("methods"), spec.Index(0), spec.Name("params"), spec.Index(0)}
	got := path.String()
	want := "$['methods'][0]['params'][0]"
	if got != want {
		t.Fatalf("normalized path render: got %q want %q", got, want)
	}
}

func containsPath(cands []*Candidate, want string) bool {
	for _, c := range cands {
		if c.Path.String() == want {
			return true
		}
	}
	return false
}

func renderPaths(cands []*Candidate) string {
	out := ""
	for i, c := range cands {
		if i > 0 {
			out += ", "
		}
		out += c.Path.String()
	}
	return out
}

func startsWith(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}
