package detector

import (
	"strings"
	"testing"
)

// TestGenerateConfig_JavaGradle verifies Gradle prereq and tests for JavaGradle type.
func TestGenerateConfig_JavaGradle(t *testing.T) {
	cfg := GenerateConfig("/tmp/gradleproj", []ProjectType{JavaGradle})
	if len(cfg.Prereqs) == 0 {
		t.Fatal("expected prereqs for JavaGradle")
	}
	if cfg.Prereqs[0].Name != "Gradle installed" {
		t.Errorf("prereq[0]: want 'Gradle installed', got %q", cfg.Prereqs[0].Name)
	}
	if len(cfg.Tests) < 2 {
		t.Fatalf("expected >=2 tests for JavaGradle, got %d", len(cfg.Tests))
	}
}

// TestGenerateConfig_Kustomize verifies kubectl prereq and renders-manifests test.
func TestGenerateConfig_Kustomize(t *testing.T) {
	cfg := GenerateConfig("/tmp/kustproj", []ProjectType{Kustomize})
	if len(cfg.Prereqs) == 0 {
		t.Fatal("expected prereqs for Kustomize")
	}
	if cfg.Prereqs[0].Name != "kubectl installed" {
		t.Errorf("prereq[0]: want 'kubectl installed', got %q", cfg.Prereqs[0].Name)
	}
	if len(cfg.Tests) == 0 {
		t.Fatal("expected tests for Kustomize")
	}
}

// TestGenerateConfig_Serverless verifies Serverless CLI prereq and test.
func TestGenerateConfig_Serverless(t *testing.T) {
	cfg := GenerateConfig("/tmp/slsproj", []ProjectType{Serverless})
	if len(cfg.Prereqs) == 0 {
		t.Fatal("expected prereqs for Serverless")
	}
	if cfg.Prereqs[0].Name != "Serverless CLI installed" {
		t.Errorf("prereq[0]: want 'Serverless CLI installed', got %q", cfg.Prereqs[0].Name)
	}
	if len(cfg.Tests) == 0 {
		t.Fatal("expected tests for Serverless")
	}
}

// TestGenerateConfig_Scala verifies sbt prereq and tests.
func TestGenerateConfig_Scala(t *testing.T) {
	cfg := GenerateConfig("/tmp/scalaproj", []ProjectType{Scala})
	if len(cfg.Prereqs) == 0 {
		t.Fatal("expected prereqs for Scala")
	}
	if cfg.Prereqs[0].Name != "sbt installed" {
		t.Errorf("prereq[0]: want 'sbt installed', got %q", cfg.Prereqs[0].Name)
	}
	if len(cfg.Tests) < 2 {
		t.Fatalf("expected >=2 tests for Scala, got %d", len(cfg.Tests))
	}
}

// TestGenerateConfig_SwiftServer verifies Swift prereq and tests.
func TestGenerateConfig_SwiftServer(t *testing.T) {
	cfg := GenerateConfig("/tmp/swiftproj", []ProjectType{SwiftServer})
	if len(cfg.Prereqs) == 0 {
		t.Fatal("expected prereqs for SwiftServer")
	}
	if cfg.Prereqs[0].Name != "Swift installed" {
		t.Errorf("prereq[0]: want 'Swift installed', got %q", cfg.Prereqs[0].Name)
	}
	if len(cfg.Tests) < 2 {
		t.Fatalf("expected >=2 tests for SwiftServer, got %d", len(cfg.Tests))
	}
}

// TestGenerateConfig_DartServer verifies Dart prereq and tests.
func TestGenerateConfig_DartServer(t *testing.T) {
	cfg := GenerateConfig("/tmp/dartproj", []ProjectType{DartServer})
	if len(cfg.Prereqs) == 0 {
		t.Fatal("expected prereqs for DartServer")
	}
	if cfg.Prereqs[0].Name != "Dart installed" {
		t.Errorf("prereq[0]: want 'Dart installed', got %q", cfg.Prereqs[0].Name)
	}
	if len(cfg.Tests) < 2 {
		t.Fatalf("expected >=2 tests for DartServer, got %d", len(cfg.Tests))
	}
}

