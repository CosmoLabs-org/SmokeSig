package schema

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestResolveRemote_HTTPS(t *testing.T) {
	remoteYAML := `version: 1
project: base-config
tests:
  - name: remote-test
    run: echo "from remote"
`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("expected GET request, got %s", r.Method)
		}
		w.Header().Set("Content-Type", "text/yaml")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, remoteYAML)
	}))
	defer server.Close()

	resolver := NewRemoteResolver(t.TempDir())
	ctx := context.Background()

	data, err := resolver.Resolve(ctx, server.URL)
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}

	cfg, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if cfg.Project != "base-config" {
		t.Errorf("expected project 'base-config', got '%s'", cfg.Project)
	}

	if len(cfg.Tests) != 1 {
		t.Fatalf("expected 1 test, got %d", len(cfg.Tests))
	}

	if cfg.Tests[0].Name != "remote-test" {
		t.Errorf("expected test name 'remote-test', got '%s'", cfg.Tests[0].Name)
	}
}

func TestResolveRemote_FileScheme(t *testing.T) {
	remoteYAML := `version: 1
project: file-config
tests:
  - name: file-test
    run: echo "from file"
`
	tmpDir := t.TempDir()
	cfgFile := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(cfgFile, []byte(remoteYAML), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	resolver := NewRemoteResolver(t.TempDir())
	ctx := context.Background()

	data, err := resolver.Resolve(ctx, "file://"+cfgFile)
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}

	cfg, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if cfg.Project != "file-config" {
		t.Errorf("expected project 'file-config', got '%s'", cfg.Project)
	}
}

func TestResolveRemote_CacheHit(t *testing.T) {
	remoteYAML := `version: 1
project: cached-config
`
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.Header().Set("Content-Type", "text/yaml")
		w.Header().Set("ETag", `"v1"`)
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, remoteYAML)
	}))
	defer server.Close()

	cacheDir := t.TempDir()
	resolver := NewRemoteResolver(cacheDir)
	ctx := context.Background()

	firstData, err := resolver.Resolve(ctx, server.URL)
	if err != nil {
		t.Fatalf("First Resolve failed: %v", err)
	}

	if requestCount != 1 {
		t.Errorf("expected 1 request after first resolve, got %d", requestCount)
	}

	secondData, err := resolver.Resolve(ctx, server.URL)
	if err != nil {
		t.Fatalf("Second Resolve failed: %v", err)
	}

	if requestCount != 2 {
		t.Errorf("expected 2 requests (one with If-None-Match), got %d", requestCount)
	}

	if string(firstData) != string(secondData) {
		t.Error("cached data should match first response")
	}
}

func TestResolveRemote_CacheHit_304(t *testing.T) {
	remoteYAML := `version: 1
project: cached-304
`
	requestCount := 0
	etagSent := false

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		if r.Header.Get("If-None-Match") == `"v1"` {
			etagSent = true
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("Content-Type", "text/yaml")
		w.Header().Set("ETag", `"v1"`)
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, remoteYAML)
	}))
	defer server.Close()

	cacheDir := t.TempDir()
	resolver := NewRemoteResolver(cacheDir)
	ctx := context.Background()

	firstData, err := resolver.Resolve(ctx, server.URL)
	if err != nil {
		t.Fatalf("First Resolve failed: %v", err)
	}

	if requestCount != 1 {
		t.Errorf("expected 1 request after first resolve, got %d", requestCount)
	}

	secondData, err := resolver.Resolve(ctx, server.URL)
	if err != nil {
		t.Fatalf("Second Resolve failed: %v", err)
	}

	if requestCount != 2 {
		t.Errorf("expected 2 requests, got %d", requestCount)
	}

	if !etagSent {
		t.Error("expected second request to send If-None-Match header")
	}

	if string(firstData) != string(secondData) {
		t.Error("cached data should match first response")
	}
}

