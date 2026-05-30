package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/CosmoLabs-org/SmokeSig/internal/schema"
)

func intPtrB5(n int) *int    { return &n }
func strPtrB5(s string) *string { return &s }

// --- runTestOnce uncovered branches: assertion dispatch through Runner.Run ---

func boost5Runner(t *testing.T, tests []schema.Test) []TestResult {
	t.Helper()
	cfg := &schema.SmokeConfig{Tests: tests}
	r := &Runner{Config: cfg, ConfigDir: t.TempDir(), Reporter: &noopReporter{}}
	return boost5RunWithRunner(r)
}

func boost5RunWithRunner(r *Runner) []TestResult {
	suite, _ := r.Run(RunOptions{})
	if suite == nil {
		return nil
	}
	return suite.Tests
}

func TestBoost5_runTestOnce_Credential_EnvSource(t *testing.T) {
	t.Setenv("TEST_CRED_BOOST5", "secret-value")
	results := boost5Runner(t, []schema.Test{{
		Name: "cred-env",
		Expect: schema.Expect{
			Credential: &schema.CredentialCheck{Source: "env", Name: "TEST_CRED_BOOST5"},
		},
	}})
	if len(results) == 0 || !results[0].Passed {
		t.Error("expected credential check to pass")
	}
}

func TestBoost5_runTestOnce_Credential_EnvMissing(t *testing.T) {
	results := boost5Runner(t, []schema.Test{{
		Name: "cred-missing",
		Expect: schema.Expect{
			Credential: &schema.CredentialCheck{Source: "env", Name: "NONEXISTENT_CRED_BOOST5_XYZ"},
		},
	}})
	if len(results) == 0 || results[0].Passed {
		t.Error("expected credential check to fail for missing env")
	}
}

func TestBoost5_runTestOnce_Credential_FileSource(t *testing.T) {
	dir := t.TempDir()
	credFile := filepath.Join(dir, "cred.txt")
	os.WriteFile(credFile, []byte("my-secret"), 0600)

	cfg := &schema.SmokeConfig{Tests: []schema.Test{{
		Name: "cred-file",
		Expect: schema.Expect{
			Credential: &schema.CredentialCheck{Source: "file", Name: credFile},
		},
	}}}
	r := &Runner{Config: cfg, ConfigDir: dir, Reporter: &noopReporter{}}
	results := boost5RunWithRunner(r)
	if len(results) == 0 || !results[0].Passed {
		t.Error("expected file credential check to pass")
	}
}

func TestBoost5_runTestOnce_Credential_FileSource_Contains(t *testing.T) {
	dir := t.TempDir()
	credFile := filepath.Join(dir, "cred.txt")
	os.WriteFile(credFile, []byte("contains-this-value"), 0600)

	cfg := &schema.SmokeConfig{Tests: []schema.Test{{
		Name: "cred-file-contains",
		Expect: schema.Expect{
			Credential: &schema.CredentialCheck{Source: "file", Name: credFile, Contains: "this-value"},
		},
	}}}
	r := &Runner{Config: cfg, ConfigDir: dir, Reporter: &noopReporter{}}
	results := boost5RunWithRunner(r)
	if len(results) == 0 || !results[0].Passed {
		t.Error("expected file credential check with contains to pass")
	}
}

func TestBoost5_runTestOnce_DNS(t *testing.T) {
	results := boost5Runner(t, []schema.Test{{
		Name: "dns-check",
		Expect: schema.Expect{
			DNS: &schema.DNSCheck{Hostname: "localhost", RecordType: "A"},
		},
	}})
	if len(results) == 0 || !results[0].Passed {
		t.Error("expected DNS resolve of localhost to pass")
	}
}

func TestBoost5_runTestOnce_DNS_Fail(t *testing.T) {
	results := boost5Runner(t, []schema.Test{{
		Name: "dns-fail",
		Expect: schema.Expect{
			DNS: &schema.DNSCheck{Hostname: "this-host-definitely-does-not-exist-smokesig.invalid", RecordType: "A"},
		},
	}})
	if len(results) == 0 || results[0].Passed {
		t.Error("expected DNS check for invalid host to fail")
	}
}

func TestBoost5_runTestOnce_ServiceReachable_Fail(t *testing.T) {
	results := boost5Runner(t, []schema.Test{{
		Name: "svc-unreachable",
		Expect: schema.Expect{
			ServiceReachable: &schema.ServiceReachableCheck{
				URL:     "http://127.0.0.1:19999",
				Timeout: schema.Duration{Duration: 200 * time.Millisecond},
			},
		},
	}})
	if len(results) == 0 || results[0].Passed {
		t.Error("expected service reachable to fail for closed port")
	}
}

