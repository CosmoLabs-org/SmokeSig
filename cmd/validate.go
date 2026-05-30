package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/CosmoLabs-org/SmokeSig/internal/plugin"
	"github.com/CosmoLabs-org/SmokeSig/internal/schema"
	"github.com/spf13/cobra"
)

var validateCmd = &cobra.Command{
	Use:   "validate [-f path]",
	Short: "Validate smoke test config without running tests",
	Long:  "Load and validate .smokesig.yaml configuration. Reports all errors at once.",
	RunE: func(cmd *cobra.Command, args []string) error {
		file, _ := cmd.Flags().GetString("file")
		checkPlugins, _ := cmd.Flags().GetBool("check-plugins")
		if file == "" {
			resolved, err := schema.LoadDefaultPath()
			if err != nil {
				return &ConfigError{Err: err}
			}
			file = resolved
		}
		out, err := runValidate(file)
		if err != nil {
			fmt.Fprint(os.Stderr, out)
			return &ConfigError{Err: err}
		}
		fmt.Fprint(os.Stdout, out)

		if checkPlugins {
			pluginOut, pluginErr := runCheckPlugins(file)
			fmt.Fprint(os.Stdout, pluginOut)
			if pluginErr != nil {
				return pluginErr
			}
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(validateCmd)
	validateCmd.Flags().StringP("file", "f", "", "Config file path (default: .smokesig.yaml, falls back to .smoke.yaml)")
	validateCmd.Flags().Bool("check-plugins", false, "Deep-validate .wasm plugin files (compile, probe exports, detect ABI)")
}

func runValidate(path string) (string, error) {
	cfg, err := schema.Load(path)
	if err != nil {
		return fmt.Sprintf("error: loading config: %v\n", err), err
	}

	if err := schema.Validate(cfg); err != nil {
		if ve, ok := err.(*schema.ValidationError); ok {
			var out string
			out += fmt.Sprintf("❌ %s: %d error(s)\n", path, len(ve.Errors))
			for _, e := range ve.Errors {
				out += fmt.Sprintf("  - %s\n", e)
			}
			return out, ve
		}
		return fmt.Sprintf("❌ %s: %v\n", path, err), err
	}

	return fmt.Sprintf("✅ %s: valid (%d tests)\n", path, len(cfg.Tests)), nil
}

func runCheckPlugins(path string) (string, error) {
	cfg, err := schema.Load(path)
	if err != nil {
		return "", err
	}

	if len(cfg.Plugins) == 0 {
		return "  (no plugins configured)\n", nil
	}

	configDir := filepath.Dir(path)
	ctx := context.Background()
	pm, err := plugin.NewPluginManager(ctx, plugin.ManagerOptions{
		ConfigDir: configDir,
	})
	if err != nil {
		return fmt.Sprintf("❌ plugin system init failed: %v\n", err), err
	}
	defer pm.Close(ctx)

	var out string
	var errCount int

	names := make([]string, 0, len(cfg.Plugins))
	for name := range cfg.Plugins {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		entry := cfg.Plugins[name]
		memoryMB := cfg.Settings.PluginMemoryMB
		if memoryMB <= 0 {
			memoryMB = plugin.DefaultMemoryLimitMB
		}
		if err := pm.LoadPlugin(ctx, name, entry, memoryMB); err != nil {
			out += fmt.Sprintf("  ❌ plugin %q: %v\n", name, err)
			errCount++
		} else {
			out += fmt.Sprintf("  ✅ plugin %q: OK\n", name)
		}
	}

	if errCount > 0 {
		return out, fmt.Errorf("%d plugin(s) failed deep validation", errCount)
	}
	return out, nil
}
