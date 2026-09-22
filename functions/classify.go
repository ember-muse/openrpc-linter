package functions

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"strings"
	"text/template"
	"time"

	"github.com/open-rpc/openrpc-linter/types"
)

// ClassifyRule is an EXPERIMENTAL rule function: it sends the matched value
// plus Jev-typed questions to a Jev classifier and flags targets via assertions.
//
//	functionOptions:
//	  endpoint: https://api.typesafe.ai  # base URL of any Jev-shaped server
//	  model: jev-latest
//	  apiKey: ...                         # or TYPESAFE_API_KEY
//	  stateField: description             # field of the target to classify; "" = whole value
//	  questions: {id: {type, instructions, criteria}}  # choice | score | noul
//	  assert:                             # per question id
//	    <id>: {below, above}              # score/noul thresholds
//	    <id>: {is, in, [notIn]}           # choice
//	    <id>: {message}                   # supports {{.Choice}} {{.Score}} {{.Noul}} {{.Confidence}}
//
// Answers are cached per (model, state, questions) within a run.
type ClassifyRule struct {
	setup *classifySetup
	cache map[string]Response
}

type classifySetup struct {
	backend *JevBackend
	classifyOptions
}

type classifyAssertion struct {
	Below   *float64 `json:"below"`
	Above   *float64 `json:"above"`
	Is      string   `json:"is"`
	In      []string `json:"in"`
	NotIn   []string `json:"notIn"`
	Message string   `json:"message"`
}

type classifyOptions struct {
	Endpoint   string                       `json:"endpoint"`
	Model      string                       `json:"model"`
	APIKey     string                       `json:"apiKey"`
	StateField string                       `json:"stateField"`
	Questions  map[string]Question          `json:"questions"`
	Assert     map[string]classifyAssertion `json:"assert"`
}

// RunRule implements types.RuleFunction.
func (r *ClassifyRule) RunRule(value interface{}, context types.RuleFunctionContext) []types.RuleFunctionResult {
	if t := context.Target; t != nil && t.Field != "" && !t.Exists {
		return nil
	}
	then := context.Rule.Then
	if then == nil || len(then.FunctionOptions) == 0 {
		return []types.RuleFunctionResult{{Message: "classify function requires functionOptions"}}
	}
	if r.setup == nil {
		s, err := buildClassifySetup(then.FunctionOptions)
		if err != nil {
			return []types.RuleFunctionResult{{Message: fmt.Sprintf("classify: %v", err)}}
		}
		r.setup = s
	}
	s := r.setup
	path := rulePath(context)

	state := classifyState(value, s.StateField)
	if state == nil {
		return nil
	}
	if str, ok := state.(string); ok && strings.TrimSpace(str) == "" {
		return nil
	}

	resp, err := r.classifyCached(s, state)
	if err != nil {
		return []types.RuleFunctionResult{{
			Message: fmt.Sprintf("classify: backend error: %v", err),
			Path:    resultPath(path),
		}}
	}
	return assertClassifyAnswers(s, resp, path)
}

func rulePath(context types.RuleFunctionContext) string {
	if t := context.Target; t != nil {
		return t.PathString()
	}
	return context.Path
}

func assertClassifyAnswers(s *classifySetup, resp Response, path string) []types.RuleFunctionResult {
	var results []types.RuleFunctionResult
	for id, assertion := range s.Assert {
		ans, ok := resp.Answers[id]
		if !ok || !assertion.tripped(ans) {
			continue
		}
		results = append(results, types.RuleFunctionResult{
			Message: assertion.renderMessage(id, ans),
			Path:    resultPath(path),
		})
	}
	return results
}