func TestBoost5_runTestOnce_ServiceReachable_Pass(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer srv.Close()

	results := boost5Runner(t, []schema.Test{{
		Name: "svc-ok",
		Expect: schema.Expect{
			ServiceReachable: &schema.ServiceReachableCheck{URL: srv.URL},
		},
	}})
	if len(results) == 0 || !results[0].Passed {
		t.Error("expected service reachable to pass")
	}
}

func TestBoost5_runTestOnce_SSLCert_BadHost(t *testing.T) {
	results := boost5Runner(t, []schema.Test{{
		Name: "ssl-bad",
		Expect: schema.Expect{
			SSLCert: &schema.SSLCertCheck{Host: "127.0.0.1", Port: 19999, MinDaysRemaining: 1},
		},
	}})
	if len(results) == 0 || results[0].Passed {
		t.Error("expected SSL cert check on closed port to fail")
	}
}

func TestBoost5_runTestOnce_DockerContainer_NotRunning(t *testing.T) {
	results := boost5Runner(t, []schema.Test{{
		Name: "docker-container",
		Expect: schema.Expect{
			DockerContainer: &schema.DockerContainerCheck{Name: "smokesig-nonexistent-container-boost5"},
		},
	}})
	if len(results) == 0 || results[0].Passed {
		t.Error("expected docker container check to fail for nonexistent container")
	}
}

func TestBoost5_runTestOnce_DockerImage_Missing(t *testing.T) {
	results := boost5Runner(t, []schema.Test{{
		Name: "docker-image",
		Expect: schema.Expect{
			DockerImage: &schema.DockerImageCheck{Image: "smokesig-nonexistent-image-boost5:latest"},
		},
	}})
	if len(results) == 0 || results[0].Passed {
		t.Error("expected docker image check to fail for missing image")
	}
}

func TestBoost5_runTestOnce_AllowFailure(t *testing.T) {
	results := boost5Runner(t, []schema.Test{{
		Name:         "allowed-fail",
		Run:          "exit 1",
		AllowFailure: true,
		Expect: schema.Expect{
			ExitCode: intPtrB5(0),
		},
	}})
	if len(results) == 0 {
		t.Fatal("expected result")
	}
	if results[0].Passed {
		t.Error("should not be marked as passed")
	}
	if !results[0].AllowedFailure {
		t.Error("should be marked as allowed failure")
	}
}

func TestBoost5_runTestOnce_Cleanup(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(dir, "cleanup-ran.txt")

	cfg := &schema.SmokeConfig{Tests: []schema.Test{{
		Name:    "with-cleanup",
		Run:     "echo hello",
		Cleanup: fmt.Sprintf("touch %s", marker),
		Expect: schema.Expect{
			ExitCode: intPtrB5(0),
		},
	}}}
	r := &Runner{Config: cfg, ConfigDir: dir, Reporter: &noopReporter{}}
	r.Run(RunOptions{})

	if _, err := os.Stat(marker); os.IsNotExist(err) {
		t.Error("cleanup command did not run")
	}
}

func TestBoost5_runTestOnce_VarsResolveTemplate(t *testing.T) {
	dir := t.TempDir()

	cfg := &schema.SmokeConfig{Tests: []schema.Test{
		{
			Name: "set-var",
			Run:  `echo '{"greeting": "hello-world"}'`,
			Expect: schema.Expect{
				JSONField: &schema.JSONFieldCheck{
					Path:    "greeting",
					Equals:  "hello-world",
					Extract: "greeting",
				},
			},
		},
		{
			Name: "use-var-template",
			Run:  `echo "{{ .Vars.greeting }}"`,
			Expect: schema.Expect{
				StdoutContains: "hello-world",
			},
		},
	}}
	r := &Runner{Config: cfg, ConfigDir: dir, Reporter: &noopReporter{}}
	results := boost5RunWithRunner(r)
	if len(results) < 2 {
		t.Fatal("expected 2 results")
	}
	if !results[1].Passed {
		t.Errorf("expected vars template resolution to work, assertions: %+v", results[1].Assertions)
	}
}

func TestBoost5_runTestOnce_ProcessExtracts_JSON(t *testing.T) {
	dir := t.TempDir()
	vs := NewVarStore()

	cfg := &schema.SmokeConfig{Tests: []schema.Test{
		{
			Name: "extract-json",
			Run:  `echo '{"id": "abc123"}'`,
			Expect: schema.Expect{
				JSONField: &schema.JSONFieldCheck{
					Path:    "id",
					Equals:  "abc123",
					Extract: "captured_id",
				},
			},
		},
		{
			Name: "use-extracted",
			Run:  "echo {{ .Vars.captured_id }}",
			Expect: schema.Expect{
				StdoutContains: "abc123",
			},
		},
	}}
	r := &Runner{Config: cfg, ConfigDir: dir, Reporter: &noopReporter{}, Vars: vs}
	results := boost5RunWithRunner(r)
	if len(results) < 2 {
		t.Fatal("expected 2 results")
	}
	if !results[0].Passed {
		t.Error("extract test should pass")
	}
	if !results[1].Passed {
		t.Error("chained test should see extracted var")
	}
}

