package e2e_test

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/cucumber/godog"

	"github.com/open-rpc/openrpc-linter/cmd"
)

// canonicalDoc is a fully populated OpenRPC document. It satisfies every rule
// in the recommended ruleset. Tests mutate copies to trigger specific rules.
const canonicalDoc = `{
  "openrpc": "1.2.6",
  "info": {
    "title": "Recommended API",
    "description": "Canonical fixture exercising every recommended rule.",
    "version": "1.0.0",
    "license": {"name": "MIT"}
  },
  "methods": [
    {
      "name": "ping",
      "summary": "Ping the server",
      "description": "Returns a pong response when the server is healthy.",
      "tags": [
        {"name": "health", "description": "Health related operations"}
      ],
      "params": [
        {
          "name": "message",
          "summary": "Echo message",
          "description": "Message to echo back to the caller.",
          "schema": {
            "type": "string",
            "title": "Message",
            "description": "Free-form text to echo."
          }
        }
      ],
      "result": {
        "name": "pong",
        "description": "The pong response value.",
        "schema": {
          "type": "string",
          "title": "Pong",
          "description": "Echoed message."
        }
      },
      "errors": [
        {"code": 100, "message": "boom"}
      ],
      "examples": [
        {
          "name": "ping example",
          "description": "Calls ping and gets pong.",
          "params": [
            {"name": "message", "value": "hi"}
          ],
          "result": {"name": "pong", "value": "hi"}
        }
      ]
    }
  ]
}`

func init() {
	registerRecommendedSteps = initializeRecommendedScenario
}

func initializeRecommendedScenario(sc *godog.ScenarioContext, s *lintScenario) {
	sc.Step(`^the bundled recommended ruleset is loaded$`, func() error {
		name, err := s.writeTempFile("rules-*.yml", `extends:
  - recommended
`)
		if err != nil {
			return err
		}
		s.rulesFile = name
		return nil
	})

	sc.Step(`^a fully populated OpenRPC document covering every approved rule$`, func() error {
		var doc map[string]any
		if err := json.Unmarshal([]byte(canonicalDoc), &doc); err != nil {
			return err
		}
		s.docData = doc
		return nil
	})

	sc.Step(`^the document is missing "([^"]*)"$`, func(path string) error {
		return deleteAtPath(s.docData, path)
	})

	sc.Step(`^the document is mutated so that "([^"]*)" is set to an empty array$`, func(path string) error {
		return setAtPath(s.docData, path, []any{})
	})

	sc.Step(`^the document is mutated so that "([^"]*)" is set to an array of (\d+) params$`, func(path string, n int) error {
		params := make([]any, 0, n)
		for i := range n {
			params = append(params, map[string]any{
				"name":        fmt.Sprintf("p%d", i),
				"summary":     fmt.Sprintf("P%d", i),
				"description": fmt.Sprintf("Param %d", i),
				"schema": map[string]any{
					"type":        "string",
					"title":       fmt.Sprintf("P%d", i),
					"description": "Schema desc.",
				},
			})
		}
		return setAtPath(s.docData, path, params)
	})

	sc.Step(`^the document is mutated so that "([^"]*)" is set to a (\d+) character string$`, func(path string, n int) error {
		return setAtPath(s.docData, path, strings.Repeat("x", n))
	})

	sc.Step(`^the linter exits non-zero$`, func() error {
		if s.err != nil {
			return nil
		}
		return fmt.Errorf("expected linter to exit non-zero, output:\n%s", s.output.String())
	})

	sc.Step(`^the linter exits zero$`, func() error {
		if s.err == nil {
			return nil
		}
		return fmt.Errorf("expected linter to exit zero, got %v\nOutput:\n%s", s.err, s.output.String())
	})

	sc.Step(`^the lint output should mention rule "([^"]*)"$`, func(rule string) error {
		if strings.Contains(s.output.String(), rule) {
			return nil
		}
		return fmt.Errorf("expected output to mention rule %q, got:\n%s", rule, s.output.String())
	})

	sc.Step(`^the lint output should contain a warning for rule "([^"]*)"$`, func(rule string) error {
		output := s.output.String()
		if strings.Contains(output, "warning") && strings.Contains(output, rule) {
			return nil
		}
		return fmt.Errorf("expected warning output for rule %q, got:\n%s", rule, output)
	})

	sc.Step(`^the lint output should not mention rule "([^"]*)"$`, func(rule string) error {
		if !strings.Contains(s.output.String(), rule) {
			return nil
		}
		return fmt.Errorf("expected output to NOT mention rule %q, got:\n%s", rule, s.output.String())
	})

}

