package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/CosmoLabs-org/SmokeSig/internal/audit"
	"github.com/CosmoLabs-org/SmokeSig/internal/detector"
	"github.com/CosmoLabs-org/SmokeSig/internal/schema"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var auditCmd = &cobra.Command{
	Use:   "audit [-f path] [--json] [--fix]",
	Short: "Check if smoke test config is up to date",
	Long:  "Inspect the project and report what's missing or outdated in the smoke test config.",
	RunE:  runAudit,
}

var (
	auditJSON bool
	auditFix  bool
)

func init() {
	rootCmd.AddCommand(auditCmd)
	auditCmd.Flags().StringP("file", "f", ".smokesig.yaml", "Path to config file")
	auditCmd.Flags().BoolVar(&auditJSON, "json", false, "Output as JSON")
	auditCmd.Flags().BoolVar(&auditFix, "fix", false, "Auto-apply safe recommendations")
}

func runAudit(cmd *cobra.Command, args []string) error {
	configFile, _ := cmd.Flags().GetString("file")

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting working directory: %w", err)
	}

	// Resolve config path relative to cwd.
	if !filepath.IsAbs(configFile) {
		configFile = filepath.Join(cwd, configFile)
	}

	report, err := audit.Run(cwd, configFile)
	if err != nil {
		return fmt.Errorf("running audit: %w", err)
	}

	if auditFix && report.ConfigExists {
		applied, fixErr := applyFixes(cwd, configFile, report)
		if fixErr != nil {
			fmt.Fprintf(os.Stderr, "⚠ fix error: %v\n", fixErr)
		} else if applied > 0 {
			fmt.Fprintf(os.Stderr, "Applied %d fix(es). Re-running audit...\n", applied)
			// Re-run audit after fixes.
			report, err = audit.Run(cwd, configFile)
			if err != nil {
				return fmt.Errorf("re-running audit after fix: %w", err)
			}
		}
	}

	if auditJSON {
		data, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			return fmt.Errorf("marshaling report: %w", err)
		}
		fmt.Println(string(data))
	} else {
		fmt.Print(audit.FormatTerminal(report))
	}

	return nil
}

// applyFixes generates new test blocks for missing assertions and appends them
// to the config. Returns the number of fixes applied.
func applyFixes(dir, configPath string, report *audit.Report) (int, error) {
	if !report.ConfigExists {
		// No config — recommend smokesig init instead.
		return 0, nil
	}

	cfg, err := schema.Load(configPath)
	if err != nil {
		return 0, fmt.Errorf("loading config for fix: %w", err)
	}

	applied := 0
	types := detector.Detect(dir)

	for _, rec := range report.Recommendations {
		if rec.Type != "missing_assertion" && rec.Type != "missing_baseline" {
			continue
		}

		tests := generateFixTests(rec, dir, types)
		if len(tests) > 0 {
			cfg.Tests = append(cfg.Tests, tests...)
			applied += len(tests)
		}
	}

	if applied == 0 {
		return 0, nil
	}

	// Write updated config.
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return 0, fmt.Errorf("marshaling fixed config: %w", err)
	}
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return 0, fmt.Errorf("writing fixed config: %w", err)
	}

	return applied, nil
}

// generateFixTests creates test blocks for a specific recommendation.
func generateFixTests(rec audit.Recommendation, dir string, types []detector.ProjectType) []schema.Test {
	exitZero := intPtrAudit(0)

	switch {
	case containsStr(rec.Message, "doc_integrity"):
		return []schema.Test{{
			Name: "Documentation sync",
			Expect: schema.Expect{
				DocIntegrity: &schema.DocIntegrityCheck{
					Binary: filepath.Base(dir),
					Docs:   []string{"README.md"},
				},
			},
		}}

	case containsStr(rec.Message, "docker_image_exists"):
		return []schema.Test{{
			Name: "Docker image builds",
			Run:  "docker build -t " + filepath.Base(dir) + ":smoke .",
			Expect: schema.Expect{
				ExitCode: exitZero,
				DockerImage: &schema.DockerImageCheck{
					Image: filepath.Base(dir) + ":smoke",
				},
			},
		}}

	case containsStr(rec.Message, "docker_container_running"):
		return []schema.Test{{
			Name: "Docker image builds",
			Run:  "docker build -t " + filepath.Base(dir) + ":smoke .",
			Expect: schema.Expect{
				ExitCode: exitZero,
				DockerImage: &schema.DockerImageCheck{
					Image: filepath.Base(dir) + ":smoke",
				},
			},
		}}

	case containsStr(rec.Message, "env_exists"):
		return []schema.Test{{
			Name: "Required env vars",
			Run:  "true",
			Expect: schema.Expect{
				EnvExists: "PATH",
			},
		}}

	case containsStr(rec.Message, "http assertion"):
		return []schema.Test{{
			Name: "HTTP health check",
			Expect: schema.Expect{
				HTTP: &schema.HTTPCheck{
					URL:        "http://localhost:8080/",
					StatusCode: intPtrAudit(200),
				},
			},
		}}

	case containsStr(rec.Message, "build test"):
		for _, t := range types {
			switch t {
			case detector.Go:
				return []schema.Test{{
					Name:   "Build",
					Run:    "go build ./...",
					Expect: schema.Expect{ExitCode: exitZero},
				}}
			case detector.Node:
				pm := "npm"
				if detector.HasBun(dir) {
					pm = "bun"
				}
				return []schema.Test{{
					Name:   "Build",
					Run:    pm + " run build",
					Expect: schema.Expect{ExitCode: exitZero},
				}}
			case detector.Rust:
				return []schema.Test{{
					Name:   "Build",
					Run:    "cargo build",
					Expect: schema.Expect{ExitCode: exitZero},
				}}
			case detector.Python:
				return []schema.Test{{
					Name:   "Syntax check",
					Run:    "python -m py_compile .",
					Expect: schema.Expect{ExitCode: exitZero},
				}}
			}
		}

	case containsStr(rec.Message, "docker_compose_healthy"):
		return []schema.Test{{
			Name: "Docker Compose services healthy",
			Expect: schema.Expect{
				DockerCompose: &schema.DockerComposeCheck{},
			},
		}}
	}

	return nil
}

func intPtrAudit(n int) *int { return &n }

func containsStr(s, substr string) bool {
	return len(s) >= len(substr) && contains(s, substr)
}

func contains(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