func TestBoost5_runTestOnce_ProcessExtracts_Regex(t *testing.T) {
	dir := t.TempDir()
	vs := NewVarStore()

	cfg := &schema.SmokeConfig{Tests: []schema.Test{
		{
			Name: "extract-regex",
			Run:  `echo "version: 1.2.3"`,
			Expect: schema.Expect{
				StdoutMatches: `version: (\d+\.\d+\.\d+)`,
				Extract:       "version",
			},
		},
		{
			Name: "use-regex-extract",
			Run:  "echo {{ .Vars.version }}",
			Expect: schema.Expect{
				StdoutContains: "1.2.3",
			},
		},
	}}
	r := &Runner{Config: cfg, ConfigDir: dir, Reporter: &noopReporter{}, Vars: vs}
	results := boost5RunWithRunner(r)
	if len(results) < 2 {
		t.Fatal("expected 2 results")
	}
	if !results[1].Passed {
		t.Errorf("chained regex test should pass, got assertions: %+v", results[1].Assertions)
	}
}

func TestBoost5_runTestOnce_GraphQL_Introspection(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"data": map[string]interface{}{
				"__schema": map[string]interface{}{
					"types": []map[string]string{{"name": "Query"}},
				},
			},
		})
	}))
	defer srv.Close()

	results := boost5Runner(t, []schema.Test{{
		Name: "graphql",
		Expect: schema.Expect{
			GraphQL: &schema.GraphQLCheck{
				URL:        srv.URL,
				StatusCode: intPtrB5(200),
			},
		},
	}})
	if len(results) == 0 || !results[0].Passed {
		t.Errorf("expected GraphQL introspection to pass, got %+v", results)
	}
}

func TestBoost5_runTestOnce_OTelTrace_Via_Runner(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"data": []map[string]interface{}{
				{"spans": []map[string]string{{"spanID": "s1"}}},
			},
		})
	}))
	defer srv.Close()

	cfg := &schema.SmokeConfig{
		Tests: []schema.Test{{
			Name: "otel-via-runner",
			Expect: schema.Expect{
				OTelTrace: &schema.OTelTraceCheck{
					JaegerURL:   srv.URL,
					ServiceName: "test-svc",
					MinSpans:    1,
					Timeout:     schema.Duration{Duration: 1 * time.Second},
				},
			},
		}},
	}
	r := &Runner{Config: cfg, ConfigDir: t.TempDir(), Reporter: &noopReporter{}}
	results := boost5RunWithRunner(r)
	if len(results) == 0 || !results[0].Passed {
		t.Error("expected OTel trace check via runner to pass")
	}
}

// --- CheckProcessRunning ---

func TestBoost5_CheckProcessRunning_EmptyName(t *testing.T) {
	result := CheckProcessRunning("")
	if result.Passed {
		t.Error("empty name should fail")
	}
	if result.Actual != "empty name" {
		t.Errorf("expected 'empty name' actual, got %q", result.Actual)
	}
}

func TestBoost5_CheckProcessRunning_CurrentProcess(t *testing.T) {
	result := CheckProcessRunning("go")
	// The test runner IS a Go process, but pgrep -x "go" may or may not match
	// depending on the binary name. Just verify it completes without panic.
	if result.Type != "process_running" {
		t.Errorf("expected type process_running, got %q", result.Type)
	}
}

func TestBoost5_CheckProcessRunning_NonexistentProcess(t *testing.T) {
	result := CheckProcessRunning("smokesig_nonexistent_process_xyz123")
	if result.Passed {
		t.Error("nonexistent process should not pass")
	}
	if result.Actual != "not found" {
		t.Errorf("expected 'not found' actual, got %q", result.Actual)
	}
}

// --- Simulator assertions (pure parse functions) ---

func TestBoost5_parseSimctlOutput_BootedWithNameFilter(t *testing.T) {
	data := `{"devices":{"com.apple.CoreSimulator.SimRuntime.iOS-17-4":[{"name":"iPhone 15","udid":"ABC","state":"Booted"},{"name":"iPad","udid":"DEF","state":"Shutdown"}]}}`
	found, desc := parseSimctlOutput([]byte(data), "iPhone", "")
	if !found {
		t.Error("expected to find booted iPhone")
	}
	if desc == "" {
		t.Error("expected non-empty description")
	}
}