// TestGenerateConfig_Hugo verifies Hugo prereq and site-builds test.
func TestGenerateConfig_Hugo(t *testing.T) {
	cfg := GenerateConfig("/tmp/hugoproj", []ProjectType{Hugo})
	if len(cfg.Prereqs) == 0 {
		t.Fatal("expected prereqs for Hugo")
	}
	if cfg.Prereqs[0].Name != "Hugo installed" {
		t.Errorf("prereq[0]: want 'Hugo installed', got %q", cfg.Prereqs[0].Name)
	}
	if len(cfg.Tests) == 0 {
		t.Fatal("expected tests for Hugo")
	}
	if cfg.Tests[0].Name != "Site builds" {
		t.Errorf("test[0]: want 'Site builds', got %q", cfg.Tests[0].Name)
	}
}

// TestGenerateConfig_Astro verifies Node prereq and Astro-specific tests.
func TestGenerateConfig_Astro(t *testing.T) {
	cfg := GenerateConfig("/tmp/astroproj", []ProjectType{Astro})
	if len(cfg.Prereqs) == 0 {
		t.Fatal("expected prereqs for Astro")
	}
	if cfg.Prereqs[0].Name != "Node installed" {
		t.Errorf("prereq[0]: want 'Node installed', got %q", cfg.Prereqs[0].Name)
	}
	if len(cfg.Tests) < 2 {
		t.Fatalf("expected >=2 tests for Astro, got %d", len(cfg.Tests))
	}
	foundBuild := false
	for _, test := range cfg.Tests {
		if strings.Contains(test.Run, "astro build") {
			foundBuild = true
		}
	}
	if !foundBuild {
		t.Error("expected an 'astro build' test for Astro")
	}
}

// TestGenerateConfig_Jekyll verifies Ruby prereq and Jekyll build test.
func TestGenerateConfig_Jekyll(t *testing.T) {
	cfg := GenerateConfig("/tmp/jekyllproj", []ProjectType{Jekyll})
	if len(cfg.Prereqs) == 0 {
		t.Fatal("expected prereqs for Jekyll")
	}
	if cfg.Prereqs[0].Name != "Ruby installed" {
		t.Errorf("prereq[0]: want 'Ruby installed', got %q", cfg.Prereqs[0].Name)
	}
	if len(cfg.Tests) == 0 {
		t.Fatal("expected tests for Jekyll")
	}
	if cfg.Tests[0].Name != "Site builds" {
		t.Errorf("test[0]: want 'Site builds', got %q", cfg.Tests[0].Name)
	}
}

// TestGenerateConfig_Haskell verifies Stack prereq and tests.
func TestGenerateConfig_Haskell(t *testing.T) {
	cfg := GenerateConfig("/tmp/haskellproj", []ProjectType{Haskell})
	if len(cfg.Prereqs) == 0 {
		t.Fatal("expected prereqs for Haskell")
	}
	if cfg.Prereqs[0].Name != "Stack installed" {
		t.Errorf("prereq[0]: want 'Stack installed', got %q", cfg.Prereqs[0].Name)
	}
	if len(cfg.Tests) < 2 {
		t.Fatalf("expected >=2 tests for Haskell, got %d", len(cfg.Tests))
	}
}

// TestGenerateConfig_Lua verifies Lua prereq and build test.
func TestGenerateConfig_Lua(t *testing.T) {
	cfg := GenerateConfig("/tmp/luaproj", []ProjectType{Lua})
	if len(cfg.Prereqs) == 0 {
		t.Fatal("expected prereqs for Lua")
	}
	if cfg.Prereqs[0].Name != "Lua installed" {
		t.Errorf("prereq[0]: want 'Lua installed', got %q", cfg.Prereqs[0].Name)
	}
	if len(cfg.Tests) == 0 {
		t.Fatal("expected tests for Lua")
	}
}

// TestGenerateConfig_IOS verifies IOS produces a deep link test.
func TestGenerateConfig_IOS(t *testing.T) {
	cfg := GenerateConfig("/tmp/iosproj", []ProjectType{IOS})
	if len(cfg.Tests) == 0 {
		t.Fatal("expected tests for IOS")
	}
	found := false
	for _, test := range cfg.Tests {
		if test.Expect.DeepLink != nil {
			found = true
		}
	}
	if !found {
		t.Error("expected a deep link test for IOS")
	}
}

// TestGenerateConfig_Android verifies Android produces a deep link test.
func TestGenerateConfig_Android(t *testing.T) {
	cfg := GenerateConfig("/tmp/androidproj", []ProjectType{Android})
	if len(cfg.Tests) == 0 {
		t.Fatal("expected tests for Android")
	}
	found := false
	for _, test := range cfg.Tests {
		if test.Expect.DeepLink != nil {
			found = true
		}
	}
	if !found {
		t.Error("expected a deep link test for Android")
	}
}
