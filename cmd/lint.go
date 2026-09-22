package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/open-rpc/openrpc-linter/location"
	"github.com/open-rpc/openrpc-linter/metaschema"
	"github.com/open-rpc/openrpc-linter/reporters"
	"github.com/open-rpc/openrpc-linter/rules"
	"github.com/open-rpc/openrpc-linter/selector"
	"github.com/open-rpc/openrpc-linter/types"

	"github.com/spf13/cobra"
)

var (
	rulesFile    string
	outputFormat string
)

type LintOptions struct {
	OpenRPCFile string
	RulesFile   string
	Output      io.Writer
	Format      string
}

func GetReporter(format string) reporters.Reporter {
	switch format {
	case "json":
		return &reporters.JSONReporter{}
	case "text":
		return &reporters.TextReporter{}
	default:
		return &reporters.TextReporter{}
	}
}

func normalizeSeverity(severity types.Severity) (types.Severity, error) {
	if severity == "" {
		return types.SeverityError, nil
	}

	switch types.Severity(strings.ToLower(string(severity))) {
	case types.SeverityIgnore:
		return types.SeverityIgnore, nil
	case types.SeverityError:
		return types.SeverityError, nil
	case types.SeverityWarn:
		return types.SeverityWarn, nil
	case types.SeverityInfo:
		return types.SeverityInfo, nil
	default:
		return "", fmt.Errorf("invalid severity %q; expected one of: error, warn, info, ignore", severity)
	}
}

func RunLint(opts LintOptions) error {
	openrpcData, err := os.ReadFile(opts.OpenRPCFile)
	if err != nil {
		fmt.Fprintf(opts.Output, "Error reading OpenRPC file: %v\n", err)
		return err
	}

	var openrpcDoc interface{}
	err = json.Unmarshal(openrpcData, &openrpcDoc)
	if err != nil {
		fmt.Fprintf(opts.Output, "Error parsing OpenRPC file: %v\n", err)
		return err
	}

	// Resolve all $ref references in the document
	resolvedDoc, err := resolveRefs(openrpcDoc)
	if err != nil {
		fmt.Fprintf(opts.Output, "Error resolving $refs in OpenRPC file: %v\n", err)
		return err
	}

	// Build the schema-aware index once per lint run. The selector and
	// every rule function consume Targets derived from this index;
	// rebuilding per-rule would be wasteful and would lose the cache.
	meta, err := metaschema.For(openrpcDoc)
	if err != nil {
		fmt.Fprintf(opts.Output, "Error selecting OpenRPC meta-schema: %v\n", err)
		return err
	}
	index := selector.Build(resolvedDoc, meta)

	rulesWrapper, err := rules.LoadRulesFileFromPath(opts.RulesFile)
	if err != nil {
		return err
	}

	err = rulesWrapper.CheckRules()
	if err != nil {
		fmt.Fprintf(opts.Output, "Error checking rules file: %v\n", err)
		return err
	}
	rulesWrapper.Rules, err = rulesWrapper.ResolvedRules()

	if err != nil {
		return err
	}

	var allResults []types.RuleFunctionResult
	totalRules := len(rulesWrapper.Rules)
	errorCount := 0

	for ruleId, rule := range rulesWrapper.Rules {
		normalizedSeverity, err := normalizeSeverity(rule.Severity)
		if err != nil {
			fmt.Fprintf(opts.Output, "Error validating rules file: rule %q %v\n", ruleId, err)
			return err
		}
		if normalizedSeverity == types.SeverityIgnore {
			continue
		}
		rule.Severity = normalizedSeverity

		context := types.RuleFunctionContext{
			Rule:             &rule,
			RuleID:           ruleId,
			Document:         openrpcDoc,
			ResolvedDocument: resolvedDoc,
			Index:            index,
		}
		results, err := rules.ExecuteRule(&rule, context)

		if err != nil {
			allResults = append(allResults, types.RuleFunctionResult{
				RuleID:   ruleId,
				Message:  err.Error(),
				Severity: types.SeverityError,
			})
			errorCount++
			continue
		}

		ruleViolationCount := 0
		for i := range results {
			if results[i].RuleID == "" {
				results[i].RuleID = ruleId
			}
			if results[i].Message != "" {
				results[i].Severity = normalizedSeverity
				ruleViolationCount++
			}
		}

		if normalizedSeverity == types.SeverityError {
			errorCount += ruleViolationCount
		}

		allResults = append(allResults, results...)
	}

	location.Enrich(allResults, resolvedDoc)

	reporter := GetReporter(opts.Format)
	if tr, ok := reporter.(*reporters.TextReporter); ok {
		tr.SourceFile = opts.OpenRPCFile
	}
	if err := reporter.Format(allResults, totalRules, opts.Output); err != nil {
		return err
	}

	if errorCount > 0 {
		return fmt.Errorf("found %d linting error(s)", errorCount)
	}

	return nil
}