func TestResolveRemote_InvalidYAML(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/yaml")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "not valid yaml {{{")
	}))
	defer server.Close()

	resolver := NewRemoteResolver(t.TempDir())
	ctx := context.Background()

	_, err := resolver.Resolve(ctx, server.URL)
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}

	if !strings.Contains(err.Error(), "invalid YAML") {
		t.Errorf("expected error mentioning invalid YAML, got: %v", err)
	}
}

func TestResolveRemote_HTTPWarns(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "version: 1\nproject: test\n")
	}))
	defer server.Close()

	resolver := NewRemoteResolver(t.TempDir())
	ctx := context.Background()

	_, err := resolver.Resolve(ctx, server.URL)
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}
}

func TestResolveRemote_ContextCanceled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "version: 1\nproject: test\n")
	}))
	defer server.Close()

	resolver := NewRemoteResolver(t.TempDir())
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := resolver.Resolve(ctx, server.URL)
	if err == nil {
		t.Fatal("expected error for canceled context")
	}
}

func TestMergeConfigs_RemoteBaseLocalOverride(t *testing.T) {
	exitCode := 0
	base := SmokeConfig{
		Version:     1,
		Project:     "base",
		Description: "Base description",
		Settings: Settings{
			Timeout:  Duration{Duration: 10 * time.Second},
			FailFast: false,
		},
		OTel: OTelConfig{
			Enabled:   false,
			JaegerURL: "http://old-jaeger:16686",
		},
		Tests: []Test{
			{Name: "base-test", Run: "echo base", Expect: Expect{ExitCode: &exitCode}},
		},
		Prereqs: []Prerequisite{
			{Name: "base-prereq", Check: "command -v docker"},
		},
	}

	overlay := SmokeConfig{
		Version:     1,
		Project:     "overlay",
		Description: "Overlay description",
		Settings: Settings{
			Timeout:  Duration{Duration: 30 * time.Second},
			FailFast: true,
			Parallel: true,
		},
		OTel: OTelConfig{
			Enabled:   true,
			JaegerURL: "http://new-jaeger:16686",
		},
		Tests: []Test{
			{Name: "overlay-test", Run: "echo overlay", Expect: Expect{ExitCode: &exitCode}},
		},
		Prereqs: []Prerequisite{
			{Name: "overlay-prereq", Check: "command -v node"},
		},
	}

	merged := MergeConfigs(base, overlay)

	if merged.Project != "overlay" {
		t.Errorf("expected project 'overlay', got '%s'", merged.Project)
	}

	if merged.Description != "Overlay description" {
		t.Errorf("expected description 'Overlay description', got '%s'", merged.Description)
	}

	if merged.Settings.Timeout.Duration != 30*time.Second {
		t.Errorf("expected timeout 30s, got %v", merged.Settings.Timeout.Duration)
	}

	if !merged.Settings.FailFast {
		t.Error("expected fail_fast to be true")
	}

	if !merged.Settings.Parallel {
		t.Error("expected parallel to be true")
	}

	if !merged.OTel.Enabled {
		t.Error("expected OTel enabled to be true")
	}

	if merged.OTel.JaegerURL != "http://new-jaeger:16686" {
		t.Errorf("expected JaegerURL 'http://new-jaeger:16686', got '%s'", merged.OTel.JaegerURL)
	}
}

func TestMergeConfigs_TestsAppended(t *testing.T) {
	exitCode := 0
	base := SmokeConfig{
		Version: 1,
		Project: "base",
		Tests: []Test{
			{Name: "test1", Run: "echo 1", Expect: Expect{ExitCode: &exitCode}},
			{Name: "test2", Run: "echo 2", Expect: Expect{ExitCode: &exitCode}},
		},
	}

	overlay := SmokeConfig{
		Version: 1,
		Project: "overlay",
		Tests: []Test{
			{Name: "test3", Run: "echo 3", Expect: Expect{ExitCode: &exitCode}},
			{Name: "test4", Run: "echo 4", Expect: Expect{ExitCode: &exitCode}},
		},
	}

	merged := MergeConfigs(base, overlay)

	if len(merged.Tests) != 4 {
		t.Fatalf("expected 4 tests, got %d", len(merged.Tests))
	}

	if merged.Tests[0].Name != "test1" {
		t.Errorf("expected first test 'test1', got '%s'", merged.Tests[0].Name)
	}

	if merged.Tests[3].Name != "test4" {
		t.Errorf("expected last test 'test4', got '%s'", merged.Tests[3].Name)
	}
}