func TestBoost5_parseSimctlOutput_OSFilter(t *testing.T) {
	data := `{"devices":{"com.apple.CoreSimulator.SimRuntime.iOS-17-4":[{"name":"iPhone 15","udid":"ABC","state":"Booted"}],"com.apple.CoreSimulator.SimRuntime.iOS-16-0":[{"name":"iPhone 14","udid":"DEF","state":"Booted"}]}}`
	found, _ := parseSimctlOutput([]byte(data), "", "17.4")
	if !found {
		t.Error("expected to find device with OS 17.4")
	}

	found2, _ := parseSimctlOutput([]byte(data), "", "15.0")
	if found2 {
		t.Error("should not find device with OS 15.0")
	}
}

func TestBoost5_parseSimctlOutput_InvalidJSON(t *testing.T) {
	found, desc := parseSimctlOutput([]byte("not json"), "", "")
	if found {
		t.Error("invalid JSON should not find devices")
	}
	if desc == "" {
		t.Error("expected error description")
	}
}

func TestBoost5_parseSimctlOutput_NoBooted(t *testing.T) {
	data := `{"devices":{"com.apple.CoreSimulator.SimRuntime.iOS-17-4":[{"name":"iPhone 15","udid":"ABC","state":"Shutdown"}]}}`
	found, desc := parseSimctlOutput([]byte(data), "", "")
	if found {
		t.Error("no booted device should return false")
	}
	if desc != "no booted simulator found" {
		t.Errorf("expected 'no booted simulator found', got %q", desc)
	}
}

func TestBoost5_formatIOSExpected_AllFilters(t *testing.T) {
	check := &schema.IOSSimulatorCheck{DeviceName: "iPhone 15", OS: "17.4"}
	expected := formatIOSExpected(check)
	if expected != `booted iOS simulator, name="iPhone 15", os="17.4"` {
		t.Errorf("unexpected format: %s", expected)
	}
}

func TestBoost5_formatAndroidExpected_WithSerial(t *testing.T) {
	check := &schema.AndroidEmulatorCheck{Serial: "emulator-5554"}
	expected := formatAndroidExpected(check)
	if expected != "android emulator ready (serial=emulator-5554)" {
		t.Errorf("unexpected format: %s", expected)
	}
}

func TestBoost5_formatAndroidExpected_NoSerial(t *testing.T) {
	check := &schema.AndroidEmulatorCheck{}
	expected := formatAndroidExpected(check)
	if expected != "android emulator ready" {
		t.Errorf("unexpected format: %s", expected)
	}
}

// --- Lifecycle hooks: WaitForPort, AlwaysRun, foreground EnvPass ---

func TestBoost5_RunLifecycleHooks_WaitForPort_Success(t *testing.T) {
	// Start a listener that the hook can wait for
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	defer ln.Close()

	hooks := []schema.LifecycleHook{{
		Command:     "sleep 0.1",
		Background:  true,
		WaitForPort: port,
		Timeout:     schema.Duration{Duration: 5 * time.Second},
	}}
	ctx := context.Background()
	_, err = RunLifecycleHooks(ctx, hooks, nil, t.TempDir())
	if err != nil {
		t.Errorf("expected success since port is already listening, got: %v", err)
	}
	CleanupBackgroundProcesses()
}

func TestBoost5_RunLifecycleHooks_WaitForPort_Timeout(t *testing.T) {
	hooks := []schema.LifecycleHook{{
		Command:        "sleep 10",
		Background:     true,
		WaitForPort:    19998,
		Timeout:        schema.Duration{Duration: 500 * time.Millisecond},
		StartupTimeout: schema.Duration{Duration: 300 * time.Millisecond},
	}}
	ctx := context.Background()
	_, err := RunLifecycleHooks(ctx, hooks, nil, t.TempDir())
	if err == nil {
		t.Error("expected timeout error when port never opens")
	}
	CleanupBackgroundProcesses()
}

func TestBoost5_RunLifecycleHooks_AlwaysRun_AfterError(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(dir, "always-ran.txt")

	hooks := []schema.LifecycleHook{
		{Command: "exit 1"},
		{Command: "echo skipped"},
		{Command: fmt.Sprintf("touch %s", marker), AlwaysRun: true},
	}
	ctx := context.Background()
	_, err := RunLifecycleHooks(ctx, hooks, nil, dir)
	if err == nil {
		t.Error("expected error from first hook")
	}
	if _, statErr := os.Stat(marker); os.IsNotExist(statErr) {
		t.Error("AlwaysRun hook should have executed despite prior error")
	}
}

func TestBoost5_RunLifecycleHooks_EmptyCommand_Skipped(t *testing.T) {
	hooks := []schema.LifecycleHook{
		{Command: ""},
		{Command: "echo ok"},
	}
	ctx := context.Background()
	_, err := RunLifecycleHooks(ctx, hooks, nil, t.TempDir())
	if err != nil {
		t.Errorf("empty command should be skipped, got: %v", err)
	}
}

