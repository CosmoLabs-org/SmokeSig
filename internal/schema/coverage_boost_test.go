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

// ─── LoadDefault ─────────────────────────────────────────────────────────────

func TestLoadDefault_NoConfigFile(t *testing.T) {
	dir := t.TempDir()
	orig, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer os.Chdir(orig)

	_, err := LoadDefault()
	if err == nil {
		t.Fatal("expected error when no config file exists")
	}
	if !strings.Contains(err.Error(), "no config file found") {
		t.Errorf("expected 'no config file found', got: %v", err)
	}
}

func TestLoadDefault_SmokesigYaml(t *testing.T) {
	dir := t.TempDir()
	orig, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer os.Chdir(orig)

	content := `version: 1
project: myapp
tests:
  - name: ping
    run: echo ok
`
	if err := os.WriteFile(".smokesig.yaml", []byte(content), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	cfg, err := LoadDefault()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Project != "myapp" {
		t.Errorf("project = %q, want myapp", cfg.Project)
	}
}

func TestLoadDefault_LegacySmokeYaml(t *testing.T) {
	dir := t.TempDir()
	orig, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer os.Chdir(orig)

	content := `version: 1
project: legacy-app
tests:
  - name: ping
    run: echo ok
`
	if err := os.WriteFile(".smoke.yaml", []byte(content), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	cfg, err := LoadDefault()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Project != "legacy-app" {
		t.Errorf("project = %q, want legacy-app", cfg.Project)
	}
}

// ─── processTemplate ─────────────────────────────────────────────────────────

func TestProcessTemplate_MissingEnvVar(t *testing.T) {
	// Missing env var in a template produces empty string (not an error in Go templates)
	os.Unsetenv("SMOKE_NONEXISTENT_VAR_XYZ")
	data := []byte(`project: {{ .Env.SMOKE_NONEXISTENT_VAR_XYZ }}`)
	result, err := processTemplate(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// The result should have an empty substitution (Go templates don't error on missing map keys)
	if !strings.Contains(string(result), "project:") {
		t.Errorf("unexpected result: %s", result)
	}
}

func TestProcessTemplate_ValidEnvVar(t *testing.T) {
	t.Setenv("SMOKE_TEST_PROJECT", "templated-project")
	data := []byte(`project: {{ .Env.SMOKE_TEST_PROJECT }}`)
	result, err := processTemplate(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(string(result), "templated-project") {
		t.Errorf("expected 'templated-project' in result, got: %s", result)
	}
}

func TestProcessTemplate_MalformedTemplate(t *testing.T) {
	data := []byte(`project: {{ .Env.FOO `)
	_, err := processTemplate(data)
	if err == nil {
		t.Fatal("expected error for malformed template")
	}
}

// ─── Duration.UnmarshalYAML ───────────────────────────────────────────────────

func TestDurationUnmarshalYAML_InvalidDuration(t *testing.T) {
	// Use the Parse path which calls yaml.Unmarshal → UnmarshalYAML on Duration
	content := `version: 1
project: test
settings:
  timeout: "not-a-duration"
tests:
  - name: t
    run: echo ok
`
	_, err := Parse([]byte(content))
	if err == nil {
		t.Fatal("expected error for invalid duration")
	}
	if !strings.Contains(err.Error(), "invalid duration") && !strings.Contains(err.Error(), "parsing") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestDurationUnmarshalYAML_ValidDuration(t *testing.T) {
	content := `version: 1
project: test
settings:
  timeout: "30s"
tests:
  - name: t
    run: echo ok
`
	cfg, err := Parse([]byte(content))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Settings.Timeout.Duration != 30*time.Second {
		t.Errorf("timeout = %v, want 30s", cfg.Settings.Timeout.Duration)
	}
}

// ─── validateBackgroundHook ───────────────────────────────────────────────────

func TestValidateBackgroundHook_BackgroundWithoutWaitFor(t *testing.T) {
	hook := LifecycleHook{
		Command:    "server start",
		Background: true,
		// No WaitForPort, no Timeout, no StartupTimeout
	}
	msg := validateBackgroundHook(hook, "lifecycle.before_all[0]")
	if msg == "" {
		t.Fatal("expected validation error for background=true without wait_for_port or timeout")
	}
	if !strings.Contains(msg, "background=true requires") {
		t.Errorf("unexpected message: %s", msg)
	}
}

func TestValidateBackgroundHook_BackgroundWithPort(t *testing.T) {
	hook := LifecycleHook{
		Command:     "server start",
		Background:  true,
		WaitForPort: 8080,
	}
	msg := validateBackgroundHook(hook, "lifecycle.before_all[0]")
	if msg != "" {
		t.Errorf("expected no error, got: %s", msg)
	}
}

func TestValidateBackgroundHook_BackgroundWithTimeout(t *testing.T) {
	hook := LifecycleHook{
		Command:    "server start",
		Background: true,
		Timeout:    Duration{Duration: 5 * time.Second},
	}
	msg := validateBackgroundHook(hook, "lifecycle.before_all[0]")
	if msg != "" {
		t.Errorf("expected no error, got: %s", msg)
	}
}

func TestValidateBackgroundHook_BackgroundWithStartupTimeout(t *testing.T) {
	hook := LifecycleHook{
		Command:        "server start",
		Background:     true,
		StartupTimeout: Duration{Duration: 10 * time.Second},
	}
	msg := validateBackgroundHook(hook, "lifecycle.before_all[0]")
	if msg != "" {
		t.Errorf("expected no error, got: %s", msg)
	}
}

func TestValidateBackgroundHook_NotBackground(t *testing.T) {
	// Non-background hooks always pass even without wait_for or timeout
	hook := LifecycleHook{
		Command:    "setup.sh",
		Background: false,
	}
	msg := validateBackgroundHook(hook, "lifecycle.before_all[0]")
	if msg != "" {
		t.Errorf("expected no error for non-background hook, got: %s", msg)
	}
}

func TestValidate_BackgroundHookIntegration(t *testing.T) {
	cfg := &SmokeConfig{
		Version: 1,
		Project: "test",
		Lifecycle: LifecycleConfig{
			BeforeAll: []LifecycleHook{
				{Command: "start-server", Background: true}, // missing wait_for_port / timeout
			},
		},
		Tests: []Test{{Name: "t1", Run: "true"}},
	}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected validation error for background hook without wait_for")
	}
	if !strings.Contains(err.Error(), "background=true requires") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidate_BackgroundHookBeforeEachIntegration(t *testing.T) {
	cfg := &SmokeConfig{
		Version: 1,
		Project: "test",
		Lifecycle: LifecycleConfig{
			BeforeEach: []LifecycleHook{
				{Command: "start-server", Background: true}, // missing wait_for_port / timeout
			},
		},
		Tests: []Test{{Name: "t1", Run: "true"}},
	}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected validation error for before_each background hook without wait_for")
	}
	if !strings.Contains(err.Error(), "background=true requires") {
		t.Errorf("unexpected error: %v", err)
	}
}

// ─── fetchFile (via Resolve with file:// scheme) ──────────────────────────────

func TestResolveFile_NotFound(t *testing.T) {
	resolver := NewRemoteResolver(t.TempDir())
	ctx := context.Background()

	_, err := resolver.Resolve(ctx, "file:///nonexistent/path/to/config.yaml")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
	if !strings.Contains(err.Error(), "reading file") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestResolveFile_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "bad.yaml")
	if err := os.WriteFile(f, []byte("key: [unclosed bracket"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	resolver := NewRemoteResolver(t.TempDir())
	ctx := context.Background()

	_, err := resolver.Resolve(ctx, "file://"+f)
	if err == nil {
		t.Fatal("expected error for invalid YAML file")
	}
	if !strings.Contains(err.Error(), "invalid YAML") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestResolveFile_ValidAbsolutePath(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "config.yaml")
	content := `version: 1
project: file-proj
tests:
  - name: t
    run: echo ok
`
	if err := os.WriteFile(f, []byte(content), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	resolver := NewRemoteResolver(t.TempDir())
	ctx := context.Background()

	data, err := resolver.Resolve(ctx, "file://"+f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(string(data), "file-proj") {
		t.Errorf("unexpected data: %s", data)
	}
}

// ─── readCache / writeCache roundtrip ─────────────────────────────────────────

func TestCacheRoundtrip(t *testing.T) {
	dir := t.TempDir()
	resolver := NewRemoteResolver(dir)
	cacheFile := filepath.Join(dir, "test_cache.yaml")

	body := []byte("version: 1\nproject: cached\n")
	etag := `"abc123"`
	lastMod := "Thu, 01 Jan 2026 00:00:00 GMT"

	if err := resolver.writeCache(cacheFile, body, etag, lastMod); err != nil {
		t.Fatalf("writeCache: %v", err)
	}

	gotBody, gotETag, gotLastMod, err := resolver.readCache(cacheFile)
	if err != nil {
		t.Fatalf("readCache: %v", err)
	}
	if string(gotBody) != string(body) {
		t.Errorf("body = %q, want %q", gotBody, body)
	}
	if gotETag != etag {
		t.Errorf("etag = %q, want %q", gotETag, etag)
	}
	if gotLastMod != lastMod {
		t.Errorf("lastMod = %q, want %q", gotLastMod, lastMod)
	}
}

func TestReadCache_FileNotFound(t *testing.T) {
	resolver := NewRemoteResolver(t.TempDir())
	_, _, _, err := resolver.readCache("/nonexistent/path/cache.yaml")
	if err == nil {
		t.Fatal("expected error for missing cache file")
	}
	if !os.IsNotExist(err) {
		t.Errorf("expected IsNotExist error, got: %v", err)
	}
}

func TestReadCache_CorruptedCache(t *testing.T) {
	dir := t.TempDir()
	resolver := NewRemoteResolver(dir)
	cacheFile := filepath.Join(dir, "corrupt.yaml")

	// Write content that is not a valid cacheEntry structure (corrupt YAML)
	if err := os.WriteFile(cacheFile, []byte("key: [unclosed"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	_, _, _, err := resolver.readCache(cacheFile)
	if err == nil {
		t.Fatal("expected error for corrupted cache")
	}
	if !strings.Contains(err.Error(), "unmarshaling cache entry") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestWriteCache_EmptyETagAndLastMod(t *testing.T) {
	dir := t.TempDir()
	resolver := NewRemoteResolver(dir)
	cacheFile := filepath.Join(dir, "empty_headers.yaml")

	body := []byte("project: test\n")
	if err := resolver.writeCache(cacheFile, body, "", ""); err != nil {
		t.Fatalf("writeCache: %v", err)
	}

	gotBody, gotETag, gotLastMod, err := resolver.readCache(cacheFile)
	if err != nil {
		t.Fatalf("readCache: %v", err)
	}
	if string(gotBody) != string(body) {
		t.Errorf("body = %q, want %q", gotBody, body)
	}
	if gotETag != "" {
		t.Errorf("etag = %q, want empty", gotETag)
	}
	if gotLastMod != "" {
		t.Errorf("lastMod = %q, want empty", gotLastMod)
	}
}

// ─── NewRemoteResolver with empty cacheDir ────────────────────────────────────

func TestNewRemoteResolver_EmptyCacheDir(t *testing.T) {
	r := NewRemoteResolver("")
	if r == nil {
		t.Fatal("expected non-nil resolver")
	}
	if r.cacheDir == "" {
		t.Error("expected cacheDir to be set to default")
	}
	if r.client == nil {
		t.Error("expected http client to be set")
	}
}

func TestNewRemoteResolver_CustomCacheDir(t *testing.T) {
	dir := t.TempDir()
	r := NewRemoteResolver(dir)
	if r.cacheDir != dir {
		t.Errorf("cacheDir = %q, want %q", r.cacheDir, dir)
	}
}

// ─── Resolve: unsupported scheme ─────────────────────────────────────────────

func TestResolve_UnsupportedScheme(t *testing.T) {
	resolver := NewRemoteResolver(t.TempDir())
	ctx := context.Background()

	_, err := resolver.Resolve(ctx, "ftp://example.com/config.yaml")
	if err == nil {
		t.Fatal("expected error for unsupported scheme")
	}
	if !strings.Contains(err.Error(), "unsupported URL scheme") {
		t.Errorf("unexpected error: %v", err)
	}
}

// ─── fetchHTTP: network error fallback, non-200 responses ────────────────────

func TestFetchHTTP_NetworkErrorNoCache(t *testing.T) {
	// Point at a server that immediately closes
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// close connection without sending anything
		hj, ok := w.(http.Hijacker)
		if !ok {
			http.Error(w, "no hijacker", 500)
			return
		}
		conn, _, _ := hj.Hijack()
		conn.Close()
	}))
	defer server.Close()

	resolver := NewRemoteResolver(t.TempDir())
	ctx := context.Background()

	_, err := resolver.Resolve(ctx, server.URL)
	if err == nil {
		t.Fatal("expected error for network failure with no cache")
	}
}

func TestFetchHTTP_NetworkErrorWithCache(t *testing.T) {
	remoteYAML := `version: 1
project: fallback-cached
tests:
  - name: t
    run: echo ok
`
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount == 1 {
			w.Header().Set("Content-Type", "text/yaml")
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, remoteYAML)
		} else {
			// Simulate connection failure by hijacking
			hj, ok := w.(http.Hijacker)
			if ok {
				conn, _, _ := hj.Hijack()
				conn.Close()
			}
		}
	}))
	defer server.Close()

	cacheDir := t.TempDir()
	resolver := NewRemoteResolver(cacheDir)
	ctx := context.Background()

	// First call populates cache
	firstData, err := resolver.Resolve(ctx, server.URL)
	if err != nil {
		t.Fatalf("first resolve failed: %v", err)
	}
	if !strings.Contains(string(firstData), "fallback-cached") {
		t.Errorf("unexpected first data: %s", firstData)
	}

	// Second call with network failure should return cached data
	secondData, err := resolver.Resolve(ctx, server.URL)
	if err != nil {
		t.Fatalf("second resolve (with fallback) failed: %v", err)
	}
	if !strings.Contains(string(secondData), "fallback-cached") {
		t.Errorf("expected cached data in second call, got: %s", secondData)
	}
}

func TestFetchHTTP_NonOKStatusNoCache(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	resolver := NewRemoteResolver(t.TempDir())
	ctx := context.Background()

	_, err := resolver.Resolve(ctx, server.URL)
	if err == nil {
		t.Fatal("expected error for HTTP 404 with no cache")
	}
	if !strings.Contains(err.Error(), "HTTP 404") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestFetchHTTP_NonOKStatusWithCache(t *testing.T) {
	remoteYAML := `version: 1
project: cached-on-error
tests:
  - name: t
    run: echo ok
`
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount == 1 {
			w.Header().Set("Content-Type", "text/yaml")
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, remoteYAML)
		} else {
			w.WriteHeader(http.StatusServiceUnavailable)
		}
	}))
	defer server.Close()

	cacheDir := t.TempDir()
	resolver := NewRemoteResolver(cacheDir)
	ctx := context.Background()

	// First call populates cache
	_, err := resolver.Resolve(ctx, server.URL)
	if err != nil {
		t.Fatalf("first resolve: %v", err)
	}

	// Second call: server returns 503, should fall back to cache
	data, err := resolver.Resolve(ctx, server.URL)
	if err != nil {
		t.Fatalf("second resolve with fallback: %v", err)
	}
	if !strings.Contains(string(data), "cached-on-error") {
		t.Errorf("expected cached data, got: %s", data)
	}
}

