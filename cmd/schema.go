package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/CosmoLabs-org/SmokeSig/internal/schema"
	"github.com/spf13/cobra"
)

var schemaCmd = &cobra.Command{
	Use:   "schema",
	Short: "Export config schema as JSON",
	Long:  "Export the assertion type schema (all types, fields, required flags) as structured JSON. Useful for editor integrations and tooling. When a config file is available, includes registered plugin metadata.",
	RunE: func(cmd *cobra.Command, args []string) error {
		file, _ := cmd.Flags().GetString("file")

		// Try to load config for plugin metadata
		var out *schema.SchemaOutput
		if file != "" {
			cfg, err := schema.Load(file)
			if err == nil && len(cfg.Plugins) > 0 {
				out = schema.ExportSchemaWithPlugins(cfg.Plugins)
			}
		} else {
			// Try default config path
			path, err := schema.LoadDefaultPath()
			if err == nil {
				cfg, err := schema.Load(path)
				if err == nil && len(cfg.Plugins) > 0 {
					out = schema.ExportSchemaWithPlugins(cfg.Plugins)
				}
			}
		}

		if out == nil {
			out = schema.ExportSchema()
		}

		data, err := json.MarshalIndent(out, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
	},
}

func init() {
	rootCmd.AddCommand(schemaCmd)
	schemaCmd.Flags().StringP("file", "f", "", "Config file path for plugin metadata (optional)")
}