func buildClassifySetup(functionOptions map[string]interface{}) (*classifySetup, error) {
	raw, err := json.Marshal(functionOptions)
	if err != nil {
		return nil, fmt.Errorf("encode functionOptions: %w", err)
	}
	var opts classifyOptions
	if err := json.Unmarshal(raw, &opts); err != nil {
		return nil, fmt.Errorf("decode functionOptions: %w", err)
	}
	if len(opts.Questions) == 0 {
		return nil, fmt.Errorf("classify requires at least one question in functionOptions.questions")
	}
	for id, q := range opts.Questions {
		switch q.Type {
		case QuestionChoice, QuestionScore, QuestionNoul:
		default:
			return nil, fmt.Errorf("question %q has unknown type %q (want choice, score, noul)", id, q.Type)
		}
	}

	key := opts.APIKey
	if key == "" {
		key = os.Getenv(JevAPIKeyEnv)
	}
	return &classifySetup{
		backend:         &JevBackend{APIKey: key, BaseURL: opts.Endpoint, Model: opts.Model},
		classifyOptions: opts,
	}, nil
}

func classifyState(value interface{}, field string) interface{} {
	if field == "" {
		return value
	}
	if m, ok := value.(map[string]interface{}); ok {
		v, ok := m[field]
		if !ok {
			return nil
		}
		return v
	}
	return nil
}

func (r *ClassifyRule) classifyCached(s *classifySetup, state interface{}) (Response, error) {
	key, err := classifyCacheKey(s.Model, state, s.Questions)
	if err != nil {
		return Response{}, err
	}
	if cached, ok := r.cache[key]; ok {
		return cached, nil
	}
	req := Request{State: state, Model: s.Model, Questions: s.Questions}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	resp, err := s.backend.Classify(ctx, req)
	if err != nil {
		return Response{}, err
	}
	if r.cache == nil {
		r.cache = make(map[string]Response)
	}
	r.cache[key] = resp
	return resp, nil
}

func classifyCacheKey(model string, state interface{}, questions map[string]Question) (string, error) {
	canonical, err := json.Marshal(map[string]interface{}{
		"model":     model,
		"state":     state,
		"questions": questions,
	})
	if err != nil {
		return "", fmt.Errorf("encode cache key: %w", err)
	}
	return string(canonical), nil
}

func (a classifyAssertion) tripped(ans Answer) bool {
	if v, ok := numericAnswer(ans); ok {
		return a.trippedNumeric(v)
	}
	return a.trippedChoice(ans)
}

func (a classifyAssertion) trippedNumeric(v float64) bool {
	return (a.Below != nil && v < *a.Below) || (a.Above != nil && v > *a.Above)
}

func (a classifyAssertion) trippedChoice(ans Answer) bool {
	if ans.Type != QuestionChoice {
		return false
	}
	if a.Is != "" && ans.Choice == a.Is {
		return true
	}
	if slices.Contains(a.In, ans.Choice) {
		return true
	}
	return len(a.NotIn) > 0 && !slices.Contains(a.NotIn, ans.Choice)
}

func numericAnswer(ans Answer) (float64, bool) {
	switch ans.Type {
	case QuestionNoul:
		return f64p(ans.Noul)
	case QuestionScore:
		return f64p(ans.Score)
	}
	return 0, false
}

func (a classifyAssertion) renderMessage(id string, ans Answer) string {
	if strings.TrimSpace(a.Message) == "" {
		return fmt.Sprintf("classify: question %q flagged (%s)", id, shortAnswer(ans))
	}
	tmpl, err := template.New("message").Parse(a.Message)
	if err != nil {
		return a.Message
	}
	data := map[string]any{
		"Choice":     ans.Choice,
		"Score":      f64(ans.Score),
		"Noul":       f64(ans.Noul),
		"Confidence": f64(ans.Confidence),
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return a.Message
	}
	return buf.String()
}

func shortAnswer(ans Answer) string {
	switch ans.Type {
	case QuestionChoice:
		return fmt.Sprintf("choice=%q", ans.Choice)
	case QuestionScore:
		return fmt.Sprintf("score=%v", f64(ans.Score))
	case QuestionNoul:
		return fmt.Sprintf("noul=%v", f64(ans.Noul))
	default:
		return ans.Type
	}
}

func f64(p *float64) float64 {
	if p == nil {
		return 0
	}
	return *p
}

func f64p(p *float64) (float64, bool) {
	if p == nil {
		return 0, false
	}
	return *p, true
}
