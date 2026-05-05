// Test helpers noopReporter and intPtr defined in runner_test.go (same package).

package runner

import (
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/CosmoLabs-org/SmokeSig/internal/monorepo"
	"github.com/CosmoLabs-org/SmokeSig/internal/schema"
)

// --- CheckPortListening extended tests ---

func TestCheckPortListening_OpenPort(t *testing.T) {
	// Start a TCP listener on a random port
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	port := ln.Addr().(*net.TCPAddr).Port
	result := CheckPortListening(port, "tcp", "127.0.0.1")
	if !result.Passed {
		t.Errorf("CheckPortListening(open port) = %+v, want passed", result)
	}
	if result.Type != "port_listening" {
		t.Errorf("Type = %q, want port_listening", result.Type)
	}
}

func TestCheckPortListening_ClosedPort(t *testing.T) {
	// Listen and immediately close to get an unused port
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()

	result := CheckPortListening(port, "tcp", "127.0.0.1")
	if result.Passed {
		t.Error("CheckPortListening(closed port) should not pass")
	}
	if result.Type != "port_listening" {
		t.Errorf("Type = %q, want port_listening", result.Type)
	}
}

func TestCheckPortListening_DefaultProtocolAndHost(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	port := ln.Addr().(*net.TCPAddr).Port
	// Empty protocol and host should default to "tcp" and "localhost"
	result := CheckPortListening(port, "", "")
	if !result.Passed {
		t.Errorf("CheckPortListening with defaults = %+v, want passed", result)
	}
}

// --- CheckProcessRunning tests ---

func TestCheckProcessRunning_EmptyNameExt(t *testing.T) {
	result := CheckProcessRunning("")
	if result.Passed {
		t.Error("empty name should not pass")
	}
	if result.Actual != "empty name" {
		t.Errorf("Actual = %q, want empty name", result.Actual)
	}
}

func TestCheckProcessRunning_NoMatch(t *testing.T) {
	result := CheckProcessRunning("totally_nonexistent_process_xyz_12345")
	if result.Passed {
		t.Error("nonexistent process should not pass")
	}
}

// --- RunMonorepo tests ---

func TestRunMonorepo_MultipleConfigs(t *testing.T) {
	dir := t.TempDir()

	// Create two sub-project configs
	svc1Dir := filepath.Join(dir, "svc1")
	svc2Dir := filepath.Join(dir, "svc2")
	os.MkdirAll(svc1Dir, 0755)
	os.MkdirAll(svc2Dir, 0755)

	svc1Config := `
version: 1
project: svc1
tests:
  - name: svc1-test
    run: echo hello-svc1
    expect:
      exit_code: 0
`
	svc2Config := `
version: 1
project: svc2
tests:
  - name: svc2-test
    run: echo hello-svc2
    expect:
      exit_code: 0
`
	os.WriteFile(filepath.Join(svc1Dir, ".smokesig.yaml"), []byte(svc1Config), 0644)
	os.WriteFile(filepath.Join(svc2Dir, ".smokesig.yaml"), []byte(svc2Config), 0644)

	cfg := &schema.SmokeConfig{Version: 1, Project: "monorepo"}
	r := &Runner{Config: cfg, Reporter: &noopReporter{}, ConfigDir: dir}

	subs := []monorepo.SubConfig{
		{Path: filepath.Join(svc1Dir, ".smokesig.yaml"), Dir: svc1Dir, Project: "svc1"},
		{Path: filepath.Join(svc2Dir, ".smokesig.yaml"), Dir: svc2Dir, Project: "svc2"},
	}

	result, err := r.RunMonorepo(RunOptions{}, subs)
	if err != nil {
		t.Fatalf("RunMonorepo: %v", err)
	}
	if result.Total != 2 {
		t.Errorf("Total = %d, want 2", result.Total)
	}
	if result.Passed != 2 {
		t.Errorf("Passed = %d, want 2", result.Passed)
	}
	if result.Duration == 0 {
		t.Error("Duration should be > 0")
	}
}

