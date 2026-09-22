package selector

import (
	"sort"
	"testing"

	"github.com/theory/jsonpath"
)

// pathParse keeps the test bodies focused; failures here mean the test
// itself is malformed, not the production code.
func pathParse(t *testing.T, p string) *jsonpath.Path {
	t.Helper()
	parsed, err := jsonpath.Parse(p)
	if err != nil {
		t.Fatalf("parse %q: %v", p, err)
	}
	return parsed
}

func collectFieldTargets(targets []Target) []string {
	out := make([]string, 0, len(targets))
	for _, t := range targets {
		out = append(out, t.PathString())
	}
	sort.Strings(out)
	return out
}

// ---------- Parent-field mode (no schema needed) ----------

func TestSelectParentFieldMissing(t *testing.T) {
	// $.info.description — info exists, description doesn't.
	// Must emit ONE field target with Exists=false. No Index needed:
	// the terminal-name shortcut runs without the schema-aware index.
	doc := docFromJSON(t, `{"info": {"title": "Demo"}}`)
	got := Select(pathParse(t, "$.info.description"), doc, nil)

	if len(got) != 1 {
		t.Fatalf("expected exactly one target, got %+v", got)
	}
	tgt := got[0]
	if tgt.Field != "description" || tgt.Exists {
		t.Fatalf("expected missing-description field target, got %+v", tgt)
	}
	if tgt.PathString() != "$['info']['description']" {
		t.Fatalf("unexpected path: %s", tgt.PathString())
	}
	if tgt.ParentPath.String() != "$['info']" {
		t.Fatalf("unexpected parent path: %s", tgt.ParentPath.String())
	}
}

func TestSelectParentFieldOverWildcard(t *testing.T) {
	// $.methods[*].description against a 2-method doc, one with desc, one without.
	// Must emit two targets in document order with Exists set accordingly.
	doc := docFromJSON(t, `{
        "methods": [
          {"name": "first", "description": "ok"},
          {"name": "second"}
        ]
      }`)
	got := Select(pathParse(t, "$.methods[*].description"), doc, nil)
	if len(got) != 2 {
		t.Fatalf("expected 2 targets, got %+v", got)
	}
	if !got[0].Exists || got[0].Node != "ok" {
		t.Fatalf("first target should be present with value 'ok', got %+v", got[0])
	}
	if got[1].Exists {
		t.Fatalf("second target should be missing, got %+v", got[1])
	}
}

// ---------- Value mode (wildcard / filter terminus) ----------

func TestSelectValueModeOnWildcard(t *testing.T) {
	// $.methods[*] — terminal segment is a wildcard, so we fall through to
	// pure value mode. Targets must have empty Field and Exists=true.
	doc := docFromJSON(t, `{"methods": [{"name":"a"}, {"name":"b"}]}`)
	got := Select(pathParse(t, "$.methods[*]"), doc, nil)
	if len(got) != 2 {
		t.Fatalf("expected 2 value targets, got %+v", got)
	}
	for i, tgt := range got {
		if tgt.Field != "" || !tgt.Exists {
			t.Fatalf("target %d should be value-mode (Field=\"\", Exists=true), got %+v", i, tgt)
		}
	}
}

// ---------- Descendant terminal: schema-aware ----------

func TestSelectDescendantTerminalUsesIndex(t *testing.T) {
	// $..description on a doc with no descriptions anywhere. The index
	// expands to every candidate parent that could legally hold description
	// (info, the single method, and the single content descriptor). All
	// three should be reported as missing.
	doc := docFromJSON(t, `{
        "openrpc": "1.4.0",
        "info": {"title": "Demo", "version": "1.0.0"},
        "methods": [
          {"name": "foo", "params": [{"name": "p"}]}
        ]
      }`)
	idx := Build(doc, metaV14(t))
	got := Select(pathParse(t, "$..description"), doc, idx)

	paths := collectFieldTargets(got)
	expected := []string{
		"$['info']['description']",
		"$['methods'][0]['description']",
		"$['methods'][0]['params'][0]['description']",
	}
	sort.Strings(expected)
	if !equalSlices(paths, expected) {
		t.Fatalf("descendant-terminal targets mismatch.\n got: %v\nwant: %v", paths, expected)
	}
	for _, tgt := range got {
		if tgt.Exists {
			t.Fatalf("every target should be missing, got %+v", tgt)
		}
	}
}

func TestSelectScopedDescendantConfinesToScope(t *testing.T) {
	// $.methods..description should NOT report info.description even though
	// info also accepts a description — the scope before .. is $.methods.
	doc := docFromJSON(t, `{
        "openrpc": "1.4.0",
        "info": {"title": "Demo", "version": "1.0.0"},
        "methods": [
          {"name": "foo", "params": []}
        ]
      }`)
	idx := Build(doc, metaV14(t))
	got := Select(pathParse(t, "$.methods..description"), doc, idx)
	paths := collectFieldTargets(got)
	for _, p := range paths {
		if startsWith(p, "$['info']") {
			t.Fatalf("scoped descendant must not include info.description, got %v", paths)
		}
	}
	if !contains(paths, "$['methods'][0]['description']") {
		t.Fatalf("expected method-level description target, got %v", paths)
	}
}

func TestSelectFilteredScopedDescendant(t *testing.T) {
	// $.methods[?@.deprecated]..description: only inside deprecated methods.
	// This exercises both the filter selector in the prefix and the
	// schema-aware step that follows.
	doc := docFromJSON(t, `{
        "openrpc": "1.4.0",
        "info": {"title": "Demo", "version": "1.0.0"},
        "methods": [
          {"name": "old", "deprecated": true, "params": []},
          {"name": "new", "params": []}
        ]
      }`)
	idx := Build(doc, metaV14(t))
	got := Select(pathParse(t, "$.methods[?@.deprecated]..description"), doc, idx)
	paths := collectFieldTargets(got)
	if !contains(paths, "$['methods'][0]['description']") {
		t.Fatalf("expected deprecated method to be reported, got %v", paths)
	}
	if contains(paths, "$['methods'][1]['description']") {
		t.Fatalf("non-deprecated method must not be reported, got %v", paths)
	}
}

// ---------- Compound descendants ----------

func TestSelectCompoundDescendant(t *testing.T) {
	// $.methods..result.schema — peel the descendant ..result, then evaluate
	// .schema on each resolved result node. The fixture's result lacks a
	// schema, so we expect a single missing-field target.
	doc := docFromJSON(t, `{
        "openrpc": "1.4.0",
        "info": {"title": "Demo", "version": "1.0.0"},
        "methods": [
          {"name": "foo", "params": [], "result": {"name": "r"}}
        ]
      }`)
	idx := Build(doc, metaV14(t))
	got := Select(pathParse(t, "$.methods..result.schema"), doc, idx)

	if len(got) != 1 {
		t.Fatalf("expected 1 compound target, got %+v", got)
	}
	tgt := got[0]
	if tgt.Field != "schema" || tgt.Exists {
		t.Fatalf("expected missing schema field, got %+v", tgt)
	}
	if tgt.PathString() != "$['methods'][0]['result']['schema']" {
		t.Fatalf("unexpected compound target path: %s", tgt.PathString())
	}
}

// ---------- Helpers ----------

func contains(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}

func equalSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
