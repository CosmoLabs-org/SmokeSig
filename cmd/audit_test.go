package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/CosmoLabs-org/SmokeSig/internal/audit"
	"github.com/CosmoLabs-org/SmokeSig/internal/detector"
)

func TestAuditCmd_NoConfig(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(dir)

	// Reset flags for test isolation.
	auditJSON = false
	auditFix = false

	err := runAudit(auditCmd, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAuditCmd_WithConfig(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(dir)

	cfg := `
version: 1
project: test-project
tests:
  - name: Build
    run: echo build
    expect:
      exit_code: 0
`
	if err := os.WriteFile(filepath.Join(dir, ".smokesig.yaml"), []byte(cfg), 0644); err != nil {
		t.Fatal(err)
	}

	auditJSON = false
	auditFix = false

	err := runAudit(auditCmd, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAuditCmd_JSONOutput(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(dir)

	cfg := `
version: 1
project: test-project
tests:
  - name: Build
    run: echo build
    expect:
      exit_code: 0
`
	if err := os.WriteFile(filepath.Join(dir, ".smokesig.yaml"), []byte(cfg), 0644); err != nil {
		t.Fatal(err)
	}

	auditJSON = true
	auditFix = false

	err := runAudit(auditCmd, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	auditJSON = false // reset
}

func TestAuditCmd_FixMode(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(dir)

	// Create a Go project with minimal config missing build test reference.
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/test\ngo 1.21\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".env.example"), []byte("FOO=bar\n"), 0644); err != nil {
		t.Fatal(err)
	}
	cfg := `
version: 1
project: test-project
tests:
  - name: Runs
    run: echo hello
    expect:
      stdout_contains: hello
`
	if err := os.WriteFile(filepath.Join(dir, ".smokesig.yaml"), []byte(cfg), 0644); err != nil {
		t.Fatal(err)
	}

	auditJSON = false
	auditFix = true

	err := runAudit(auditCmd, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify the config was updated with additional tests.
	data, err := os.ReadFile(filepath.Join(dir, ".smokesig.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if len(content) <= len(cfg) {
		t.Error("expected config to grow after --fix")
	}

	auditFix = false // reset
}

func TestGenerateFixTests(t *testing.T) {
	dir := t.TempDir()

	tests := []struct {
		name       string
		rec        audit.Recommendation
		types      []detector.ProjectType
		wantCount  int
		wantNames  []string
	}{
		{
			name: "single missing assertion doc_integrity",
			rec: audit.Recommendation{
				Type:     "missing_assertion",
				Severity: audit.SeverityWarning,
				Message:  "Add doc_integrity assertion — Go CLI project with cmd/ directory",
			},
			types:     []detector.ProjectType{detector.Go},
			wantCount: 1,
			wantNames: []string{"Documentation sync"},
		},
		{
			name: "single missing assertion env_exists",
			rec: audit.Recommendation{
				Type:     "missing_assertion",
				Severity: audit.SeverityWarning,
				Message:  "Add env_exists assertions — .env.example found but no env checks configured",
			},
			types:     []detector.ProjectType{detector.Go},
			wantCount: 1,
			wantNames: []string{"Required env vars"},
		},
		{
			name: "single missing assertion docker_image_exists",
			rec: audit.Recommendation{
				Type:     "missing_assertion",
				Severity: audit.SeverityInfo,
				Message:  "Add docker_image_exists assertion — Dockerfile found in project",
			},
			types:     []detector.ProjectType{detector.Docker},
			wantCount: 1,
			wantNames: []string{"Docker image builds"},
		},
		{
			name: "single missing assertion docker_container_running",
			rec: audit.Recommendation{
				Type:     "missing_assertion",
				Severity: audit.SeverityWarning,
				Message:  "Add docker_container_running or docker_image_exists assertion — Docker project detected",
			},
			types:     []detector.ProjectType{detector.Docker},
			wantCount: 1,
			wantNames: []string{"Docker image builds"},
		},
		{
			name: "single missing assertion http",
			rec: audit.Recommendation{
				Type:     "missing_assertion",
				Severity: audit.SeverityWarning,
				Message:  "Add http assertion — HTTP server detected in Go source",
			},
			types:     []detector.ProjectType{detector.Go},
			wantCount: 1,
			wantNames: []string{"HTTP health check"},
		},
		{
			name: "single missing assertion docker_compose_healthy",
			rec: audit.Recommendation{
				Type:     "missing_assertion",
				Severity: audit.SeverityInfo,
				Message:  "Add docker_compose_healthy assertion — docker-compose.yml found",
			},
			types:     []detector.ProjectType{detector.Docker},
			wantCount: 1,
			wantNames: []string{"Docker Compose services healthy"},
		},
		{
			name: "missing baseline build test for Go",
			rec: audit.Recommendation{
				Type:     "missing_baseline",
				Severity: audit.SeverityWarning,
				Message:  "Add a build test — no test with build/compile in name or command",
			},
			types:     []detector.ProjectType{detector.Go},
			wantCount: 1,
			wantNames: []string{"Build"},
		},
		{
			name: "missing baseline build test for Node",
			rec: audit.Recommendation{
				Type:     "missing_baseline",
				Severity: audit.SeverityWarning,
				Message:  "Add a build test — no test with build/compile in name or command",
			},
			types:     []detector.ProjectType{detector.Node},
			wantCount: 1,
			wantNames: []string{"Build"},
		},
		{
			name: "missing baseline build test for Rust",
			rec: audit.Recommendation{
				Type:     "missing_baseline",
				Severity: audit.SeverityWarning,
				Message:  "Add a build test — no test with build/compile in name or command",
			},
			types:     []detector.ProjectType{detector.Rust},
			wantCount: 1,
			wantNames: []string{"Build"},
		},
		{
			name: "missing baseline build test for Python",
			rec: audit.Recommendation{
				Type:     "missing_baseline",
				Severity: audit.SeverityWarning,
				Message:  "Add a build test — no test with build/compile in name or command",
			},
			types:     []detector.ProjectType{detector.Python},
			wantCount: 1,
			wantNames: []string{"Syntax check"},
		},
		{
			name: "unhandled recommendation type returns nil",
			rec: audit.Recommendation{
				Type:     "stale_config",
				Severity: audit.SeverityWarning,
				Message:  "Legacy .smoke.yaml found — rename to .smokesig.yaml",
			},
			types:     nil,
			wantCount: 0,
			wantNames: nil,
		},
		{
			name: "unhandled message returns nil",
			rec: audit.Recommendation{
				Type:     "missing_assertion",
				Severity: audit.SeverityInfo,
				Message:  "Consider adding k8s_resource assertion — Helm chart project detected",
			},
			types:     []detector.ProjectType{detector.Helm},
			wantCount: 0,
			wantNames: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := generateFixTests(tt.rec, dir, tt.types)
			if len(got) != tt.wantCount {
				t.Fatalf("expected %d tests, got %d", tt.wantCount, len(got))
			}
			if tt.wantNames != nil {
				for i, wantName := range tt.wantNames {
					if i >= len(got) {
						break
					}
					if got[i].Name != wantName {
						t.Errorf("test[%d]: expected name %q, got %q", i, wantName, got[i].Name)
					}
				}
			}
		})
	}
}

func TestGenerateFixTests_EmptyFindings(t *testing.T) {
	dir := t.TempDir()
	// Calling with a recommendation that has no matching fix handler.
	rec := audit.Recommendation{
		Type:    "missing_assertion",
		Message: "Consider adding deep_link assertion — React Native project detected",
	}
	got := generateFixTests(rec, dir, []detector.ProjectType{detector.ReactNative})
	if got != nil {
		t.Fatalf("expected nil for unhandled message, got %d tests", len(got))
	}
}

func TestGenerateFixTests_MultipleTypes(t *testing.T) {
	dir := t.TempDir()
	// Simulate calling generateFixTests for multiple distinct recommendations.
	recs := []audit.Recommendation{
		{Type: "missing_assertion", Message: "Add env_exists assertions — .env.example found"},
		{Type: "missing_assertion", Message: "Add http assertion — HTTP server detected"},
		{Type: "missing_baseline", Message: "Add a build test — no test with build/compile in name"},
	}

	totalTests := 0
	for _, rec := range recs {
		tests := generateFixTests(rec, dir, []detector.ProjectType{detector.Go})
		totalTests += len(tests)
	}

	if totalTests != 3 {
		t.Fatalf("expected 3 total tests from 3 findings, got %d", totalTests)
	}
}

// ---------------------------------------------------------------------------
// applyFixes tests
// ---------------------------------------------------------------------------

// TestApplyFixes_NoConfig returns 0 when report says config doesn't exist.
func TestApplyFixes_NoConfig(t *testing.T) {
	dir := t.TempDir()
	report := &audit.Report{ConfigExists: false}
	n, err := applyFixes(dir, filepath.Join(dir, ".smokesig.yaml"), report)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 0 {
		t.Errorf("expected 0 fixes, got %d", n)
	}
}

// TestApplyFixes_NoRecommendations returns 0 when there are no fixable recommendations.
func TestApplyFixes_NoRecommendations(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, ".smokesig.yaml")
	cfg := `version: 1
project: fix-test
tests:
  - name: basic
    run: echo ok
    expect:
      exit_code: 0
`
	if err := os.WriteFile(cfgPath, []byte(cfg), 0644); err != nil {
		t.Fatal(err)
	}
	report := &audit.Report{
		ConfigExists:    true,
		Recommendations: []audit.Recommendation{
			{Type: "other", Message: "not a fixable type"},
		},
	}
	n, err := applyFixes(dir, cfgPath, report)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 0 {
		t.Errorf("expected 0 fixes for non-fixable recs, got %d", n)
	}
}

// TestApplyFixes_WithMissingAssertion applies a missing_assertion fix and writes updated config.
func TestApplyFixes_WithMissingAssertion(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, ".smokesig.yaml")
	cfg := `version: 1
project: fix-test
tests:
  - name: basic
    run: echo ok
    expect:
      exit_code: 0
`
	if err := os.WriteFile(cfgPath, []byte(cfg), 0644); err != nil {
		t.Fatal(err)
	}
	report := &audit.Report{
		ConfigExists: true,
		Recommendations: []audit.Recommendation{
			{Type: "missing_assertion", Message: "Add env_exists assertions — .env.example found"},
		},
	}
	n, err := applyFixes(dir, cfgPath, report)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n == 0 {
		t.Error("expected at least 1 fix applied, got 0")
	}
	// Verify the file was rewritten.
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) == 0 {
		t.Error("config file should not be empty after fix")
	}
}

// TestApplyFixes_InvalidConfig returns an error when the config file is malformed.
func TestApplyFixes_InvalidConfig(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, ".smokesig.yaml")
	if err := os.WriteFile(cfgPath, []byte("{{not valid yaml"), 0644); err != nil {
		t.Fatal(err)
	}
	report := &audit.Report{ConfigExists: true}
	_, err := applyFixes(dir, cfgPath, report)
	if err == nil {
		t.Fatal("expected error for invalid config, got nil")
	}
}