func TestFetchHTTP_InvalidYAMLResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/yaml")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "key: [unclosed bracket")
	}))
	defer server.Close()

	resolver := NewRemoteResolver(t.TempDir())
	ctx := context.Background()

	_, err := resolver.Resolve(ctx, server.URL)
	if err == nil {
		t.Fatal("expected error for invalid YAML response")
	}
	if !strings.Contains(err.Error(), "invalid YAML") {
		t.Errorf("unexpected error: %v", err)
	}
}

// ─── MergeConfigs edge cases ──────────────────────────────────────────────────

func TestMergeConfigs_OverlayOverridesProject(t *testing.T) {
	base := SmokeConfig{Project: "base-proj", Version: 1}
	overlay := SmokeConfig{Project: "overlay-proj"}
	merged := MergeConfigs(base, overlay)
	if merged.Project != "overlay-proj" {
		t.Errorf("project = %q, want overlay-proj", merged.Project)
	}
}

func TestMergeConfigs_BaseProjectPreservedWhenOverlayEmpty(t *testing.T) {
	base := SmokeConfig{Project: "base-proj", Version: 1}
	overlay := SmokeConfig{}
	merged := MergeConfigs(base, overlay)
	if merged.Project != "base-proj" {
		t.Errorf("project = %q, want base-proj", merged.Project)
	}
}

