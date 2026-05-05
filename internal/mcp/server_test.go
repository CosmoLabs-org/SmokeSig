package mcp

import (
	"context"
	"os"
	"testing"
)

// TestNewServerRegistersTools verifies that the MCP server registers all 7 expected tools.
func TestNewServerRegistersTools(t *testing.T) {
	srv := NewServer()
	if srv == nil {
		t.Fatal("NewServer() returned nil")
	}

	expected := []string{
		"smoke_run",
		"smoke_init",
		"smoke_validate",
		"smoke_list",
		"smoke_discover",
		"smoke_explain",
		"smoke_generate_test",
	}

	registered := srv.ToolNames()
	if len(registered) != len(expected) {
		t.Errorf("expected %d tools, got %d", len(expected), len(registered))
	}

	for _, name := range expected {
		handler := srv.Handler(name)
		if handler == nil {
			t.Errorf("expected handler for tool %q, got nil", name)
		}
	}
}

// TestSmokeRunHandlerExists verifies smoke_run handler is non-nil.
func TestSmokeRunHandlerExists(t *testing.T) {
	srv := NewServer()
	handler := srv.Handler("smoke_run")
	if handler == nil {
		t.Fatal("smoke_run handler should be registered")
	}
}

// TestSmokeRunAgainstSelf runs smoke_run against the project's own .smokesig.yaml.
func TestSmokeRunAgainstSelf(t *testing.T) {
	configPath := "../../.smokesig.yaml"
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Skip("no .smokesig.yaml found, skipping integration test")
	}

	srv := NewServer()
	handler := srv.Handler("smoke_run")

	result, err := handler(context.Background(), map[string]interface{}{
		"config_path": configPath,
	})
	if err != nil {
		t.Fatalf("smoke_run handler returned error: %v", err)
	}

	runResult, ok := result.(*RunResult)
	if !ok {
		t.Fatalf("expected *RunResult, got %T", result)
	}

	if runResult.Total == 0 {
		t.Error("expected at least one test in .smokesig.yaml")
	}

	if runResult.Passed+runResult.Failed+runResult.Skipped != runResult.Total {
		t.Errorf("result counts don't add up: passed=%d failed=%d skipped=%d total=%d",
			runResult.Passed, runResult.Failed, runResult.Skipped, runResult.Total)
	}
}

// TestSmokeRunWithTags tests tag filtering via the handler.
func TestSmokeRunWithTags(t *testing.T) {
	configPath := "../../.smokesig.yaml"
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Skip("no .smokesig.yaml found, skipping integration test")
	}

	srv := NewServer()
	handler := srv.Handler("smoke_run")

	// Run with a non-existent tag — should return 0 tests
	result, err := handler(context.Background(), map[string]interface{}{
		"config_path": configPath,
		"tags":        []interface{}{"nonexistent-tag-xyz"},
	})
	if err != nil {
		t.Fatalf("smoke_run with tags returned error: %v", err)
	}

	runResult := result.(*RunResult)
	if runResult.Total != 0 {
		t.Errorf("expected 0 tests with nonexistent tag, got %d", runResult.Total)
	}
}

// TestMCPServerHasUnderlyingServer verifies the mcp-go server is initialized.
func TestMCPServerHasUnderlyingServer(t *testing.T) {
	srv := NewServer()
	if srv.MCPServer() == nil {
		t.Error("MCPServer() should not return nil")
	}
}
