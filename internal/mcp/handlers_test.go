package mcp

import (
	"context"
	"os"
	"testing"
)

// TestHandleSmokeValidate tests config validation via MCP handler.
func TestHandleSmokeValidate(t *testing.T) {
	configPath := "../../.smokesig.yaml"
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Skip("no .smokesig.yaml found")
	}

	srv := NewServer()
	handler := srv.Handler("smoke_validate")

	result, err := handler(context.Background(), map[string]interface{}{
		"config_path": configPath,
	})
	if err != nil {
		t.Fatalf("smoke_validate returned error: %v", err)
	}

	vr, ok := result.(*ValidateResult)
	if !ok {
		t.Fatalf("expected *ValidateResult, got %T", result)
	}

	if !vr.Valid {
		t.Errorf("expected valid config, got errors: %v", vr.Errors)
	}
	if len(vr.Tests) == 0 {
		t.Error("expected at least one test in valid config")
	}
}

// TestHandleSmokeValidateBadPath tests validation with a nonexistent config.
func TestHandleSmokeValidateBadPath(t *testing.T) {
	srv := NewServer()
	handler := srv.Handler("smoke_validate")

	_, err := handler(context.Background(), map[string]interface{}{
		"config_path": "/nonexistent/.smokesig.yaml",
	})
	if err == nil {
		t.Error("expected error for nonexistent config path")
	}
}

// TestHandleSmokeList tests listing tests via MCP handler.
func TestHandleSmokeList(t *testing.T) {
	configPath := "../../.smokesig.yaml"
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Skip("no .smokesig.yaml found")
	}

	srv := NewServer()
	handler := srv.Handler("smoke_list")

	result, err := handler(context.Background(), map[string]interface{}{
		"config_path": configPath,
	})
	if err != nil {
		t.Fatalf("smoke_list returned error: %v", err)
	}

	lr, ok := result.(*ListResult)
	if !ok {
		t.Fatalf("expected *ListResult, got %T", result)
	}

	if len(lr.Tests) == 0 {
		t.Error("expected at least one test in config")
	}

	// Each test should have a name
	for _, test := range lr.Tests {
		if test.Name == "" {
			t.Error("test entry has empty name")
		}
	}
}

// TestHandleSmokeListWithTags tests tag filtering in list.
func TestHandleSmokeListWithTags(t *testing.T) {
	configPath := "../../.smokesig.yaml"
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Skip("no .smokesig.yaml found")
	}

	srv := NewServer()
	handler := srv.Handler("smoke_list")

	result, err := handler(context.Background(), map[string]interface{}{
		"config_path": configPath,
		"tags":        []interface{}{"nonexistent-tag-xyz"},
	})
	if err != nil {
		t.Fatalf("smoke_list with tags returned error: %v", err)
	}

	lr := result.(*ListResult)
	if len(lr.Tests) != 0 {
		t.Errorf("expected 0 tests with nonexistent tag, got %d", len(lr.Tests))
	}
}

// TestHandleSmokeDiscover tests finding .smokesig.yaml files.
func TestHandleSmokeDiscover(t *testing.T) {
	dir := "../.."
	if _, err := os.Stat(dir + "/.smokesig.yaml"); os.IsNotExist(err) {
		t.Skip("no .smokesig.yaml found in project root")
	}

	srv := NewServer()
	handler := srv.Handler("smoke_discover")

	result, err := handler(context.Background(), map[string]interface{}{
		"directory": dir,
	})
	if err != nil {
		t.Fatalf("smoke_discover returned error: %v", err)
	}

	dr, ok := result.(*DiscoverResult)
	if !ok {
		t.Fatalf("expected *DiscoverResult, got %T", result)
	}

	if len(dr.Configs) == 0 {
		t.Error("expected at least one discovered config")
	}

	found := false
	for _, cfg := range dr.Configs {
		if cfg.ProjectName != "" && cfg.Path != "" {
			found = true
			break
		}
	}
	if !found {
		t.Error("discovered configs missing project_name or path")
	}
}

// TestHandleSmokeExplain tests assertion type explanation.
func TestHandleSmokeExplain(t *testing.T) {
	tests := []struct {
		assertionType string
		wantDesc      string
	}{
		{"http", "HTTP endpoint"},
		{"redis_ping", "Redis PING"},
		{"exit_code", "exit code"},
		{"port_listening", "port"},
		{"ssl_cert", "TLS certificate"},
	}

	srv := NewServer()
	handler := srv.Handler("smoke_explain")

	for _, tt := range tests {
		t.Run(tt.assertionType, func(t *testing.T) {
			result, err := handler(context.Background(), map[string]interface{}{
				"assertion_type": tt.assertionType,
			})
			if err != nil {
				t.Fatalf("smoke_explain(%s) returned error: %v", tt.assertionType, err)
			}

			er, ok := result.(*ExplainResult)
			if !ok {
				t.Fatalf("expected *ExplainResult, got %T", result)
			}

			if er.Type != tt.assertionType {
				t.Errorf("expected type %s, got %s", tt.assertionType, er.Type)
			}
			if er.Example == "" {
				t.Error("expected non-empty example YAML")
			}
			if len(er.Fields) == 0 {
				t.Error("expected at least one field description")
			}
		})
	}
}

// TestHandleSmokeExplainUnknown tests explanation for unknown assertion type.
func TestHandleSmokeExplainUnknown(t *testing.T) {
	srv := NewServer()
	handler := srv.Handler("smoke_explain")

	_, err := handler(context.Background(), map[string]interface{}{
		"assertion_type": "totally_fake_type",
	})
	if err == nil {
		t.Error("expected error for unknown assertion type")
	}
}