var lintCmd = &cobra.Command{
	Use:   "lint [openrpc-file]",
	Short: "Lint an OpenRPC document",
	Long:  "Lint an OpenRPC document for compliance with OpenRPC specification",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		openrpcFile := "openrpc.json"
		if len(args) > 0 {
			openrpcFile = args[0]
		}

		opts := LintOptions{
			OpenRPCFile: openrpcFile,
			RulesFile:   rulesFile,
			Output:      cmd.OutOrStdout(),
			Format:      outputFormat,
		}

		if err := RunLint(opts); err != nil {
			os.Exit(1)
		}
	},
}

func init() {
	lintCmd.Flags().StringVarP(&rulesFile, "rules", "r", "", "Path to rules YAML file")
	lintCmd.Flags().StringVarP(&outputFormat, "format", "f", "text", "Output format (text, json)")
	rootCmd.AddCommand(lintCmd)
}

// resolveRefs resolves all $ref references in the document
func resolveRefs(document interface{}) (interface{}, error) {
	// For now, we'll implement a basic $ref resolver that handles internal references
	// This is a simplified implementation that can be enhanced later

	docBytes, err := json.Marshal(document)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal document: %w", err)
	}

	var resolved interface{}
	err = json.Unmarshal(docBytes, &resolved)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal document: %w", err)
	}

	// Recursively resolve $refs within the document
	resolved = resolveRefsRecursive(resolved, document, types.ResolvingRefs{})

	return resolved, nil
}

// resolveRefsRecursive recursively resolves $ref references in the document
func resolveRefsRecursive(current interface{}, root interface{}, resolving types.ResolvingRefs) interface{} {
	switch v := current.(type) {
	case map[string]interface{}:
		// Check if this is a $ref
		if ref, exists := v["$ref"]; exists {
			if refStr, ok := ref.(string); ok {
				// Handle internal refs (starting with #)
				if strings.HasPrefix(refStr, "#/") {
					if resolving[refStr] {
						return v
					}
					resolved := resolveJSONPointer(refStr[2:], root) // Remove the "#/" prefix
					if resolved != nil {
						resolving[refStr] = true
						result := resolveRefsRecursive(resolved, root, resolving)
						delete(resolving, refStr)
						return result
					}
				}
			}
			// If we can't resolve the ref, return the original $ref
			return v
		}

		// Recursively process all values in the map
		result := make(map[string]interface{})
		for key, value := range v {
			result[key] = resolveRefsRecursive(value, root, resolving)
		}
		return result

	case []interface{}:
		// Recursively process all items in the array
		result := make([]interface{}, len(v))
		for i, item := range v {
			result[i] = resolveRefsRecursive(item, root, resolving)
		}
		return result

	default:
		// For primitive types, return as-is
		return v
	}
}

// resolveJSONPointer resolves a JSON pointer path in the document
func resolveJSONPointer(path string, document interface{}) interface{} {
	if path == "" {
		return document
	}

	parts := strings.Split(path, "/")
	current := document

	for _, part := range parts {
		// Unescape JSON pointer characters
		part = strings.ReplaceAll(part, "~1", "/")
		part = strings.ReplaceAll(part, "~0", "~")

		switch v := current.(type) {
		case map[string]interface{}:
			if val, exists := v[part]; exists {
				current = val
			} else {
				return nil // Path not found
			}
		case []interface{}:
			// Handle array indices (not commonly used in OpenRPC, but for completeness)
			return nil
		default:
			return nil // Can't traverse further
		}
	}

	return current
}