func TestBoost5_RunLifecycleHooks_Foreground_EnvPass(t *testing.T) {
	hooks := []schema.LifecycleHook{{
		Command: `echo "MY_KEY=my_value"`,
		EnvPass: true,
	}}
	ctx := context.Background()
	env, err := RunLifecycleHooks(ctx, hooks, nil, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if env["MY_KEY"] != "my_value" {
		t.Errorf("expected MY_KEY=my_value, got %q", env["MY_KEY"])
	}
}

// --- OTel backends: honeycomb and datadog queryTrace ---

func TestBoost5_honeycombBackend_queryTrace(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Honeycomb-Team") == "" {
			t.Error("expected X-Honeycomb-Team header")
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"data": map[string]interface{}{
				"spans": []map[string]string{{"id": "s1"}, {"id": "s2"}},
			},
		})
	}))
	defer srv.Close()

	b := &honeycombBackend{baseURL: srv.URL, apiKey: "test-key"}
	count, err := b.queryTrace(srv.Client(), "trace-123")
	if err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Errorf("expected 2 spans, got %d", count)
	}
}

func TestBoost5_honeycombBackend_queryTrace_InvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("not json"))
	}))
	defer srv.Close()

	b := &honeycombBackend{baseURL: srv.URL, apiKey: "test-key"}
	_, err := b.queryTrace(srv.Client(), "trace-123")
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestBoost5_datadogBackend_queryTrace(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("DD-API-KEY") == "" {
			t.Error("expected DD-API-KEY header")
		}
		if r.Header.Get("DD-APPLICATION-KEY") == "" {
			t.Error("expected DD-APPLICATION-KEY header")
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"traces": []map[string]interface{}{
				{"spans": []map[string]string{{"id": "s1"}}},
				{"spans": []map[string]string{{"id": "s2"}, {"id": "s3"}}},
			},
		})
	}))
	defer srv.Close()

	b := &datadogBackend{baseURL: srv.URL, apiKey: "dd-key", appKey: "dd-app"}
	count, err := b.queryTrace(srv.Client(), "trace-456")
	if err != nil {
		t.Fatal(err)
	}
	if count != 3 {
		t.Errorf("expected 3 spans across 2 traces, got %d", count)
	}
}

func TestBoost5_datadogBackend_queryTrace_EmptyTraces(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{"traces": []interface{}{}})
	}))
	defer srv.Close()

	b := &datadogBackend{baseURL: srv.URL, apiKey: "dd-key"}
	count, err := b.queryTrace(srv.Client(), "trace-456")
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Errorf("expected 0 spans for empty traces, got %d", count)
	}
}

func TestBoost5_datadogBackend_queryTrace_NoAppKey(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("DD-APPLICATION-KEY") != "" {
			t.Error("should not set DD-APPLICATION-KEY when appKey is empty")
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"traces": []map[string]interface{}{
				{"spans": []map[string]string{{"id": "s1"}}},
			},
		})
	}))
	defer srv.Close()

	b := &datadogBackend{baseURL: srv.URL, apiKey: "dd-key", appKey: ""}
	count, err := b.queryTrace(srv.Client(), "trace-789")
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Errorf("expected 1 span, got %d", count)
	}
}

func TestBoost5_datadogBackend_queryTrace_InvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("bad json"))
	}))
	defer srv.Close()

	b := &datadogBackend{baseURL: srv.URL, apiKey: "dd-key"}
	_, err := b.queryTrace(srv.Client(), "trace-789")
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

// --- doc_integrity: parseCommandName, checkDocExamples ---

func TestBoost5_parseCommandName_FlagLine(t *testing.T) {
	result := parseCommandName("-v, --verbose")
	if result != "" {
		t.Errorf("flag line should return empty, got %q", result)
	}
}

func TestBoost5_parseCommandName_EmptyLine(t *testing.T) {
	result := parseCommandName("")
	if result != "" {
		t.Errorf("empty line should return empty, got %q", result)
	}
}

func TestBoost5_parseCommandName_ValidCommand(t *testing.T) {
	result := parseCommandName("  run-tests   Execute the test suite")
	if result != "run-tests" {
		t.Errorf("expected 'run-tests', got %q", result)
	}
}

func TestBoost5_parseCommandName_InvalidStartChar(t *testing.T) {
	result := parseCommandName("  123invalid  Some description")
	if result != "" {
		t.Errorf("number-starting name should return empty, got %q", result)
	}
}

