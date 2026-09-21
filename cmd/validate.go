package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/spf13/cobra"
)

type HttpLoader struct {
	client *http.Client
}

func (l *HttpLoader) Load(url string) (any, error) {
	response, err := l.client.Get(url)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}

	var data any
	err = json.Unmarshal(body, &data)
	if err != nil {
		return nil, err
	}

	if dataMap, ok := data.(map[string]any); ok {
		if schema, exists := dataMap["$schema"]; exists {
			if schema == "https://meta.json-schema.tools/" {
				dataMap["$schema"] = "http://json-schema.org/draft-07/schema#"
			}
		}
	}

	return data, nil
}

func fetchOpenRPCSchema() (string, error) {
	schemaURL := "https://meta.open-rpc.org"
	response, err := http.Get(schemaURL)
	if err != nil {
		return "", fmt.Errorf("cannot load OpenRPC schema: %w", err)
	}
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return "", fmt.Errorf("cannot read OpenRPC schema: %w", err)
	}
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("cannot load OpenRPC schema: %s", response.Status)
	}
	return string(body), nil
}

var validateCmd = &cobra.Command{
	Use:   "validate [file]",
	Short: "Validate an OpenRPC document",
	Long:  "Validate an OpenRPC document against basic OpenRPC specification requirements. Defaults to 'openrpc.json' if no file is specified.",
	RunE:  runValidate,
}

func runValidate(cmd *cobra.Command, args []string) error {
	filename := "openrpc.json"
	if len(args) > 0 {
		filename = args[0]
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Validating OpenRPC document: %s\n", filename)

	schemaJSON, err := fetchOpenRPCSchema()
	if err != nil {
		return err
	}

	compiler := jsonschema.NewCompiler()

	schemaData, err := jsonschema.UnmarshalJSON(strings.NewReader(schemaJSON))
	if err != nil {
		return fmt.Errorf("Error parsing schema JSON: %w", err)
	}
	compiler.UseLoader(&HttpLoader{client: &http.Client{}})

	err = compiler.AddResource("schema.json", schemaData)
	if err != nil {
		return fmt.Errorf("Error adding schema: %w", err)
	}

	schema, err := compiler.Compile("schema.json")
	if err != nil {
		return fmt.Errorf("Error compiling schema: %w", err)
	}

	openrpc, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("Error reading %s: %w", filename, err)
	}

	data, err := jsonschema.UnmarshalJSON(strings.NewReader(string(openrpc)))
	if err != nil {
		return fmt.Errorf("Error parsing JSON: %w", err)
	}

	err = schema.Validate(data)
	if err != nil {
		return fmt.Errorf("❌ Validation failed: %w", err)
	}

	_, err = fmt.Fprintln(cmd.OutOrStdout(), "✅ OpenRPC document is valid!")
	return err
}

func init() {
	rootCmd.AddCommand(validateCmd)
}