func TestMergeConfigs_OTelSettingsOverlay(t *testing.T) {
	base := SmokeConfig{
		Project: "base",
		OTel:    OTelConfig{Enabled: false, JaegerURL: "http://old:16686"},
	}
	overlay := SmokeConfig{
		OTel: OTelConfig{
			Enabled:    true,
			JaegerURL:  "http://new:16686",
			ServiceName: "mysvc",
		},
	}
	merged := MergeConfigs(base, overlay)
	if !merged.OTel.Enabled {
		t.Error("expected OTel.Enabled = true after overlay")
	}
	if merged.OTel.JaegerURL != "http://new:16686" {
		t.Errorf("JaegerURL = %q, want http://new:16686", merged.OTel.JaegerURL)
	}
	if merged.OTel.ServiceName != "mysvc" {
		t.Errorf("ServiceName = %q, want mysvc", merged.OTel.ServiceName)
	}
}

func TestMergeConfigs_MonorepoExcludeOverlay(t *testing.T) {
	base := SmokeConfig{
		Project: "base",
		Settings: Settings{MonorepoExclude: []string{"vendor"}},
	}
	overlay := SmokeConfig{
		Settings: Settings{MonorepoExclude: []string{"node_modules", ".git"}},
	}
	merged := MergeConfigs(base, overlay)
	if len(merged.Settings.MonorepoExclude) != 2 {
		t.Errorf("MonorepoExclude = %v, want 2 items", merged.Settings.MonorepoExclude)
	}
}