func TestBoost5_checkDocExamples_PassingExample(t *testing.T) {
	dir := t.TempDir()
	docPath := filepath.Join(dir, "README.md")
	os.WriteFile(docPath, []byte("# Test\n```bash\necho hello\n```\n"), 0644)

	failures := checkDocExamples([]string{docPath}, "echo", dir, 5*time.Second)
	if len(failures) != 0 {
		t.Errorf("expected no failures, got %v", failures)
	}
}

func TestBoost5_checkDocExamples_FailingExample(t *testing.T) {
	dir := t.TempDir()
	docPath := filepath.Join(dir, "README.md")
	os.WriteFile(docPath, []byte("# Test\n```bash\nfake-binary-boost5 --help\n```\n"), 0644)

	failures := checkDocExamples([]string{docPath}, "fake-binary-boost5", dir, 5*time.Second)
	if len(failures) == 0 {
		t.Error("expected failure for nonexistent binary")
	}
}

func TestBoost5_checkDocExamples_MissingDocFile(t *testing.T) {
	dir := t.TempDir()
	failures := checkDocExamples([]string{"/nonexistent/doc.md"}, "echo", dir, 5*time.Second)
	if len(failures) != 0 {
		t.Error("missing doc file should be skipped, not fail")
	}
}

func TestBoost5_checkDocExamples_RelativePath(t *testing.T) {
	dir := t.TempDir()
	docPath := filepath.Join(dir, "docs", "USAGE.md")
	os.MkdirAll(filepath.Join(dir, "docs"), 0755)
	os.WriteFile(docPath, []byte("# Usage\n```bash\necho works\n```\n"), 0644)

	failures := checkDocExamples([]string{"docs/USAGE.md"}, "echo", dir, 5*time.Second)
	if len(failures) != 0 {
		t.Errorf("relative path should resolve correctly, got %v", failures)
	}
}

func TestBoost5_checkDocExamples_LongCommandTruncated(t *testing.T) {
	dir := t.TempDir()
	// Build a long command string that exceeds the 60-char truncation threshold
	longArg := ""
	for i := 0; i < 20; i++ {
		longArg += "arg" + fmt.Sprintf("%d", i) + " "
	}
	longCmd := "fake-binary-boost5 " + longArg
	docPath := filepath.Join(dir, "README.md")
	os.WriteFile(docPath, []byte(fmt.Sprintf("# Test\n```bash\n%s\n```\n", longCmd)), 0644)

	failures := checkDocExamples([]string{docPath}, "fake-binary-boost5", dir, 5*time.Second)
	if len(failures) == 0 {
		t.Error("expected failure for nonexistent binary in example")
	}
}

// --- queryJSON error paths ---

func TestBoost5_queryJSON_NonOKStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	defer srv.Close()

	_, err := queryJSON(srv.Client(), srv.URL, nil)
	if err == nil {
		t.Error("expected error for non-200 status")
	}
}

func TestBoost5_queryJSON_WithHeaders(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Custom") != "value" {
			t.Error("expected custom header")
		}
		w.Write([]byte(`{"ok": true}`))
	}))
	defer srv.Close()

	result, err := queryJSON(srv.Client(), srv.URL, map[string]string{"X-Custom": "value"})
	if err != nil {
		t.Fatal(err)
	}
	if result == nil {
		t.Error("expected non-nil result")
	}
}

// --- CheckSSLCert: self-signed allowed ---

func TestBoost5_CheckSSLCert_SelfSignedAllowed(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer srv.Close()

	port := srv.Listener.Addr().(*net.TCPAddr).Port
	result := CheckSSLCert(&schema.SSLCertCheck{
		Host:            "127.0.0.1",
		Port:            port,
		AllowSelfSigned: true,
	})
	if !result.Passed {
		t.Errorf("self-signed with AllowSelfSigned should pass, got: %s", result.Actual)
	}
}

func TestBoost5_CheckSSLCert_SelfSignedNotAllowed(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer srv.Close()

	port := srv.Listener.Addr().(*net.TCPAddr).Port
	result := CheckSSLCert(&schema.SSLCertCheck{
		Host:            "127.0.0.1",
		Port:            port,
		AllowSelfSigned: false,
	})
	if result.Passed {
		t.Error("self-signed without AllowSelfSigned should fail")
	}
}

// --- More runTestOnce assertion dispatch branches ---

func TestBoost5_runTestOnce_SMTP_Fail(t *testing.T) {
	results := boost5Runner(t, []schema.Test{{
		Name: "smtp-fail",
		Expect: schema.Expect{
			SMTP: &schema.SMTPCheck{
				Host:    "127.0.0.1",
				Port:    19997,
				Timeout: schema.Duration{Duration: 200 * time.Millisecond},
			},
		},
	}})
	if len(results) == 0 || results[0].Passed {
		t.Error("expected SMTP check to fail on closed port")
	}
}

