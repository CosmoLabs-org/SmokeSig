package mcp

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/CosmoLabs-org/SmokeSig/internal/detector"
	"github.com/CosmoLabs-org/SmokeSig/internal/monorepo"
	"github.com/CosmoLabs-org/SmokeSig/internal/reporter"
	"gopkg.in/yaml.v3"
	"github.com/CosmoLabs-org/SmokeSig/internal/runner"
	"github.com/CosmoLabs-org/SmokeSig/internal/schema"
)

// handleSmokeRun executes smoke tests and returns structured results.
func handleSmokeRun(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	configPath := resolveConfigPath(strArg(args, "config_path", ".smokesig.yaml"))

	cfg, err := schema.Load(configPath)
	if err != nil {
		return nil, fmt.Errorf("loading config: %w", err)
	}

	configDir := filepath.Dir(configPath)

	opts := runner.RunOptions{
		Tags:        strSliceArg(args, "tags"),
		ExcludeTags: strSliceArg(args, "exclude_tags"),
		FailFast:    boolArg(args, "fail_fast", false),
		DryRun:      boolArg(args, "dry_run", false),
	}

	if timeout := strArg(args, "timeout", ""); timeout != "" {
		d, err := time.ParseDuration(timeout)
		if err != nil {
			return nil, fmt.Errorf("invalid timeout %q: %w", timeout, err)
		}
		opts.Timeout = d
	}

	r := &runner.Runner{
		Config:    cfg,
		Reporter:  &noopReporter{},
		ConfigDir: configDir,
	}

	suiteResult, err := r.Run(opts)
	if err != nil {
		return nil, fmt.Errorf("running tests: %w", err)
	}

	return suiteResultToMCP(suiteResult, configPath), nil
}

// handleSmokeInit generates a .smokesig.yaml config.
func handleSmokeInit(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	dir := strArg(args, "directory", ".")
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("resolving directory: %w", err)
	}

	types := detector.Detect(absDir)
	if len(types) == 0 {
		return nil, fmt.Errorf("no known project type detected in %s", absDir)
	}

	cfg := detector.GenerateConfig(absDir, types)
	yamlBytes, err := yaml.Marshal(cfg)
	if err != nil {
		return nil, fmt.Errorf("marshalling config: %w", err)
	}

	shouldWrite := boolArg(args, "write", false)
	if shouldWrite {
		outPath := filepath.Join(absDir, ".smokesig.yaml")
		force := boolArg(args, "force", false)
		if !force {
			if _, err := filepath.Abs(outPath); err == nil {
				if fi, statErr := os.Stat(outPath); statErr == nil && !fi.IsDir() {
					return nil, fmt.Errorf(".smokesig.yaml already exists (use force=true to overwrite)")
				}
			}
		}
		if err := os.WriteFile(outPath, yamlBytes, 0644); err != nil {
			return nil, fmt.Errorf("writing config: %w", err)
		}
		return &InitResult{YAML: string(yamlBytes), Written: true, WritePath: outPath}, nil
	}

	return &InitResult{YAML: string(yamlBytes), Written: false}, nil
}

// handleSmokeValidate validates a config file.
func handleSmokeValidate(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	configPath := resolveConfigPath(strArg(args, "config_path", ".smokesig.yaml"))

	cfg, err := schema.Load(configPath)
	if err != nil {
		return nil, fmt.Errorf("loading config: %w", err)
	}

	verr := schema.Validate(cfg)
	if verr != nil {
		if ve, ok := verr.(*schema.ValidationError); ok {
			return &ValidateResult{Valid: false, Errors: ve.Errors}, nil
		}
		return &ValidateResult{Valid: false, Errors: []string{verr.Error()}}, nil
	}

	testNames := make([]string, len(cfg.Tests))
	for i, t := range cfg.Tests {
		testNames[i] = t.Name
	}

	return &ValidateResult{Valid: true, Tests: testNames}, nil
}