func (s *lintScenario) runWithDoc() error {
	if s.docData == nil {
		return fmt.Errorf("no document data set")
	}
	b, err := json.MarshalIndent(s.docData, "", "  ")
	if err != nil {
		return err
	}
	name, err := s.writeTempFile("openrpc-*.json", string(b))
	if err != nil {
		return err
	}
	s.openRPCFile = name
	s.output.Reset()
	s.err = cmd.RunLint(cmd.LintOptions{
		OpenRPCFile: s.openRPCFile,
		RulesFile:   s.rulesFile,
		Output:      &s.output,
		Format:      "text",
	})
	return nil
}

// pathSegments parses dotted paths with bracket indexing.
// "methods[0].params[0].description" -> ["methods", 0, "params", 0, "description"]
var pathTokenRe = regexp.MustCompile(`([^.\[\]]+)|\[(\d+)\]`)

func parsePath(path string) []any {
	matches := pathTokenRe.FindAllStringSubmatch(path, -1)
	out := make([]any, 0, len(matches))
	for _, m := range matches {
		if m[2] != "" {
			i, _ := strconv.Atoi(m[2])
			out = append(out, i)
			continue
		}
		out = append(out, m[1])
	}
	return out
}

func navigate(root any, segs []any) (any, error) {
	cur := root
	for _, seg := range segs {
		switch s := seg.(type) {
		case string:
			m, ok := cur.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("expected object at segment %v", seg)
			}
			cur = m[s]
		case int:
			arr, ok := cur.([]any)
			if !ok {
				return nil, fmt.Errorf("expected array at segment %v", seg)
			}
			if s >= len(arr) {
				return nil, fmt.Errorf("index %d out of range", s)
			}
			cur = arr[s]
		}
	}
	return cur, nil
}

func deleteAtPath(root any, path string) error {
	segs := parsePath(path)
	if len(segs) == 0 {
		return fmt.Errorf("empty path")
	}
	parent, err := navigate(root, segs[:len(segs)-1])
	if err != nil {
		return err
	}
	last := segs[len(segs)-1]
	switch k := last.(type) {
	case string:
		m, ok := parent.(map[string]any)
		if !ok {
			return fmt.Errorf("cannot delete key from non-object")
		}
		delete(m, k)
		return nil
	case int:
		arr, ok := parent.([]any)
		if !ok {
			return fmt.Errorf("cannot delete index from non-array")
		}
		_ = arr
		_ = k
		return fmt.Errorf("array index delete not supported")
	}
	return nil
}

func setAtPath(root any, path string, value any) error {
	segs := parsePath(path)
	if len(segs) == 0 {
		return fmt.Errorf("empty path")
	}
	parent, err := navigate(root, segs[:len(segs)-1])
	if err != nil {
		return err
	}
	last := segs[len(segs)-1]
	switch k := last.(type) {
	case string:
		m, ok := parent.(map[string]any)
		if !ok {
			return fmt.Errorf("cannot set key on non-object")
		}
		m[k] = value
		return nil
	case int:
		arr, ok := parent.([]any)
		if !ok {
			return fmt.Errorf("cannot set index on non-array")
		}
		if k >= len(arr) {
			return fmt.Errorf("index out of range")
		}
		arr[k] = value
		return nil
	}
	return nil
}