func TestBoost5_runTestOnce_Ping_Localhost(t *testing.T) {
	results := boost5Runner(t, []schema.Test{{
		Name: "ping-local",
		Expect: schema.Expect{
			Ping: &schema.PingCheck{
				Host:    "127.0.0.1",
				Count:   1,
				Timeout: schema.Duration{Duration: 2 * time.Second},
			},
		},
	}})
	if len(results) == 0 || !results[0].Passed {
		t.Error("expected ping localhost to pass")
	}
}

func TestBoost5_runTestOnce_NTP(t *testing.T) {
	results := boost5Runner(t, []schema.Test{{
		Name: "ntp-check",
		Expect: schema.Expect{
			NTP: &schema.NTPCheck{
				Server:      "127.0.0.1",
				MaxOffsetMs: 99999,
				Timeout:     schema.Duration{Duration: 500 * time.Millisecond},
			},
		},
	}})
	// NTP may fail if no server, but exercises the code path
	if len(results) == 0 {
		t.Error("expected result")
	}
}

func TestBoost5_runTestOnce_Mongo_Fail(t *testing.T) {
	results := boost5Runner(t, []schema.Test{{
		Name: "mongo-fail",
		Expect: schema.Expect{
			Mongo: &schema.MongoCheck{Host: "127.0.0.1", Port: 19996},
		},
	}})
	if len(results) == 0 || results[0].Passed {
		t.Error("expected Mongo check to fail on closed port")
	}
}

func TestBoost5_runTestOnce_MQTT_Fail(t *testing.T) {
	results := boost5Runner(t, []schema.Test{{
		Name: "mqtt-fail",
		Expect: schema.Expect{
			MQTT: &schema.MQTTCheck{
				Broker:  "127.0.0.1:19995",
				Timeout: schema.Duration{Duration: 200 * time.Millisecond},
			},
		},
	}})
	if len(results) == 0 || results[0].Passed {
		t.Error("expected MQTT check to fail on closed port")
	}
}

func TestBoost5_runTestOnce_FileSize(t *testing.T) {
	dir := t.TempDir()
	testFile := filepath.Join(dir, "testfile.txt")
	os.WriteFile(testFile, []byte("hello world"), 0644)

	minB := int64(5)
	maxB := int64(100)
	cfg := &schema.SmokeConfig{Tests: []schema.Test{{
		Name: "filesize",
		Expect: schema.Expect{
			FileSize: &schema.FileSizeCheck{
				Path:     "testfile.txt",
				MinBytes: &minB,
				MaxBytes: &maxB,
			},
		},
	}}}
	r := &Runner{Config: cfg, ConfigDir: dir, Reporter: &noopReporter{}}
	results := boost5RunWithRunner(r)
	if len(results) == 0 || !results[0].Passed {
		t.Error("expected file size check to pass")
	}
}

// --- CheckHTTPWithTrace ---

func TestBoost5_CheckHTTPWithTrace_InjectsTraceparent(t *testing.T) {
	var gotHeader string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHeader = r.Header.Get("traceparent")
		w.WriteHeader(200)
	}))
	defer srv.Close()

	tc := NewTraceContext()
	span := tc.NewSpan()
	results := CheckHTTPWithTrace(&schema.HTTPCheck{
		URL:        srv.URL,
		StatusCode: intPtrB5(200),
	}, span)
	if gotHeader == "" {
		t.Error("expected traceparent header to be injected")
	}
	allPassed := true
	for _, r := range results {
		if !r.Passed {
			allPassed = false
		}
	}
	if !allPassed {
		t.Errorf("expected all assertions to pass, got %+v", results)
	}
}

func TestBoost5_CheckHTTPWithTrace_PreservesExistingHeaders(t *testing.T) {
	var gotCustom string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotCustom = r.Header.Get("X-Custom")
		w.WriteHeader(200)
	}))
	defer srv.Close()

	tc := NewTraceContext()
	span := tc.NewSpan()
	results := CheckHTTPWithTrace(&schema.HTTPCheck{
		URL:        srv.URL,
		StatusCode: intPtrB5(200),
		Headers:    map[string]string{"X-Custom": "test-value"},
	}, span)
	if gotCustom != "test-value" {
		t.Errorf("expected X-Custom header preserved, got %q", gotCustom)
	}
	_ = results
}

// --- checkExecCredential ---

func TestBoost5_checkExecCredential_Success(t *testing.T) {
	result := checkExecCredential(&schema.CredentialCheck{
		Source: "exec",
		Name:   "echo secret-val",
	})
	if !result.Passed {
		t.Errorf("expected exec credential to pass, got: %s", result.Actual)
	}
}

