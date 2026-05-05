package runner

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/CosmoLabs-org/SmokeSig/internal/schema"
)

func TestLifecycleIntegration_BeforeAllAfterAll(t *testing.T) {
	tmpDir := t.TempDir()

	flagFile := func(name string) string {
		return filepath.Join(tmpDir, name)
	}

	cfg := &schema.SmokeConfig{
		Version: 1,
		Project: "lifecycle-test",
		Lifecycle: schema.LifecycleConfig{
			BeforeAll: []schema.LifecycleHook{
				{Command: "touch " + flagFile("before-all-ran")},
			},
			AfterAll: []schema.LifecycleHook{
				{Command: "touch " + flagFile("after-all-ran"), AlwaysRun: true},
			},
		},
		Tests: []schema.Test{
			{Name: "test1", Run: "echo test1"},
			{Name: "test2", Run: "echo test2"},
		},
	}

	runner := &Runner{
		Config:    cfg,
		ConfigDir: tmpDir,
		Reporter:  &noopReporter{},
		Vars:      NewVarStore(),
	}

	_, err := runner.Run(RunOptions{})
	if err != nil {
		t.Fatalf("Run() failed: %v", err)
	}

	checkFile := func(path string, shouldExist bool) {
		if _, err := os.Stat(path); shouldExist && os.IsNotExist(err) {
			t.Errorf("expected file %s to exist", path)
		} else if !shouldExist && err == nil {
			t.Errorf("expected file %s to not exist", path)
		}
	}

	checkFile(flagFile("before-all-ran"), true)
	checkFile(flagFile("after-all-ran"), true)
}

func TestLifecycleIntegration_BeforeEachAfterEach(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &schema.SmokeConfig{
		Version: 1,
		Project: "lifecycle-test",
		Lifecycle: schema.LifecycleConfig{
			BeforeEach: []schema.LifecycleHook{
				{Command: "touch " + filepath.Join(tmpDir, "before-each-flag")},
			},
			AfterEach: []schema.LifecycleHook{
				{Command: "touch " + filepath.Join(tmpDir, "after-each-flag")},
			},
		},
		Tests: []schema.Test{
			{Name: "test1", Run: "echo test1"},
			{Name: "test2", Run: "echo test2"},
		},
	}

	runner := &Runner{
		Config:    cfg,
		ConfigDir: tmpDir,
		Reporter:  &noopReporter{},
		Vars:      NewVarStore(),
	}

	_, err := runner.Run(RunOptions{})
	if err != nil {
		t.Fatalf("Run() failed: %v", err)
	}

	checkFile := func(path string, shouldExist bool) {
		if _, err := os.Stat(path); shouldExist && os.IsNotExist(err) {
			t.Errorf("expected file %s to exist", path)
		} else if !shouldExist && err == nil {
			t.Errorf("expected file %s to not exist", path)
		}
	}

	checkFile(filepath.Join(tmpDir, "before-each-flag"), true)
	checkFile(filepath.Join(tmpDir, "after-each-flag"), true)
}