func TestMergeConfigs_ExportHeaders_Boost(t *testing.T) {
	base := SmokeConfig{Project: "base"}
	overlay := SmokeConfig{
		OTel: OTelConfig{
			ExportHeaders: map[string]string{"X-Team": "platform"},
		},
	}
	merged := MergeConfigs(base, overlay)
	if merged.OTel.ExportHeaders["X-Team"] != "platform" {
		t.Errorf("ExportHeaders = %v, want X-Team=platform", merged.OTel.ExportHeaders)
	}
}

// ─── loadWithDepthAndResolver: depth limit ───────────────────────────────────

func TestLoadWithDepthAndResolver_CircularIncludeDepthLimit(t *testing.T) {
	dir := t.TempDir()

	// Create file A that includes file B which includes file A (circular)
	fileA := filepath.Join(dir, "a.yaml")
	fileB := filepath.Join(dir, "b.yaml")

	contentA := fmt.Sprintf(`version: 1
project: a
includes:
  - %s
tests:
  - name: ta
    run: echo a
`, fileB)
	contentB := fmt.Sprintf(`version: 1
project: b
includes:
  - %s
tests:
  - name: tb
    run: echo b
`, fileA)

	if err := os.WriteFile(fileA, []byte(contentA), 0644); err != nil {
		t.Fatalf("write a: %v", err)
	}
	if err := os.WriteFile(fileB, []byte(contentB), 0644); err != nil {
		t.Fatalf("write b: %v", err)
	}

	_, err := Load(fileA)
	if err == nil {
		t.Fatal("expected error for circular include depth limit")
	}
	if !strings.Contains(err.Error(), "include depth exceeded") {
		t.Errorf("unexpected error: %v", err)
	}
}