// handleSmokeList lists tests in a config.
func handleSmokeList(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	configPath := resolveConfigPath(strArg(args, "config_path", ".smokesig.yaml"))

	cfg, err := schema.Load(configPath)
	if err != nil {
		return nil, fmt.Errorf("loading config: %w", err)
	}

	tagFilter := strSliceArg(args, "tags")
	tests := make([]ListedTest, 0)
	for _, t := range cfg.Tests {
		if len(tagFilter) > 0 && !hasTag(t.Tags, tagFilter) {
			continue
		}
		tests = append(tests, ListedTest{
			Name:           t.Name,
			Tags:           t.Tags,
			RunCommand:     t.Run,
			AssertionTypes: getAssertionTypes(t.Expect),
			SkipIf:         skipIfString(t.SkipIf),
		})
	}

	return &ListResult{ConfigPath: configPath, Tests: tests}, nil
}

// handleSmokeDiscover finds .smokesig.yaml files.
func handleSmokeDiscover(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	dir := strArg(args, "directory", ".")

	absDir, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("resolving directory: %w", err)
	}

	subs, err := monorepo.Discover(absDir, nil)
	if err != nil {
		return nil, fmt.Errorf("discovering configs: %w", err)
	}

	// Also check if the root directory has a .smokesig.yaml
	configs := make([]DiscoveredConfig, 0)
	rootConfig := filepath.Join(absDir, ".smokesig.yaml")
	if _, err := filepath.Abs(rootConfig); err == nil {
		if fi, err := filepath.Abs(rootConfig); err == nil {
			if _, statErr := filepath.Abs(fi); statErr == nil {
				configs = append(configs, DiscoveredConfig{
					Path:        rootConfig,
					Directory:   absDir,
					ProjectName: filepath.Base(absDir),
				})
			}
		}
	}

	for _, sub := range subs {
		configs = append(configs, DiscoveredConfig{
			Path:        sub.Path,
			Directory:   sub.Dir,
			ProjectName: sub.Project,
		})
	}

	return &DiscoverResult{Configs: configs}, nil
}

// handleSmokeExplain explains an assertion type.
func handleSmokeExplain(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	assertionType := strArg(args, "assertion_type", "")
	if assertionType == "" {
		return nil, fmt.Errorf("assertion_type is required")
	}

	info, ok := assertionDocs[assertionType]
	if !ok {
		return nil, fmt.Errorf("unknown assertion type: %s", assertionType)
	}
	return info, nil
}

// handleSmokeGenerateTest generates a single test YAML snippet.
func handleSmokeGenerateTest(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	name := strArg(args, "name", "")
	if name == "" {
		return nil, fmt.Errorf("name is required")
	}
	assertionType := strArg(args, "assertion_type", "")
	if assertionType == "" {
		return nil, fmt.Errorf("assertion_type is required")
	}

	params, _ := args["params"].(map[string]interface{})
	yaml := generateTestYAML(name, assertionType, params, strSliceArg(args, "tags"))
	return &GenerateTestResult{YAML: yaml}, nil
}

// generateTestYAML creates a YAML snippet for a single test.
func generateTestYAML(name, assertionType string, params map[string]interface{}, tags []string) string {
	var b strings.Builder
	b.WriteString("  - name: ")
	b.WriteString(name)
	b.WriteString("\n")

	if len(tags) > 0 {
		b.WriteString("    tags:\n")
		for _, t := range tags {
			fmt.Fprintf(&b, "      - %s\n", t)
		}
	}

	expectBlock := generateExpectBlock(assertionType, params)
	if expectBlock != "" {
		if needsRunCommand(assertionType) {
			b.WriteString("    run: <command>\n")
		}
		b.WriteString("    expect:\n")
		b.WriteString(expectBlock)
	}

	return b.String()
}

// needsRunCommand returns true if the assertion type requires a run command.
func needsRunCommand(assertionType string) bool {
	standalone := map[string]bool{
		"port_listening": true, "process_running": true, "http": true,
		"ssl_cert": true, "redis_ping": true, "memcached_version": true,
		"postgres_ping": true, "mysql_ping": true, "grpc_health": true,
		"docker_container_running": true, "docker_image_exists": true,
		"url_reachable": true, "service_reachable": true, "s3_bucket": true,
		"version_check": true, "websocket": true, "otel_trace": true,
		"credential_check": true, "graphql": true,
	}
	return !standalone[assertionType]
}

