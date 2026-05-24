package detector

import (
	"testing"
)

func TestBoolPtr(t *testing.T) {
	p := boolPtr(true)
	if p == nil {
		t.Fatal("boolPtr(true) returned nil")
	}
	if !*p {
		t.Error("boolPtr(true) should return pointer to true")
	}
	p2 := boolPtr(false)
	if p2 == nil {
		t.Fatal("boolPtr(false) returned nil")
	}
	if *p2 {
		t.Error("boolPtr(false) should return pointer to false")
	}
}

func TestGenerateConfig_ReactNative(t *testing.T) {
	cfg := GenerateConfig("/tmp/myrnapp", []ProjectType{ReactNative})
	if cfg.Project != "myrnapp" {
		t.Errorf("project = %q, want %q", cfg.Project, "myrnapp")
	}
	if len(cfg.Tests) == 0 {
		t.Fatal("expected at least one test")
	}
	found := false
	for _, test := range cfg.Tests {
		if test.Expect.DeepLink != nil {
			found = true
		}
	}
	if !found {
		t.Error("expected a deep link test for ReactNative")
	}
}

func TestGenerateConfig_Flutter(t *testing.T) {
	cfg := GenerateConfig("/tmp/flutterapp", []ProjectType{Flutter})
	if cfg.Project != "flutterapp" {
		t.Errorf("project = %q, want %q", cfg.Project, "flutterapp")
	}
	if len(cfg.Tests) == 0 {
		t.Fatal("expected at least one test")
	}
	found := false
	for _, test := range cfg.Tests {
		if test.Expect.DeepLink != nil {
			found = true
		}
	}
	if !found {
		t.Error("expected a deep link test for Flutter")
	}
}

func TestGenerateConfig_Multiple(t *testing.T) {
	cfg := GenerateConfig("/tmp/multiproject", []ProjectType{Go, Docker})
	if cfg.Project != "multiproject" {
		t.Errorf("project = %q, want %q", cfg.Project, "multiproject")
	}
	if len(cfg.Tests) < 3 {
		t.Fatalf("expected >= 3 tests for Go+Docker, got %d", len(cfg.Tests))
	}
	if len(cfg.Prereqs) == 0 {
		t.Fatal("expected prereqs from Go type")
	}
}

func TestGenerateConfig_EmptyTypes(t *testing.T) {
	cfg := GenerateConfig("/tmp/emptyproj", []ProjectType{})
	if cfg == nil {
		t.Fatal("expected non-nil config")
	}
	if cfg.Version != 1 {
		t.Errorf("version = %d, want 1", cfg.Version)
	}
	if cfg.Project != "emptyproj" {
		t.Errorf("project = %q, want %q", cfg.Project, "emptyproj")
	}
	if len(cfg.Tests) != 0 {
		t.Errorf("expected 0 tests for empty types, got %d", len(cfg.Tests))
	}
}