func TestRunMonorepo_BadConfig(t *testing.T) {
	dir := t.TempDir()
	badDir := filepath.Join(dir, "bad")
	os.MkdirAll(badDir, 0755)
	os.WriteFile(filepath.Join(badDir, ".smokesig.yaml"), []byte("not: valid\n[yaml"), 0644)

	cfg := &schema.SmokeConfig{Version: 1, Project: "test"}
	r := &Runner{Config: cfg, Reporter: &noopReporter{}, ConfigDir: dir}

	subs := []monorepo.SubConfig{
		{Path: filepath.Join(badDir, ".smokesig.yaml"), Dir: badDir, Project: "bad"},
	}

	_, err := r.RunMonorepo(RunOptions{}, subs)
	if err == nil {
		t.Fatal("expected error for bad config")
	}
}

func TestRunMonorepo_FailingTest(t *testing.T) {
	dir := t.TempDir()
	svcDir := filepath.Join(dir, "svc")
	os.MkdirAll(svcDir, 0755)

	config := `
version: 1
project: fail-svc
tests:
  - name: will-fail
    run: exit 1
    expect:
      exit_code: 0
`
	os.WriteFile(filepath.Join(svcDir, ".smokesig.yaml"), []byte(config), 0644)

	cfg := &schema.SmokeConfig{Version: 1, Project: "monorepo"}
	r := &Runner{Config: cfg, Reporter: &noopReporter{}, ConfigDir: dir}

	subs := []monorepo.SubConfig{
		{Path: filepath.Join(svcDir, ".smokesig.yaml"), Dir: svcDir, Project: "fail-svc"},
	}

	result, err := r.RunMonorepo(RunOptions{}, subs)
	if err != nil {
		t.Fatalf("RunMonorepo: %v", err)
	}
	if result.Failed != 1 {
		t.Errorf("Failed = %d, want 1", result.Failed)
	}
	if result.Passed != 0 {
		t.Errorf("Passed = %d, want 0", result.Passed)
	}
}

// --- SkipIf integration through Runner ---

