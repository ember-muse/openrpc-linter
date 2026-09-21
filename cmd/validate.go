package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/open-rpc/openrpc-linter/metaschema"
	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/spf13/cobra"
)

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

	openrpc, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("Error reading %s: %w", filename, err)
	}

	data, err := jsonschema.UnmarshalJSON(strings.NewReader(string(openrpc)))
	if err != nil {
		return fmt.Errorf("Error parsing JSON: %w", err)
	}

	meta, err := metaschema.For(data)
	if err != nil {
		return err
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Validating OpenRPC document: %s (OpenRPC %s)\n", filename, meta.Version)

	schema, err := meta.Compile()
	if err != nil {
		return fmt.Errorf("Error compiling schema: %w", err)
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
