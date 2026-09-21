package selector

import (
	"strings"

	"github.com/open-rpc/openrpc-linter/metaschema"
	"github.com/theory/jsonpath/spec"
)

// Candidate records a document object that the meta-schema recognizes as
// belonging to a named OpenRPC type. Fields is the set of properties that
// the matched schema declares
type Candidate struct {
	Path        spec.NormalizedPath
	Node        any
	SchemaTitle string
	Fields      map[string]struct{}
}

// Index answers two questions used by the selector:
//
//	ByField["description"]  -> every Candidate whose schema declares description
//	ByPath[path.String()]   -> the Candidate at that exact document position
type Index struct {
	ByField map[string][]*Candidate
	ByPath  map[string]*Candidate
}

// Build walks doc using meta to classify each object and collect candidates.
// Assumes $refs are already resolved.
func Build(doc any, meta *metaschema.MetaSchema) *Index {
	idx := &Index{
		ByField: make(map[string][]*Candidate),
		ByPath:  make(map[string]*Candidate),
	}
	b := &builder{meta: meta, idx: idx}
	b.walk(doc, meta.Root(), spec.NormalizedPath{})
	return idx
}

type builder struct {
	meta *metaschema.MetaSchema
	idx  *Index
}

// walk recurses over (node, schema) in lockstep. node is the document
// value; schema is the meta-schema fragment that describes it.
func (b *builder) walk(node any, schema map[string]any, path spec.NormalizedPath) {
	if schema == nil {
		return
	}

	schema = b.deref(schema)
	if schema == nil {
		// External ref (JSON Schema boundary)
		return
	}

	if branch := b.chooseOneOf(schema); branch != nil {
		schema = branch
	}

	switch v := node.(type) {
	case map[string]any:
		b.indexObject(v, schema, path)
	case []any:
		b.indexArray(v, schema, path)
	}
}

func (b *builder) indexObject(obj map[string]any, schema map[string]any, path spec.NormalizedPath) {
	props := mapOf(schema["properties"])

	// Record this object as a candidate if the schema names at least one
	// property — otherwise the node is generic (e.g. a JSONSchema we've
	// declined to index) and has nothing to contribute.
	if len(props) > 0 {
		cand := &Candidate{
			Path:        append(spec.NormalizedPath{}, path...),
			Node:        obj,
			SchemaTitle: stringOf(schema["title"]),
			Fields:      make(map[string]struct{}, len(props)),
		}
		for field := range props {
			cand.Fields[field] = struct{}{}
		}
		b.idx.ByPath[cand.Path.String()] = cand
		for field := range cand.Fields {
			b.idx.ByField[field] = append(b.idx.ByField[field], cand)
		}
	}

	// Recurse into every actual document key whose schema position we can
	// determine. Unknown keys (extensions, "x-*" via patternProperties, or
	// genuinely unrecognized) are skipped — they don't break indexing,
	// they just don't get classified.
	for key, child := range obj {
		childSchema := b.schemaForKey(schema, key)
		if childSchema == nil {
			continue
		}
		b.walk(child, childSchema, append(path, spec.Name(key)))
	}
}

func (b *builder) indexArray(arr []any, schema map[string]any, path spec.NormalizedPath) {
	itemSchema := mapOf(schema["items"])
	if itemSchema == nil {
		return
	}
	for i, item := range arr {
		b.walk(item, itemSchema, append(path, spec.Index(i)))
	}
}

// schemaForKey finds the schema fragment governing a specific child key.
// properties wins; patternProperties is consulted next; additionalProperties
// is consulted last. Returns nil for keys the schema doesn't know about
// (which is normal for "x-" extensions when additionalProperties is false).
func (b *builder) schemaForKey(schema map[string]any, key string) map[string]any {
	if props := mapOf(schema["properties"]); props != nil {
		if direct, ok := props[key]; ok {
			return mapOf(direct)
		}
	}
	if patternProps := mapOf(schema["patternProperties"]); patternProps != nil {
		for pattern, candidate := range patternProps {
			if matchesPattern(pattern, key) {
				return mapOf(candidate)
			}
		}
	}
	if addl, ok := schema["additionalProperties"]; ok {
		if m, ok := addl.(map[string]any); ok {
			return m
		}
	}
	return nil
}

// deref returns the resolved local definition if schema is a {"$ref": "#/definitions/..."}
// object. For external refs it returns nil to signal "stop here". For
// schemas with no $ref it returns schema unchanged.
func (b *builder) deref(schema map[string]any) map[string]any {
	ref, ok := schema["$ref"].(string)
	if !ok {
		return schema
	}
	return b.meta.Resolve(ref)
}

// chooseOneOf picks the concrete branch from a v1.4-style oneOf. Every
// oneOf in the meta-schema is [concreteOpenRPCType, referenceObject];
func (b *builder) chooseOneOf(schema map[string]any) map[string]any {
	raw, ok := schema["oneOf"].([]any)
	if !ok {
		return nil
	}
	for _, branch := range raw {
		m, ok := branch.(map[string]any)
		if !ok {
			continue
		}
		resolved := b.deref(m)
		if resolved == nil || stringOf(resolved["title"]) == "referenceObject" {
			continue
		}
		return resolved
	}
	return nil
}

// matchesPattern is a deliberately tiny matcher: we only need to recognize
// the "^x-" extension pattern that the meta-schema uses. A full regex
// engine would be overkill and a security liability for a meta-schema that
// changes once per OpenRPC release.
func matchesPattern(pattern, key string) bool {
	switch pattern {
	case "^x-":
		return strings.HasPrefix(key, "x-")
	case "[0-z]+":
		return key != ""
	}
	return false
}

func mapOf(v any) map[string]any {
	m, _ := v.(map[string]any)
	return m
}

func stringOf(v any) string {
	s, _ := v.(string)
	return s
}