func TestRunner_SkipIf_EnvUnset(t *testing.T) {
	cfg := newConfig([]schema.Test{
		{
			Name:    "skipped-test",
			Run:     "exit 1",
			Expect:  schema.Expect{ExitCode: intPtr(0)},
			SkipIf:  &schema.SkipIf{EnvUnset: "COSMO_RUNNER_SKIP_XYZ"},
		},
	})
	r := &Runner{Config: cfg, Reporter: &noopReporter{}, ConfigDir: t.TempDir()}
	result, err := r.Run(RunOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Skipped != 1 {
		t.Errorf("Skipped = %d, want 1", result.Skipped)
	}
}

func TestRunner_EnvExists_WithSetEnv(t *testing.T) {
	t.Setenv("COSMO_RUNNER_ENV_EXISTS_TEST", "1")
	cfg := newConfig([]schema.Test{
		{
			Name: "env-check",
			Run:  "echo ok",
			Expect: schema.Expect{
				ExitCode: intPtr(0),
				EnvExists: "COSMO_RUNNER_ENV_EXISTS_TEST",
			},
		},
	})
	r := &Runner{Config: cfg, Reporter: &noopReporter{}, ConfigDir: t.TempDir()}
	result, err := r.Run(RunOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Passed != 1 {
		t.Errorf("Passed = %d, want 1", result.Passed)
	}
}

func TestRunner_EnvExists_UnsetEnv(t *testing.T) {
	cfg := newConfig([]schema.Test{
		{
			Name: "env-check-missing",
			Run:  "echo ok",
			Expect: schema.Expect{
				ExitCode: intPtr(0),
				EnvExists: "COSMO_RUNNER_ENV_UNSET_XYZ",
			},
		},
	})
	r := &Runner{Config: cfg, Reporter: &noopReporter{}, ConfigDir: t.TempDir()}
	result, err := r.Run(RunOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Passed != 0 {
		t.Errorf("Passed = %d, want 0 (env not set)", result.Passed)
	}
}

func TestRunner_FileExists_Present(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "testfile.txt"), []byte("data"), 0644)

	cfg := newConfig([]schema.Test{
		{
			Name: "file-check",
			Run:  "echo ok",
			Expect: schema.Expect{
				ExitCode:  intPtr(0),
				FileExists: "testfile.txt",
			},
		},
	})
	r := &Runner{Config: cfg, Reporter: &noopReporter{}, ConfigDir: dir}
	result, err := r.Run(RunOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Passed != 1 {
		t.Errorf("Passed = %d, want 1", result.Passed)
	}
}

func TestRunner_StderrContains(t *testing.T) {
	cfg := newConfig([]schema.Test{
		{
			Name: "stderr-check",
			Run:  "echo error >&2",
			Expect: schema.Expect{
				StderrContains: "error",
			},
		},
	})
	r := &Runner{Config: cfg, Reporter: &noopReporter{}, ConfigDir: t.TempDir()}
	result, err := r.Run(RunOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Passed != 1 {
		t.Errorf("Passed = %d, want 1", result.Passed)
	}
}

func TestRunner_StdoutMatches(t *testing.T) {
	cfg := newConfig([]schema.Test{
		{
			Name: "stdout-regex",
			Run:  "echo 'version 1.2.3'",
			Expect: schema.Expect{
				StdoutMatches: `version \d+\.\d+\.\d+`,
			},
		},
	})
	r := &Runner{Config: cfg, Reporter: &noopReporter{}, ConfigDir: t.TempDir()}
	result, err := r.Run(RunOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Passed != 1 {
		t.Errorf("Passed = %d, want 1", result.Passed)
	}
}

func TestRunner_StderrMatches(t *testing.T) {
	cfg := newConfig([]schema.Test{
		{
			Name: "stderr-regex",
			Run:  "echo 'error code 42' >&2",
			Expect: schema.Expect{
				StderrMatches: `error code \d+`,
			},
		},
	})
	r := &Runner{Config: cfg, Reporter: &noopReporter{}, ConfigDir: t.TempDir()}
	result, err := r.Run(RunOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Passed != 1 {
		t.Errorf("Passed = %d, want 1", result.Passed)
	}
}

func TestRunner_ResponseTimeMs(t *testing.T) {
	rtms := 5000
	cfg := newConfig([]schema.Test{
		{
			Name: "fast-test",
			Run:  "echo ok",
			Expect: schema.Expect{
				ExitCode:       intPtr(0),
				ResponseTimeMs: &rtms,
			},
			Timeout: schema.Duration{Duration: 5 * time.Second},
		},
	})

	r := &Runner{Config: cfg, Reporter: &noopReporter{}, ConfigDir: t.TempDir()}
	result, err := r.Run(RunOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Passed != 1 {
		t.Errorf("Passed = %d, want 1 (echo should be < 5000ms)", result.Passed)
	}
}

func TestRunner_WithTimeout(t *testing.T) {
	cfg := newConfig([]schema.Test{
		{
			Name: "timeout-test",
			Run:  "echo ok",
			Expect: schema.Expect{
				ExitCode: intPtr(0),
			},
		},
	})
	r := &Runner{Config: cfg, Reporter: &noopReporter{}, ConfigDir: t.TempDir()}
	result, err := r.Run(RunOptions{Timeout: 5 * time.Second})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Passed != 1 {
		t.Errorf("Passed = %d, want 1", result.Passed)
	}
}

func TestRunner_ConfigTimeout(t *testing.T) {
	cfg := newConfig([]schema.Test{
		{
			Name: "config-timeout",
			Run:  "echo ok",
			Expect: schema.Expect{
				ExitCode: intPtr(0),
			},
		},
	})
	cfg.Settings.Timeout = schema.Duration{Duration: 5 * time.Second}
	r := &Runner{Config: cfg, Reporter: &noopReporter{}, ConfigDir: t.TempDir()}
	result, err := r.Run(RunOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Passed != 1 {
		t.Errorf("Passed = %d, want 1", result.Passed)
	}
}