// generateExpectBlock creates the YAML for an assertion's expect block.
func generateExpectBlock(assertionType string, params map[string]interface{}) string {
	switch assertionType {
	case "exit_code":
		code := intParam(params, "code", 0)
		return fmt.Sprintf("      exit_code: %d\n", code)

	case "stdout_contains":
		return fmt.Sprintf("      stdout_contains: %q\n", strParam(params, "text", "expected output"))

	case "stdout_matches":
		return fmt.Sprintf("      stdout_matches: %q\n", strParam(params, "pattern", ".*"))

	case "stderr_contains":
		return fmt.Sprintf("      stderr_contains: %q\n", strParam(params, "text", "error output"))

	case "stderr_matches":
		return fmt.Sprintf("      stderr_matches: %q\n", strParam(params, "pattern", ".*"))

	case "file_exists":
		return fmt.Sprintf("      file_exists: %q\n", strParam(params, "path", "output.txt"))

	case "env_exists":
		return fmt.Sprintf("      env_exists: %q\n", strParam(params, "var", "HOME"))

	case "port_listening":
		port := intParam(params, "port", 8080)
		proto := strParam(params, "protocol", "tcp")
		host := strParam(params, "host", "localhost")
		return fmt.Sprintf("      port_listening:\n        port: %d\n        protocol: %s\n        host: %s\n", port, proto, host)

	case "process_running":
		return fmt.Sprintf("      process_running: %q\n", strParam(params, "name", "nginx"))

	case "http":
		url := strParam(params, "url", "http://localhost:8080/health")
		code := intParam(params, "status_code", 200)
		return fmt.Sprintf("      http:\n        url: %q\n        status_code: %d\n", url, code)

	case "json_field":
		path := strParam(params, "path", "key")
		eq := strParam(params, "equals", "")
		if eq != "" {
			return fmt.Sprintf("      json_field:\n        path: %q\n        equals: %q\n", path, eq)
		}
		return fmt.Sprintf("      json_field:\n        path: %q\n", path)

	case "response_time_ms":
		ms := intParam(params, "ms", 500)
		return fmt.Sprintf("      response_time_ms: %d\n", ms)

	case "ssl_cert":
		host := strParam(params, "host", "example.com")
		days := intParam(params, "min_days_remaining", 30)
		return fmt.Sprintf("      ssl_cert:\n        host: %q\n        min_days_remaining: %d\n", host, days)

	case "redis_ping":
		host := strParam(params, "host", "localhost")
		port := intParam(params, "port", 6379)
		return fmt.Sprintf("      redis_ping:\n        host: %q\n        port: %d\n", host, port)

	case "memcached_version":
		return fmt.Sprintf("      memcached_version:\n        host: %q\n        port: %d\n", strParam(params, "host", "localhost"), intParam(params, "port", 11211))

	case "postgres_ping":
		return fmt.Sprintf("      postgres_ping:\n        host: %q\n        port: %d\n", strParam(params, "host", "localhost"), intParam(params, "port", 5432))

	case "mysql_ping":
		return fmt.Sprintf("      mysql_ping:\n        host: %q\n        port: %d\n", strParam(params, "host", "localhost"), intParam(params, "port", 3306))

	case "grpc_health":
		addr := strParam(params, "address", "localhost:9090")
		return fmt.Sprintf("      grpc_health:\n        address: %q\n", addr)

	case "docker_container_running":
		return fmt.Sprintf("      docker_container_running:\n        name: %q\n", strParam(params, "name", "my-container"))

	case "docker_image_exists":
		return fmt.Sprintf("      docker_image_exists:\n        image: %q\n", strParam(params, "image", "nginx:alpine"))

	case "url_reachable":
		url := strParam(params, "url", "https://example.com")
		code := intParam(params, "status_code", 200)
		return fmt.Sprintf("      url_reachable:\n        url: %q\n        status_code: %d\n", url, code)

	case "service_reachable":
		return fmt.Sprintf("      service_reachable:\n        url: %q\n", strParam(params, "url", "https://api.example.com/health"))

	case "s3_bucket":
		return fmt.Sprintf("      s3_bucket:\n        bucket: %q\n        region: %q\n", strParam(params, "bucket", "my-bucket"), strParam(params, "region", "us-east-1"))

	case "version_check":
		return fmt.Sprintf("      version_check:\n        command: %q\n        pattern: %q\n", strParam(params, "command", "go version"), strParam(params, "pattern", `\d+\.\d+\.\d+`))

	case "websocket":
		url := strParam(params, "url", "ws://localhost:8080/ws")
		send := strParam(params, "send", "ping")
		return fmt.Sprintf("      websocket:\n        url: %q\n        send: %q\n        expect_contains: %q\n", url, send, "pong")

	case "otel_trace":
		svc := strParam(params, "service_name", "my-service")
		return fmt.Sprintf("      otel_trace:\n        service_name: %q\n        min_spans: %d\n", svc, intParam(params, "min_spans", 1))

	case "credential_check":
		return fmt.Sprintf("      credential_check:\n        source: %q\n        name: %q\n", strParam(params, "source", "env"), strParam(params, "name", "API_KEY"))

	case "graphql":
		url := strParam(params, "url", "http://localhost:8080/graphql")
		return fmt.Sprintf("      graphql:\n        url: %q\n", url)

	default:
		return fmt.Sprintf("      # assertion type %q — add assertion fields here\n", assertionType)
	}
}