// ─── Validate: notification edge cases ───────────────────────────────────────

func TestValidate_NotificationMissingURL(t *testing.T) {
	cfg := &SmokeConfig{
		Version: 1,
		Project: "test",
		Tests:   []Test{{Name: "t", Run: "true"}},
		Notifications: []Notification{
			{Format: "slack", On: "failure"},
		},
	}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected error for notification missing URL")
	}
	if !strings.Contains(err.Error(), "url is required") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidate_NotificationMissingFormat(t *testing.T) {
	cfg := &SmokeConfig{
		Version: 1,
		Project: "test",
		Tests:   []Test{{Name: "t", Run: "true"}},
		Notifications: []Notification{
			{URL: "https://hooks.slack.com/foo", On: "failure"},
		},
	}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected error for notification missing format")
	}
	if !strings.Contains(err.Error(), "format is required") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidate_NotificationInvalidFormat(t *testing.T) {
	cfg := &SmokeConfig{
		Version: 1,
		Project: "test",
		Tests:   []Test{{Name: "t", Run: "true"}},
		Notifications: []Notification{
			{URL: "https://hooks.slack.com/foo", Format: "teams"},
		},
	}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected error for invalid notification format")
	}
	if !strings.Contains(err.Error(), "format must be slack, pagerduty, or json") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidate_NotificationInvalidOn(t *testing.T) {
	cfg := &SmokeConfig{
		Version: 1,
		Project: "test",
		Tests:   []Test{{Name: "t", Run: "true"}},
		Notifications: []Notification{
			{URL: "https://hooks.slack.com/foo", Format: "slack", On: "never"},
		},
	}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected error for invalid notification 'on' value")
	}
	if !strings.Contains(err.Error(), "on must be failure, always, or change") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidate_NotificationValid(t *testing.T) {
	for _, format := range []string{"slack", "pagerduty", "json"} {
		for _, on := range []string{"", "failure", "always", "change"} {
			t.Run(format+"/"+on, func(t *testing.T) {
				cfg := &SmokeConfig{
					Version: 1,
					Project: "test",
					Tests:   []Test{{Name: "t", Run: "true"}},
					Notifications: []Notification{
						{URL: "https://hooks.example.com/foo", Format: format, On: on},
					},
				}
				if err := Validate(cfg); err != nil {
					t.Errorf("unexpected error for format=%s on=%s: %v", format, on, err)
				}
			})
		}
	}
}