func TestBoost5_checkExecCredential_Failure(t *testing.T) {
	result := checkExecCredential(&schema.CredentialCheck{
		Source: "exec",
		Name:   "exit 1",
	})
	if result.Passed {
		t.Error("expected exec credential to fail for exit 1")
	}
}

func TestBoost5_checkExecCredential_ContainsMismatch(t *testing.T) {
	result := checkExecCredential(&schema.CredentialCheck{
		Source:   "exec",
		Name:     "echo hello",
		Contains: "world",
	})
	if result.Passed {
		t.Error("expected exec credential contains check to fail")
	}
}

// --- CheckSSLCert: min days remaining with self-signed cert ---

func TestBoost5_CheckSSLCert_MinDaysRemaining(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer srv.Close()

	port := srv.Listener.Addr().(*net.TCPAddr).Port
	result := CheckSSLCert(&schema.SSLCertCheck{
		Host:             "127.0.0.1",
		Port:             port,
		AllowSelfSigned:  true,
		MinDaysRemaining: 999999,
	})
	if result.Passed {
		t.Error("min days remaining of 999999 should fail for test cert")
	}
}

func TestBoost5_CheckSSLCert_DefaultPort(t *testing.T) {
	result := CheckSSLCert(&schema.SSLCertCheck{
		Host: "127.0.0.1",
	})
	// Port 443 on localhost will fail, but this exercises the default port path
	if result.Type != "ssl_cert" {
		t.Errorf("expected ssl_cert type, got %q", result.Type)
	}
}

// --- runTestOnce: DocIntegrity through Runner ---

func TestBoost5_runTestOnce_DocIntegrity(t *testing.T) {
	dir := t.TempDir()
	readme := filepath.Join(dir, "README.md")
	os.WriteFile(readme, []byte("# Test CLI\n```bash\necho hello\n```\n"), 0644)

	cfg := &schema.SmokeConfig{Tests: []schema.Test{{
		Name: "doc-integrity",
		Expect: schema.Expect{
			DocIntegrity: &schema.DocIntegrityCheck{
				Binary:   "echo",
				Docs:     []string{"README.md"},
				Timeout:  schema.Duration{Duration: 5 * time.Second},
			},
		},
	}}}
	r := &Runner{Config: cfg, ConfigDir: dir, Reporter: &noopReporter{}}
	results := boost5RunWithRunner(r)
	if len(results) == 0 {
		t.Fatal("expected results")
	}
}

// --- runTestOnce: Skip guards - recursion ---

func TestBoost5_runTestOnce_RecursionGuard(t *testing.T) {
	t.Setenv("SMOKESIG_RUNNING", "1")
	results := boost5Runner(t, []schema.Test{{
		Name: "recursive",
		Run:  "go test ./...",
		Expect: schema.Expect{
			ExitCode: intPtrB5(0),
		},
	}})
	if len(results) == 0 {
		t.Fatal("expected result")
	}
	if !results[0].Skipped {
		t.Error("expected recursive test command to be skipped")
	}
}

// --- runTestOnce: DryRun mode ---

func TestBoost5_runTestOnce_DryRun(t *testing.T) {
	cfg := &schema.SmokeConfig{Tests: []schema.Test{{
		Name: "dry-test",
		Run:  "exit 1",
		Expect: schema.Expect{
			ExitCode: intPtrB5(0),
		},
	}}}
	r := &Runner{Config: cfg, ConfigDir: t.TempDir(), Reporter: &noopReporter{}}
	suite, _ := r.Run(RunOptions{DryRun: true})
	if suite == nil || len(suite.Tests) == 0 {
		t.Fatal("expected results")
	}
	if !suite.Tests[0].Passed {
		t.Error("dry run should mark test as passed regardless of command")
	}
}

// --- runTestOnce: SkipIf ---

func TestBoost5_runTestOnce_SkipIf_EnvUnset(t *testing.T) {
	results := boost5Runner(t, []schema.Test{{
		Name: "skip-if-env-unset",
		Run:  "exit 1",
		SkipIf: &schema.SkipIf{
			EnvUnset: "THIS_ENV_VAR_DOES_NOT_EXIST_BOOST5",
		},
	}})
	if len(results) == 0 {
		t.Fatal("expected result")
	}
	if !results[0].Skipped {
		t.Error("expected test to be skipped when env is unset")
	}
}

func TestBoost5_runTestOnce_SkipIf_FileMissing(t *testing.T) {
	results := boost5Runner(t, []schema.Test{{
		Name: "skip-if-file-missing",
		Run:  "exit 1",
		SkipIf: &schema.SkipIf{
			FileMissing: "/nonexistent/file/boost5.txt",
		},
	}})
	if len(results) == 0 {
		t.Fatal("expected result")
	}
	if !results[0].Skipped {
		t.Error("expected test to be skipped when file is missing")
	}
}

