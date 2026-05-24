package audit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/CosmoLabs-org/SmokeSig/internal/schema"
)

func TestRun_NoConfig(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, ".smokesig.yaml")

	report, err := Run(dir, configPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if report.ConfigExists {
		t.Error("expected ConfigExists=false")
	}
	if report.Score != 0 {
		t.Errorf("expected score 0 for missing config, got %d", report.Score)
	}
	if len(report.Recommendations) == 0 {
		t.Error("expected at least one recommendation for missing config")
	}
	found := false
	for _, r := range report.Recommendations {
		if r.Type == "missing_config" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected missing_config recommendation")
	}
}

func TestRun_CompleteConfig(t *testing.T) {
	dir := t.TempDir()

	// Create a Go project with a complete config.
	writeFile(t, dir, "go.mod", "module example.com/test\ngo 1.21\n")
	writeFile(t, dir, ".smokesig.yaml", `
version: 1
project: test-project
tests:
  - name: Build
    run: go build ./...
    expect:
      exit_code: 0
  - name: Tests pass
    run: go test ./...
    expect:
      exit_code: 0
`)

	configPath := filepath.Join(dir, ".smokesig.yaml")
	report, err := Run(dir, configPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !report.ConfigExists {
		t.Error("expected ConfigExists=true")
	}
	if report.TestCount != 2 {
		t.Errorf("expected 2 tests, got %d", report.TestCount)
	}
	if report.Score < 5 {
		t.Errorf("expected score >= 5 for reasonable config, got %d", report.Score)
	}
	if report.ProjectType == "" || report.ProjectType == "unknown" {
		t.Error("expected project type to be detected")
	}
}

func TestRun_GoCLI_MissingDocIntegrity(t *testing.T) {
	dir := t.TempDir()

	// Create a Go CLI project (has cmd/ directory) without doc_integrity.
	writeFile(t, dir, "go.mod", "module example.com/test\ngo 1.21\n")
	if err := os.MkdirAll(filepath.Join(dir, "cmd"), 0755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, dir, ".smokesig.yaml", `
version: 1
project: test-project
tests:
  - name: Build
    run: go build ./...
    expect:
      exit_code: 0
`)

	configPath := filepath.Join(dir, ".smokesig.yaml")
	report, err := Run(dir, configPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	found := false
	for _, r := range report.Recommendations {
		if r.Type == "missing_assertion" && strings.Contains(r.Message, "doc_integrity") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected doc_integrity recommendation for Go CLI project")
	}
}

func TestRun_DockerProject_MissingAssertions(t *testing.T) {
	dir := t.TempDir()

	writeFile(t, dir, "Dockerfile", "FROM alpine\n")
	writeFile(t, dir, ".smokesig.yaml", `
version: 1
project: test-project
tests:
  - name: Build image
    run: docker build .
    expect:
      exit_code: 0
`)

	configPath := filepath.Join(dir, ".smokesig.yaml")
	report, err := Run(dir, configPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	foundDocker := false
	for _, r := range report.Recommendations {
		if r.Type == "missing_assertion" &&
			(strings.Contains(r.Message, "docker_container_running") || strings.Contains(r.Message, "docker_image_exists")) {
			foundDocker = true
			break
		}
	}
	if !foundDocker {
		t.Error("expected docker assertion recommendation")
	}
}

func TestRun_EnvExample_MissingEnvExists(t *testing.T) {
	dir := t.TempDir()

	writeFile(t, dir, ".env.example", "DATABASE_URL=postgres://...\n")
	writeFile(t, dir, ".smokesig.yaml", `
version: 1
project: test-project
tests:
  - name: Runs
    run: echo ok
    expect:
      exit_code: 0
`)

	configPath := filepath.Join(dir, ".smokesig.yaml")
	report, err := Run(dir, configPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	found := false
	for _, r := range report.Recommendations {
		if strings.Contains(r.Message, "env_exists") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected env_exists recommendation when .env.example present")
	}
}

func TestRun_StaleConfig(t *testing.T) {
	dir := t.TempDir()

	// Both old and new config present.
	writeFile(t, dir, ".smoke.yaml", "version: 1\nproject: old\ntests:\n  - name: x\n    run: echo\n    expect:\n      exit_code: 0\n")
	writeFile(t, dir, ".smokesig.yaml", `
version: 1
project: test-project
tests:
  - name: Build
    run: echo build
    expect:
      exit_code: 0
`)

	configPath := filepath.Join(dir, ".smokesig.yaml")
	report, err := Run(dir, configPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	found := false
	for _, r := range report.Recommendations {
		if r.Type == "stale_config" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected stale_config recommendation when .smoke.yaml exists")
	}
}

func TestRun_MissingBuildTest(t *testing.T) {
	dir := t.TempDir()

	writeFile(t, dir, ".smokesig.yaml", `
version: 1
project: test-project
tests:
  - name: Runs
    run: echo ok
    expect:
      stdout_contains: ok
`)

	configPath := filepath.Join(dir, ".smokesig.yaml")
	report, err := Run(dir, configPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	foundBuild := false
	foundExit := false
	for _, r := range report.Recommendations {
		if strings.Contains(r.Message, "build test") {
			foundBuild = true
		}
		if strings.Contains(r.Message, "exit_code") {
			foundExit = true
		}
	}
	if !foundBuild {
		t.Error("expected missing build test recommendation")
	}
	if !foundExit {
		t.Error("expected missing exit_code recommendation")
	}
}

func TestScoreCalculation(t *testing.T) {
	tests := []struct {
		name     string
		report   Report
		expected int
	}{
		{
			name:     "no config",
			report:   Report{ConfigExists: false},
			expected: 0,
		},
		{
			name: "perfect config",
			report: Report{
				ConfigExists: true,
				TestCount:    5,
			},
			expected: 10,
		},
		{
			name: "two warnings",
			report: Report{
				ConfigExists: true,
				TestCount:    3,
				Recommendations: []Recommendation{
					{Severity: SeverityWarning},
					{Severity: SeverityWarning},
				},
			},
			expected: 6,
		},
		{
			name: "no tests",
			report: Report{
				ConfigExists: true,
				TestCount:    0,
			},
			expected: 7,
		},
		{
			name: "many warnings floor at zero",
			report: Report{
				ConfigExists: true,
				TestCount:    1,
				Recommendations: []Recommendation{
					{Severity: SeverityWarning},
					{Severity: SeverityWarning},
					{Severity: SeverityWarning},
					{Severity: SeverityWarning},
					{Severity: SeverityWarning},
					{Severity: SeverityWarning},
				},
			},
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := calculateScore(&tt.report)
			if got != tt.expected {
				t.Errorf("calculateScore() = %d, want %d", got, tt.expected)
			}
		})
	}
}

func TestFormatTerminal(t *testing.T) {
	report := &Report{
		ConfigExists:    true,
		ConfigPath:      "/tmp/.smokesig.yaml",
		ProjectType:     "go",
		TestCount:       6,
		AssertionsUsed:  4,
		TotalAssertions: 45,
		Score:           7,
		Recommendations: []Recommendation{
			{Message: "Add doc_integrity assertion"},
		},
		Passes: []string{"Build test present"},
	}

	out := FormatTerminal(report)
	if out == "" {
		t.Fatal("expected non-empty output")
	}

	checks := []string{
		"SmokeSig Audit Report",
		"go (detected)",
		".smokesig.yaml (found)",
		"6 defined",
		"4 of 45",
		"doc_integrity",
		"Build test present",
		"7/10",
	}
	for _, check := range checks {
		if !strings.Contains(out, check) {
			t.Errorf("output missing %q:\n%s", check, out)
		}
	}
}

func TestCollectUsedAssertions(t *testing.T) {
	exitZero := 0
	cfg := &schema.SmokeConfig{
		Tests: []schema.Test{
			{
				Expect: schema.Expect{
					ExitCode:       &exitZero,
					StdoutContains: "hello",
				},
			},
			{
				Expect: schema.Expect{
					HTTP: &schema.HTTPCheck{URL: "http://localhost"},
					DocIntegrity: &schema.DocIntegrityCheck{
						Binary: "test",
						Docs:   []string{"README.md"},
					},
				},
			},
		},
	}

	used := collectUsedAssertions(cfg)

	expected := []string{"exit_code", "stdout_contains", "http", "doc_integrity"}
	for _, name := range expected {
		if !used[name] {
			t.Errorf("expected %q to be collected as used", name)
		}
	}
	if used["redis_ping"] {
		t.Error("redis_ping should not be in used set")
	}
}

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}