func TestMergeConfigs_PrereqsAppended(t *testing.T) {
	base := SmokeConfig{
		Version: 1,
		Project: "base",
		Prereqs: []Prerequisite{
			{Name: "base-prereq", Check: "command -v docker"},
		},
	}

	overlay := SmokeConfig{
		Version: 1,
		Project: "overlay",
		Prereqs: []Prerequisite{
			{Name: "overlay-prereq", Check: "command -v node"},
		},
	}

	merged := MergeConfigs(base, overlay)

	if len(merged.Prereqs) != 2 {
		t.Fatalf("expected 2 prereqs, got %d", len(merged.Prereqs))
	}

	if merged.Prereqs[0].Name != "base-prereq" {
		t.Errorf("expected first prereq 'base-prereq', got '%s'", merged.Prereqs[0].Name)
	}

	if merged.Prereqs[1].Name != "overlay-prereq" {
		t.Errorf("expected second prereq 'overlay-prereq', got '%s'", merged.Prereqs[1].Name)
	}
}

func TestMergeConfigs_EmptyOverlayPreservesBase(t *testing.T) {
	exitCode := 0
	base := SmokeConfig{
		Version:     1,
		Project:     "base",
		Description: "Base description",
		Settings: Settings{
			Timeout: Duration{Duration: 10 * time.Second},
		},
		Tests: []Test{
			{Name: "base-test", Run: "echo base", Expect: Expect{ExitCode: &exitCode}},
		},
	}

	overlay := SmokeConfig{
		Version: 1,
		Project: "overlay",
	}

	merged := MergeConfigs(base, overlay)

	if merged.Project != "overlay" {
		t.Errorf("expected project 'overlay', got '%s'", merged.Project)
	}

	if merged.Settings.Timeout.Duration != 10*time.Second {
		t.Errorf("expected base timeout preserved, got %v", merged.Settings.Timeout.Duration)
	}

	if len(merged.Tests) != 1 {
		t.Fatalf("expected 1 base test preserved, got %d", len(merged.Tests))
	}
}

func TestLoadConfig_WithExtends(t *testing.T) {
	remoteYAML := `version: 1
project: remote-base
description: "Base config from remote"
settings:
  timeout: 5s
tests:
  - name: remote-test
    run: echo "remote"
`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/yaml")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, remoteYAML)
	}))
	defer server.Close()

	localYAML := fmt.Sprintf(`extends: %s
project: local-project
description: "Local override"
settings:
  fail_fast: true
tests:
  - name: local-test
    run: echo "local"
`, server.URL)

	tmpDir := t.TempDir()
	localCfgPath := filepath.Join(tmpDir, ".smoke.yaml")
	if err := os.WriteFile(localCfgPath, []byte(localYAML), 0644); err != nil {
		t.Fatalf("failed to write local config: %v", err)
	}

	cfg, err := Load(localCfgPath)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.Project != "local-project" {
		t.Errorf("expected project 'local-project', got '%s'", cfg.Project)
	}

	if cfg.Description != "Local override" {
		t.Errorf("expected description 'Local override', got '%s'", cfg.Description)
	}

	if cfg.Settings.Timeout.Duration != 5*time.Second {
		t.Errorf("expected remote timeout preserved, got %v", cfg.Settings.Timeout.Duration)
	}

	if !cfg.Settings.FailFast {
		t.Error("expected local fail_fast to be true")
	}

	if len(cfg.Tests) != 2 {
		t.Fatalf("expected 2 tests (remote + local), got %d", len(cfg.Tests))
	}

	if cfg.Tests[0].Name != "remote-test" {
		t.Errorf("expected first test 'remote-test', got '%s'", cfg.Tests[0].Name)
	}

	if cfg.Tests[1].Name != "local-test" {
		t.Errorf("expected second test 'local-test', got '%s'", cfg.Tests[1].Name)
	}
}