// ─── Validate: k8s_resource ──────────────────────────────────────────────────

func TestValidate_K8sResourceMissingFields(t *testing.T) {
	tests := []struct {
		name    string
		check   *K8sResourceCheck
		wantMsg string
	}{
		{
			name:    "missing namespace",
			check:   &K8sResourceCheck{Kind: "Deployment", Name: "myapp"},
			wantMsg: "k8s_resource.namespace is required",
		},
		{
			name:    "missing kind",
			check:   &K8sResourceCheck{Namespace: "default", Name: "myapp"},
			wantMsg: "k8s_resource.kind is required",
		},
		{
			name:    "missing name",
			check:   &K8sResourceCheck{Namespace: "default", Kind: "Deployment"},
			wantMsg: "k8s_resource.name is required",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &SmokeConfig{
				Version: 1,
				Project: "test",
				Tests: []Test{{
					Name:   "k8s",
					Expect: Expect{K8sResource: tt.check},
				}},
			}
			err := Validate(cfg)
			if err == nil {
				t.Fatalf("expected error")
			}
			if !strings.Contains(err.Error(), tt.wantMsg) {
				t.Errorf("error = %q, want %q", err.Error(), tt.wantMsg)
			}
		})
	}
}

// ─── Validate: file_size ──────────────────────────────────────────────────────

func TestValidate_FileSizeMissingPath(t *testing.T) {
	cfg := &SmokeConfig{
		Version: 1,
		Project: "test",
		Tests: []Test{{
			Name:   "fs",
			Expect: Expect{FileSize: &FileSizeCheck{}},
		}},
	}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected error for file_size missing path")
	}
	if !strings.Contains(err.Error(), "file_size.path is required") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidate_FileSizeMinGreaterThanMax(t *testing.T) {
	minB := int64(1000)
	maxB := int64(500)
	cfg := &SmokeConfig{
		Version: 1,
		Project: "test",
		Tests: []Test{{
			Name: "fs",
			Expect: Expect{FileSize: &FileSizeCheck{
				Path:     "/tmp/file.bin",
				MinBytes: &minB,
				MaxBytes: &maxB,
			}},
		}},
	}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected error for min_bytes > max_bytes")
	}
	if !strings.Contains(err.Error(), "min_bytes must be <= max_bytes") {
		t.Errorf("unexpected error: %v", err)
	}
}

// ─── Validate: doc_integrity ──────────────────────────────────────────────────

func TestValidate_DocIntegrityMissingBinary(t *testing.T) {
	cfg := &SmokeConfig{
		Version: 1,
		Project: "test",
		Tests: []Test{{
			Name: "doc",
			Expect: Expect{DocIntegrity: &DocIntegrityCheck{
				Docs: []string{"README.md"},
			}},
		}},
	}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected error for doc_integrity missing binary")
	}
	if !strings.Contains(err.Error(), "doc_integrity.binary is required") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidate_DocIntegrityMissingDocs(t *testing.T) {
	cfg := &SmokeConfig{
		Version: 1,
		Project: "test",
		Tests: []Test{{
			Name: "doc",
			Expect: Expect{DocIntegrity: &DocIntegrityCheck{
				Binary: "mycli",
				Docs:   []string{},
			}},
		}},
	}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected error for doc_integrity missing docs")
	}
	if !strings.Contains(err.Error(), "doc_integrity.docs is required") {
		t.Errorf("unexpected error: %v", err)
	}
}

