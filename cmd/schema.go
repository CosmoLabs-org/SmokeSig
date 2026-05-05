package cmd

import (
	"fmt"

	"github.com/CosmoLabs-org/SmokeSig/internal/schema"
	"github.com/spf13/cobra"
)

var schemaCmd = &cobra.Command{
	Use:   "schema",
	Short: "Export config schema as JSON",
	Long:  "Export the assertion type schema (all types, fields, required flags) as structured JSON. Useful for editor integrations and tooling.",
	RunE: func(cmd *cobra.Command, args []string) error {
		data, err := schema.ExportSchemaJSON()
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
	},
}

func init() {
	rootCmd.AddCommand(schemaCmd)
}