func TestLoadConfig_WithExtendsAndIncludes(t *testing.T) {
	remoteYAML := `version: 1
project: remote-base
tests:
  - name: remote-test
    run: echo "remote"
`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/yaml")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, remoteYAML)
	}))
	defer server.Close()

	includeYAML := `version: 1
project: include
tests:
  - name: include-test
    run: echo "include"
`

	localYAML := fmt.Sprintf(`extends: %s
project: local-project
includes:
  - include.yaml
tests:
  - name: local-test
    run: echo "local"
`, server.URL)

	tmpDir := t.TempDir()
	localCfgPath := filepath.Join(tmpDir, ".smoke.yaml")
	includeCfgPath := filepath.Join(tmpDir, "include.yaml")

	if err := os.WriteFile(localCfgPath, []byte(localYAML), 0644); err != nil {
		t.Fatalf("failed to write local config: %v", err)
	}
	if err := os.WriteFile(includeCfgPath, []byte(includeYAML), 0644); err != nil {
		t.Fatalf("failed to write include config: %v", err)
	}

	cfg, err := Load(localCfgPath)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if len(cfg.Tests) != 3 {
		t.Fatalf("expected 3 tests (remote + include + local), got %d", len(cfg.Tests))
	}

	if cfg.Tests[0].Name != "remote-test" {
		t.Errorf("expected first test 'remote-test', got '%s'", cfg.Tests[0].Name)
	}

	if cfg.Tests[1].Name != "include-test" {
		t.Errorf("expected second test 'include-test', got '%s'", cfg.Tests[1].Name)
	}

	if cfg.Tests[2].Name != "local-test" {
		t.Errorf("expected third test 'local-test', got '%s'", cfg.Tests[2].Name)
	}
}

func TestResolveRemote_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, "Not found")
	}))
	defer server.Close()

	resolver := NewRemoteResolver(t.TempDir())
	ctx := context.Background()

	_, err := resolver.Resolve(ctx, server.URL)
	if err == nil {
		t.Fatal("expected error for 404 response")
	}

	if !strings.Contains(err.Error(), "404") {
		t.Errorf("expected error mentioning 404, got: %v", err)
	}
}

func TestResolveRemote_InvalidURLScheme(t *testing.T) {
	resolver := NewRemoteResolver(t.TempDir())
	ctx := context.Background()

	_, err := resolver.Resolve(ctx, "ftp://example.com/config.yaml")
	if err == nil {
		t.Fatal("expected error for unsupported URL scheme")
	}

	if !strings.Contains(err.Error(), "unsupported URL scheme") {
		t.Errorf("expected error mentioning unsupported URL scheme, got: %v", err)
	}
}

func TestMergeConfigs_ExportHeaders(t *testing.T) {
	base := SmokeConfig{
		Version: 1,
		Project: "base",
		OTel: OTelConfig{
			ExportHeaders: map[string]string{
				"X-Base": "base-value",
			},
		},
	}

	overlay := SmokeConfig{
		Version: 1,
		Project: "overlay",
		OTel: OTelConfig{
			ExportHeaders: map[string]string{
				"X-Overlay": "overlay-value",
			},
		},
	}

	merged := MergeConfigs(base, overlay)

	if len(merged.OTel.ExportHeaders) != 1 {
		t.Errorf("expected 1 export header (overlay replaces base), got %d", len(merged.OTel.ExportHeaders))
	}

	if merged.OTel.ExportHeaders["X-Overlay"] != "overlay-value" {
		t.Error("expected overlay export header")
	}
}