// ─── Validate: ping / kafka / ldap / mqtt ────────────────────────────────────

func TestValidate_PingMissingHost(t *testing.T) {
	cfg := &SmokeConfig{
		Version: 1,
		Project: "test",
		Tests: []Test{{
			Name:   "ping",
			Expect: Expect{Ping: &PingCheck{}},
		}},
	}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected error for ping missing host")
	}
	if !strings.Contains(err.Error(), "ping.host is required") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidate_KafkaMissingBrokers(t *testing.T) {
	cfg := &SmokeConfig{
		Version: 1,
		Project: "test",
		Tests: []Test{{
			Name:   "kafka",
			Expect: Expect{Kafka: &KafkaCheck{}},
		}},
	}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected error for kafka missing brokers")
	}
	if !strings.Contains(err.Error(), "kafka_broker.brokers is required") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidate_LDAPMissingHost(t *testing.T) {
	cfg := &SmokeConfig{
		Version: 1,
		Project: "test",
		Tests: []Test{{
			Name:   "ldap",
			Expect: Expect{LDAP: &LDAPCheck{}},
		}},
	}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected error for ldap missing host")
	}
	if !strings.Contains(err.Error(), "ldap_bind.host is required") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidate_MQTTMissingBroker(t *testing.T) {
	cfg := &SmokeConfig{
		Version: 1,
		Project: "test",
		Tests: []Test{{
			Name:   "mqtt",
			Expect: Expect{MQTT: &MQTTCheck{}},
		}},
	}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected error for mqtt missing broker")
	}
	if !strings.Contains(err.Error(), "mqtt_ping.broker is required") {
		t.Errorf("unexpected error: %v", err)
	}
}

// ─── Validate: OTelTrace min_spans negative ───────────────────────────────────

func TestValidate_OTelTrace_NegativeMinSpans_Boost(t *testing.T) {
	cfg := &SmokeConfig{
		Version: 1,
		Project: "test",
		Tests: []Test{{
			Name: "t",
			Expect: Expect{
				OTelTrace: &OTelTraceCheck{JaegerURL: "http://localhost:16686", MinSpans: -1},
			},
		}},
	}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected error for negative min_spans")
	}
	if !strings.Contains(err.Error(), "min_spans must be >= 0") {
		t.Errorf("unexpected error: %v", err)
	}
}

// ─── Validate: OTelJaegerURL invalid prefix ───────────────────────────────────

func TestValidate_OTelEnabledInvalidJaegerURLPrefix(t *testing.T) {
	cfg := &SmokeConfig{
		Version: 1,
		Project: "test",
		OTel:    OTelConfig{Enabled: true, JaegerURL: "ftp://jaeger:16686"},
		Tests:   []Test{{Name: "t", Run: "true"}},
	}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected error for otel jaeger_url with invalid prefix")
	}
	if !strings.Contains(err.Error(), "otel.jaeger_url must start with http") {
		t.Errorf("unexpected error: %v", err)
	}
}

// ─── Validate: Retry edge cases ───────────────────────────────────────────────

func TestValidate_RetryCountZero_Boost(t *testing.T) {
	cfg := &SmokeConfig{
		Version: 1,
		Project: "test",
		Tests: []Test{{
			Name: "t",
			Run:  "echo ok",
			Retry: &RetryPolicy{
				Count:   0,
				Backoff: Duration{Duration: time.Second},
			},
		}},
	}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected error for retry count = 0")
	}
	if !strings.Contains(err.Error(), "retry.count must be >= 1") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidate_RetryBackoffZero_Boost(t *testing.T) {
	cfg := &SmokeConfig{
		Version: 1,
		Project: "test",
		Tests: []Test{{
			Name: "t",
			Run:  "echo ok",
			Retry: &RetryPolicy{
				Count:   3,
				Backoff: Duration{Duration: 0},
			},
		}},
	}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected error for retry backoff = 0")
	}
	if !strings.Contains(err.Error(), "retry.backoff must be > 0") {
		t.Errorf("unexpected error: %v", err)
	}
}
