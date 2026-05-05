package cmd

import (
	"encoding/json"
	"testing"

	"github.com/CosmoLabs-org/SmokeSig/internal/schema"
)

func TestSchemaJSONIsValid(t *testing.T) {
	data, err := schema.ExportSchemaJSON()
	if err != nil {
		t.Fatalf("ExportSchemaJSON returned error: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("ExportSchemaJSON returned empty data")
	}
	if !json.Valid(data) {
		t.Fatalf("schema output is not valid JSON:\n%s", string(data))
	}
}

func TestSchemaRoundtrip(t *testing.T) {
	original := schema.ExportSchema()
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var decoded schema.SchemaOutput
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if decoded.Version != original.Version {
		t.Errorf("version mismatch: got %q, want %q", decoded.Version, original.Version)
	}
	if len(decoded.AssertionTypes) != len(original.AssertionTypes) {
		t.Fatalf("assertion count mismatch: got %d, want %d", len(decoded.AssertionTypes), len(original.AssertionTypes))
	}
	for i, got := range decoded.AssertionTypes {
		want := original.AssertionTypes[i]
		if got.Name != want.Name {
			t.Errorf("assertion[%d].Name: got %q, want %q", i, got.Name, want.Name)
		}
		if got.YAML != want.YAML {
			t.Errorf("assertion[%d].YAML: got %q, want %q", i, got.YAML, want.YAML)
		}
		if len(got.Fields) != len(want.Fields) {
			t.Errorf("assertion[%d].Fields count: got %d, want %d", i, len(got.Fields), len(want.Fields))
		}
	}
}

func TestSchemaContainsExpectedAssertionTypes(t *testing.T) {
	s := schema.ExportSchema()
	expected := []string{
		"exit_code", "stdout_contains", "http", "json_field",
		"ssl_cert", "redis_ping", "websocket", "docker_container_running",
		"url_reachable", "s3_bucket", "version_check", "otel_trace",
		"credential_check", "graphql",
	}

	found := make(map[string]bool, len(s.AssertionTypes))
	for _, a := range s.AssertionTypes {
		found[a.Name] = true
	}

	for _, name := range expected {
		if !found[name] {
			t.Errorf("missing expected assertion type: %q", name)
		}
	}
}

func TestSchemaEachAssertionHasFields(t *testing.T) {
	s := schema.ExportSchema()
	for _, a := range s.AssertionTypes {
		if len(a.Fields) == 0 {
			t.Errorf("assertion type %q has no fields", a.Name)
		}
	}
}

func TestSchemaFieldTypesNonEmpty(t *testing.T) {
	s := schema.ExportSchema()
	for _, a := range s.AssertionTypes {
		for _, f := range a.Fields {
			if f.Type == "" {
				t.Errorf("assertion %q field %q has empty type", a.Name, f.Name)
			}
			if f.Name == "" {
				t.Errorf("assertion %q has a field with empty name", a.Name)
			}
		}
	}
}