func strParam(params map[string]interface{}, key, def string) string {
	if v, ok := params[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return def
}

func intParam(params map[string]interface{}, key string, def int) int {
	if v, ok := params[key]; ok {
		if f, ok := v.(float64); ok {
			return int(f)
		}
	}
	return def
}

// --- helpers ---

func resolveConfigPath(configPath string) string {
	if filepath.IsAbs(configPath) {
		return configPath
	}
	abs, err := filepath.Abs(configPath)
	if err != nil {
		return configPath
	}
	return abs
}

func strArg(args map[string]interface{}, key, def string) string {
	v, ok := args[key]
	if !ok {
		return def
	}
	s, ok := v.(string)
	if !ok {
		return def
	}
	return s
}

func boolArg(args map[string]interface{}, key string, def bool) bool {
	v, ok := args[key]
	if !ok {
		return def
	}
	b, ok := v.(bool)
	if !ok {
		return def
	}
	return b
}

func strSliceArg(args map[string]interface{}, key string) []string {
	v, ok := args[key]
	if !ok {
		return nil
	}
	switch val := v.(type) {
	case []string:
		return val
	case []interface{}:
		result := make([]string, 0, len(val))
		for _, item := range val {
			if s, ok := item.(string); ok {
				result = append(result, s)
			}
		}
		return result
	default:
		return nil
	}
}

func hasTag(tags []string, filter []string) bool {
	for _, ft := range filter {
		for _, t := range tags {
			if t == ft {
				return true
			}
		}
	}
	return false
}

// getAssertionTypes returns the list of assertion types set in an Expect block.
func getAssertionTypes(e schema.Expect) []string {
	var types []string
	if e.ExitCode != nil {
		types = append(types, "exit_code")
	}
	if e.StdoutContains != "" {
		types = append(types, "stdout_contains")
	}
	if e.StdoutMatches != "" {
		types = append(types, "stdout_matches")
	}
	if e.StderrContains != "" {
		types = append(types, "stderr_contains")
	}
	if e.StderrMatches != "" {
		types = append(types, "stderr_matches")
	}
	if e.FileExists != "" {
		types = append(types, "file_exists")
	}
	if e.EnvExists != "" {
		types = append(types, "env_exists")
	}
	if e.PortListening != nil {
		types = append(types, "port_listening")
	}
	if e.ProcessRunning != "" {
		types = append(types, "process_running")
	}
	if e.HTTP != nil {
		types = append(types, "http")
	}
	if e.JSONField != nil {
		types = append(types, "json_field")
	}
	if e.ResponseTimeMs != nil {
		types = append(types, "response_time_ms")
	}
	if e.SSLCert != nil {
		types = append(types, "ssl_cert")
	}
	if e.Redis != nil {
		types = append(types, "redis_ping")
	}
	if e.Memcached != nil {
		types = append(types, "memcached_version")
	}
	if e.Postgres != nil {
		types = append(types, "postgres_ping")
	}
	if e.MySQL != nil {
		types = append(types, "mysql_ping")
	}
	if e.GRPCHealth != nil {
		types = append(types, "grpc_health")
	}
	if e.DockerContainer != nil {
		types = append(types, "docker_container_running")
	}
	if e.DockerImage != nil {
		types = append(types, "docker_image_exists")
	}
	if e.URLReachable != nil {
		types = append(types, "url_reachable")
	}
	if e.ServiceReachable != nil {
		types = append(types, "service_reachable")
	}
	if e.S3Bucket != nil {
		types = append(types, "s3_bucket")
	}
	if e.VersionCheck != nil {
		types = append(types, "version_check")
	}
	if e.OTelTrace != nil {
		types = append(types, "otel_trace")
	}
	if e.WebSocket != nil {
		types = append(types, "websocket")
	}
	if e.Credential != nil {
		types = append(types, "credential_check")
	}
	if e.GraphQL != nil {
		types = append(types, "graphql")
	}
	return types
}

// suiteResultToMCP converts runner.SuiteResult to MCP RunResult.
func suiteResultToMCP(sr *runner.SuiteResult, configPath string) *RunResult {
	tests := make([]TestResult, len(sr.Tests))
	for i, tr := range sr.Tests {
		assertions := make([]AssertionResult, len(tr.Assertions))
		for j, ar := range tr.Assertions {
			assertions[j] = AssertionResult{
				Type:     ar.Type,
				Expected: ar.Expected,
				Actual:   ar.Actual,
				Passed:   ar.Passed,
			}
		}

		tests[i] = TestResult{
			Name:           tr.Name,
			Passed:         tr.Passed,
			Skipped:        tr.Skipped,
			AllowedFailure: tr.AllowedFailure,
			DurationMs:     tr.Duration.Milliseconds(),
			Assertions:     assertions,
		}
		if tr.Error != nil {
			tests[i].Error = tr.Error.Error()
		}
		// Add fix suggestions for failed assertions
		if !tr.Passed && !tr.AllowedFailure {
			for _, ar := range tr.Assertions {
				if !ar.Passed {
					tests[i].FixSuggestions = append(tests[i].FixSuggestions, GetSuggestions(ar.Type, ar.Actual)...)
				}
			}
			if tr.Error != nil {
				tests[i].FixSuggestions = append(tests[i].FixSuggestions, GetSuggestions("exit_code", tr.Error.Error())...)
			}
		}
	}

	return &RunResult{
		Project:    sr.Project,
		Total:      sr.Total,
		Passed:     sr.Passed,
		Failed:     sr.Failed,
		Skipped:    sr.Skipped,
		Duration:   sr.Duration,
		Tests:      tests,
		ConfigPath: configPath,
	}
}

// noopReporter is a silent reporter for MCP handler use.
type noopReporter struct{}

func skipIfString(si *schema.SkipIf) string {
	if si == nil {
		return ""
	}
	parts := make([]string, 0)
	if si.EnvUnset != "" {
		parts = append(parts, fmt.Sprintf("env_unset:%s", si.EnvUnset))
	}
	if si.EnvEquals != nil {
		parts = append(parts, fmt.Sprintf("env_equals:%s=%s", si.EnvEquals.Var, si.EnvEquals.Value))
	}
	if si.FileMissing != "" {
		parts = append(parts, fmt.Sprintf("file_missing:%s", si.FileMissing))
	}
	return strings.Join(parts, ", ")
}

func (n *noopReporter) PrereqStart(_ string)                     {}
func (n *noopReporter) PrereqResult(_ reporter.PrereqResultData) {}
func (n *noopReporter) TestStart(_ string)                       {}
func (n *noopReporter) TestResult(_ reporter.TestResultData)     {}
func (n *noopReporter) Summary(_ reporter.SuiteResultData)       {}

// Sanitize truncates output for display in MCP results.
func sanitize(s string, maxLen int) string {
	s = strings.TrimSpace(s)
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + fmt.Sprintf("\n[... truncated, full output: %d bytes]", len(s))
}
