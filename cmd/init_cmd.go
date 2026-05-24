package cmd

import (
	"fmt"
	"os"

	"github.com/CosmoLabs-org/SmokeSig/internal/detector"
	"github.com/CosmoLabs-org/SmokeSig/internal/schema"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Generate .smokesig.yaml for this project",
	Long:  "Auto-detect project type and generate a .smokesig.yaml configuration",
	RunE:  runInit,
}

var (
	forceOverwrite   bool
	fromRunning      string
	withDocIntegrity bool
)

func init() {
	rootCmd.AddCommand(initCmd)
	initCmd.Flags().BoolVarP(&forceOverwrite, "force", "f", false, "Overwrite existing .smokesig.yaml")
	initCmd.Flags().StringVar(&fromRunning, "from-running", "", "Generate config by inspecting a running Docker container")
	initCmd.Flags().BoolVar(&withDocIntegrity, "with-doc-integrity", false, "Include doc_integrity test even if CLI auto-detection does not trigger")
}

func runInit(cmd *cobra.Command, args []string) error {
	if _, err := os.Stat(".smokesig.yaml"); err == nil && !forceOverwrite {
		return fmt.Errorf(".smokesig.yaml already exists (use --force to overwrite)")
	}

	var cfg *schema.SmokeConfig

	if fromRunning != "" {
		// Inspect running container
		fmt.Printf("Inspecting container: %s\n", fromRunning)
		var err error
		cfg, err = detector.InspectContainer(fromRunning)
		if err != nil {
			return fmt.Errorf("inspecting container: %w", err)
		}
		fmt.Printf("Found: %d ports, %d processes\n", len(cfg.Tests), countProcessTests(cfg))
	} else {
		// Auto-detect from filesystem
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("getting working directory: %w", err)
		}

		types := detector.Detect(cwd)
		if len(types) == 0 {
			fmt.Println("No project type detected. Creating a minimal .smokesig.yaml")
		} else {
			names := make([]string, len(types))
			for i, t := range types {
				names[i] = string(t)
			}
			fmt.Printf("Detected: %v\n", names)
		}

		opts := detector.ConfigOptions{
			WithDocIntegrity: withDocIntegrity,
		}
		cfg = detector.GenerateConfigWithOptions(cwd, types, opts)
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}

	if err := os.WriteFile(".smokesig.yaml", data, 0644); err != nil {
		return fmt.Errorf("writing .smokesig.yaml: %w", err)
	}

	fmt.Println("Created .smokesig.yaml")
	return nil
}

func countProcessTests(cfg *schema.SmokeConfig) int {
	count := 0
	for _, t := range cfg.Tests {
		if t.Expect.PortListening != nil {
			count++
		}
	}
	return count
}
