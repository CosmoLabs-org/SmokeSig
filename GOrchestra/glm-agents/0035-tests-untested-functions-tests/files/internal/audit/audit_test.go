package audit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/CosmoLabs-org/SmokeSig/internal/schema"
)

func TestRun_NoConfig(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, ".smokesig.yaml")

	report, err := Run(dir, configPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if report.ConfigExists {
		t.Error("expected ConfigExists=false")
	}
	if report.Score != 0 {
		t.Errorf("expected score 0 for missing config, got %d", report.Score)
	}
	if len(report.Recommendations) == 0 {
		t.Error("expected at least one recommendation for missing config")
	}
	found := false
	for _, r := range report.Recommendations {
		if r.Type == "missing_config" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected missing_config recommendation")
	}
}

func TestRun_CompleteConfig(t *testing.T) {
	dir := t.TempDir()

	// Create a Go project with a complete config.
	writeFile(t, dir, "go.mod", "module example.com/test\ngo 1.21\n")
	writeFile(t, dir, ".smokesig.yaml", `
version: 1
project: test-project
tests:
  - name: Build
    run: go build ./...
    expect:
      exit_code: 0
  - name: Tests pass
    run: go test ./...
    expect:
      exit_code: 0
`)

	configPath := filepath.Join(dir, ".smokesig.yaml")
	report, err := Run(dir, configPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !report.ConfigExists {
		t.Error("expected ConfigExists=true")
	}
	if report.TestCount != 2 {
		t.Errorf("expected 2 tests, got %d", report.TestCount)
	}
	if report.Score < 5 {
		t.Errorf("expected score >= 5 for reasonable config, got %d", report.Score)
	}
	if report.ProjectType == "" || report.ProjectType == "unknown" {
		t.Error("expected project type to be detected")
	}
}

func TestRun_GoCLI_MissingDocIntegrity(t *testing.T) {
	dir := t.TempDir()

	// Create a Go CLI project (has cmd/ directory) without doc_integrity.
	writeFile(t, dir, "go.mod", "module example.com/test\ngo 1.21\n")
	if err := os.MkdirAll(filepath.Join(dir, "cmd"), 0755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, dir, ".smokesig.yaml", `
version: 1
project: test-project
tests:
  - name: Build
    run: go build ./...
    expect:
      exit_code: 0
`)

	configPath := filepath.Join(dir, ".smokesig.yaml")
	report, err := Run(dir, configPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	found := false
	for _, r := range report.Recommendations {
		if r.Type == "missing_assertion" && strings.Contains(r.Message, "doc_integrity") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected doc_integrity recommendation for Go CLI project")
	}
}

func TestRun_DockerProject_MissingAssertions(t *testing.T) {
	dir := t.TempDir()

	writeFile(t, dir, "Dockerfile", "FROM alpine\n")
	writeFile(t, dir, ".smokesig.yaml", `
version: 1
project: test-project
tests:
  - name: Build image
    run: docker build .
    expect:
      exit_code: 0
`)

	configPath := filepath.Join(dir, ".smokesig.yaml")
	report, err := Run(dir, configPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	foundDocker := false
	for _, r := range report.Recommendations {
		if r.Type == "missing_assertion" &&
			(strings.Contains(r.Message, "docker_container_running") || strings.Contains(r.Message, "docker_image_exists")) {
			foundDocker = true
			break
		}
	}
	if !foundDocker {
		t.Error("expected docker assertion recommendation")
	}
}

func TestRun_EnvExample_MissingEnvExists(t *testing.T) {
	dir := t.TempDir()

	writeFile(t, dir, ".env.example", "DATABASE_URL=postgres://...\n")
	writeFile(t, dir, ".smokesig.yaml", `
version: 1
project: test-project
tests:
  - name: Runs
    run: echo ok
    expect:
      exit_code: 0
`)

	configPath := filepath.Join(dir, ".smokesig.yaml")
	report, err := Run(dir, configPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	found := false
	for _, r := range report.Recommendations {
		if strings.Contains(r.Message, "env_exists") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected env_exists recommendation when .env.example present")
	}
}

func TestRun_StaleConfig(t *testing.T) {
	dir := t.TempDir()

	// Both old and new config present.
	writeFile(t, dir, ".smoke.yaml", "version: 1\nproject: old\ntests:\n  - name: x\n    run: echo\n    expect:\n      exit_code: 0\n")
	writeFile(t, dir, ".smokesig.yaml", `
version: 1
project: test-project
tests:
  - name: Build
    run: echo build
    expect:
      exit_code: 0
`)

	configPath := filepath.Join(dir, ".smokesig.yaml")
	report, err := Run(dir, configPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	found := false
	for _, r := range report.Recommendations {
		if r.Type == "stale_config" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected stale_config recommendation when .smoke.yaml exists")
	}
}

func TestRun_MissingBuildTest(t *testing.T) {
	dir := t.TempDir()

	writeFile(t, dir, ".smokesig.yaml", `
version: 1
project: test-project
tests:
  - name: Runs
    run: echo ok
    expect:
      stdout_contains: ok
`)

	configPath := filepath.Join(dir, ".smokesig.yaml")
	report, err := Run(dir, configPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	foundBuild := false
	foundExit := false
	for _, r := range report.Recommendations {
		if strings.Contains(r.Message, "build test") {
			foundBuild = true
		}
		if strings.Contains(r.Message, "exit_code") {
			foundExit = true
		}
	}
	if !foundBuild {
		t.Error("expected missing build test recommendation")
	}
	if !foundExit {
		t.Error("expected missing exit_code recommendation")
	}
}

func TestScoreCalculation(t *testing.T) {
	tests := []struct {
		name     string
		report   Report
		expected int
	}{
		{
			name:     "no config",
			report:   Report{ConfigExists: false},
			expected: 0,
		},
		{
			name: "perfect config",
			report: Report{
				ConfigExists: true,
				TestCount:    5,
			},
			expected: 10,
		},
		{
			name: "two warnings",
			report: Report{
				ConfigExists: true,
				TestCount:    3,
				Recommendations: []Recommendation{
					{Severity: SeverityWarning},
					{Severity: SeverityWarning},
				},
			},
			expected: 6,
		},
		{
			name: "no tests",
			report: Report{
				ConfigExists: true,
				TestCount:    0,
			},
			expected: 7,
		},
		{
			name: "many warnings floor at zero",
			report: Report{
				ConfigExists: true,
				TestCount:    1,
				Recommendations: []Recommendation{
					{Severity: SeverityWarning},
					{Severity: SeverityWarning},
					{Severity: SeverityWarning},
					{Severity: SeverityWarning},
					{Severity: SeverityWarning},
					{Severity: SeverityWarning},
				},
			},
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := calculateScore(&tt.report)
			if got != tt.expected {
				t.Errorf("calculateScore() = %d, want %d", got, tt.expected)
			}
		})
	}
}

func TestFormatTerminal(t *testing.T) {
	report := &Report{
		ConfigExists:    true,
		ConfigPath:      "/tmp/.smokesig.yaml",
		ProjectType:     "go",
		TestCount:       6,
		AssertionsUsed:  4,
		TotalAssertions: 45,
		Score:           7,
		Recommendations: []Recommendation{
			{Message: "Add doc_integrity assertion"},
		},
		Passes: []string{"Build test present"},
	}

	out := FormatTerminal(report)
	if out == "" {
		t.Fatal("expected non-empty output")
	}

	checks := []string{
		"SmokeSig Audit Report",
		"go (detected)",
		".smokesig.yaml (found)",
		"6 defined",
		"4 of 45",
		"doc_integrity",
		"Build test present",
		"7/10",
	}
	for _, check := range checks {
		if !strings.Contains(out, check) {
			t.Errorf("output missing %q:\n%s", check, out)
		}
	}
}

func TestCollectUsedAssertions(t *testing.T) {
	exitZero := 0
	cfg := &schema.SmokeConfig{
		Tests: []schema.Test{
			{
				Expect: schema.Expect{
					ExitCode:       &exitZero,
					StdoutContains: "hello",
				},
			},
			{
				Expect: schema.Expect{
					HTTP: &schema.HTTPCheck{URL: "http://localhost"},
					DocIntegrity: &schema.DocIntegrityCheck{
						Binary: "test",
						Docs:   []string{"README.md"},
					},
				},
			},
		},
	}

	used := collectUsedAssertions(cfg)

	expected := []string{"exit_code", "stdout_contains", "http", "doc_integrity"}
	for _, name := range expected {
		if !used[name] {
			t.Errorf("expected %q to be collected as used", name)
		}
	}
	if used["redis_ping"] {
		t.Error("redis_ping should not be in used set")
	}
}

func TestCollectUsedAssertions_EmptyTests(t *testing.T) {
	cfg := &schema.SmokeConfig{Tests: []schema.Test{}}
	used := collectUsedAssertions(cfg)
	if len(used) != 0 {
		t.Errorf("expected empty map for empty tests, got %d entries", len(used))
	}
}

func TestCollectUsedAssertions_NoAssertions(t *testing.T) {
	cfg := &schema.SmokeConfig{
		Tests: []schema.Test{
			{Name: "no-op", Run: "echo ok", Expect: schema.Expect{}},
		},
	}
	used := collectUsedAssertions(cfg)
	if len(used) != 0 {
		t.Errorf("expected empty map for test with no assertions, got %d entries", len(used))
	}
}

func TestCollectUsedAssertions_AllTypes(t *testing.T) {
	exitCode := 0
	responseTime := 100
	cfg := &schema.SmokeConfig{
		Tests: []schema.Test{
			{
				Expect: schema.Expect{
					ExitCode:         &exitCode,
					StdoutContains:   "hello",
					StdoutMatches:    "hel*o",
					StderrContains:   "error",
					StderrMatches:    "err.*",
					FileExists:       "/tmp/test",
					FileSize:         &schema.FileSizeCheck{Path: "/tmp/test"},
					EnvExists:        "HOME",
					PortListening:    &schema.PortCheck{Port: 8080},
					ProcessRunning:   "nginx",
					HTTP:             &schema.HTTPCheck{URL: "http://localhost"},
					JSONField:        &schema.JSONFieldCheck{Path: "status"},
					ResponseTimeMs:   &responseTime,
					SSLCert:          &schema.SSLCertCheck{Host: "example.com"},
					Redis:            &schema.RedisCheck{},
					Memcached:        &schema.MemcachedCheck{},
					Postgres:         &schema.PostgresCheck{},
					MySQL:            &schema.MySQLCheck{},
					GRPCHealth:       &schema.GRPCHealthCheck{Address: "localhost:50051"},
					DockerContainer:  &schema.DockerContainerCheck{Name: "test"},
					DockerImage:      &schema.DockerImageCheck{Image: "alpine"},
					URLReachable:     &schema.URLReachableCheck{URL: "http://example.com"},
					ServiceReachable: &schema.ServiceReachableCheck{URL: "http://example.com"},
					S3Bucket:         &schema.S3BucketCheck{Bucket: "test"},
					VersionCheck:     &schema.VersionCheck{Command: "go version", Pattern: "go1."},
					WebSocket:        &schema.WebSocketCheck{URL: "ws://localhost"},
					OTelTrace:        &schema.OTelTraceCheck{JaegerURL: "http://jaeger:16686"},
					Credential:       &schema.CredentialCheck{Source: "env", Name: "API_KEY"},
					GraphQL:          &schema.GraphQLCheck{URL: "http://localhost/graphql"},
					DeepLink:         &schema.DeepLinkCheck{URL: "myapp://path"},
					DNS:              &schema.DNSCheck{Hostname: "example.com"},
					SMTP:             &schema.SMTPCheck{Host: "smtp.example.com"},
					DockerCompose:    &schema.DockerComposeCheck{ComposeFile: "docker-compose.yml"},
					Ping:             &schema.PingCheck{Host: "example.com"},
					Mongo:            &schema.MongoCheck{},
					Kafka:            &schema.KafkaCheck{Brokers: []string{"localhost:9092"}},
					LDAP:             &schema.LDAPCheck{Host: "ldap.example.com"},
					MQTT:             &schema.MQTTCheck{Broker: "tcp://localhost:1883"},
					NTP:              &schema.NTPCheck{},
					K8sResource:      &schema.K8sResourceCheck{Namespace: "default", Kind: "pod", Name: "test"},
					IOSSimulator:     &schema.IOSSimulatorCheck{},
					AndroidEmulator:  &schema.AndroidEmulatorCheck{},
					DocIntegrity:     &schema.DocIntegrityCheck{Binary: "test", Docs: []string{"README.md"}},
				},
			},
		},
	}

	used := collectUsedAssertions(cfg)
	expected := []string{
		"exit_code", "stdout_contains", "stdout_matches", "stderr_contains", "stderr_matches",
		"file_exists", "file_size", "env_exists", "port_listening", "process_running",
		"http", "json_field", "response_time_ms", "ssl_cert", "redis_ping",
		"memcached_version", "postgres_ping", "mysql_ping", "grpc_health", "docker_container_running",
		"docker_image_exists", "url_reachable", "service_reachable", "s3_bucket", "version_check",
		"websocket", "otel_trace", "credential_check", "graphql", "deep_link",
		"dns_resolve", "smtp_ping", "docker_compose_healthy", "ping", "mongo_ping",
		"kafka_broker", "ldap_bind", "mqtt_ping", "ntp_check", "k8s_resource",
		"ios_simulator", "android_emulator", "doc_integrity",
	}
	if len(used) != len(expected) {
		t.Errorf("expected %d assertion types, got %d", len(expected), len(used))
	}
	for _, name := range expected {
		if !used[name] {
			t.Errorf("expected %q to be collected as used", name)
		}
	}
}

func TestCollectUsedAssertions_DuplicateTypesDeduped(t *testing.T) {
	exitCode := 0
	cfg := &schema.SmokeConfig{
		Tests: []schema.Test{
			{Expect: schema.Expect{ExitCode: &exitCode}},
			{Expect: schema.Expect{ExitCode: &exitCode}},
		},
	}
	used := collectUsedAssertions(cfg)
	if len(used) != 1 {
		t.Errorf("expected 1 unique assertion type, got %d", len(used))
	}
	if !used["exit_code"] {
		t.Error("expected exit_code to be collected")
	}
}

func TestHasHTTPServer(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(t *testing.T, dir string)
		expected bool
	}{
		{
			name:     "empty directory",
			setup:    func(t *testing.T, dir string) {},
			expected: false,
		},
		{
			name: "go file with ListenAndServe",
			setup: func(t *testing.T, dir string) {
				writeFile(t, dir, "main.go", "package main\nfunc main() { http.ListenAndServe(\":8080\", nil) }")
			},
			expected: true,
		},
		{
			name: "go file with ListenAndServeTLS",
			setup: func(t *testing.T, dir string) {
				writeFile(t, dir, "main.go", "package main\nfunc main() { http.ListenAndServeTLS(\":8443\", \"c\", \"k\", nil) }")
			},
			expected: true,
		},
		{
			name: "go file with gin.Default",
			setup: func(t *testing.T, dir string) {
				writeFile(t, dir, "server.go", "package main\nfunc main() { r := gin.Default() }")
			},
			expected: true,
		},
		{
			name: "go file with echo.New",
			setup: func(t *testing.T, dir string) {
				writeFile(t, dir, "server.go", "package main\nfunc main() { e := echo.New() }")
			},
			expected: true,
		},
		{
			name: "go file with fiber.New",
			setup: func(t *testing.T, dir string) {
				writeFile(t, dir, "server.go", "package main\nfunc main() { app := fiber.New() }")
			},
			expected: true,
		},
		{
			name: "go file with chi.NewRouter",
			setup: func(t *testing.T, dir string) {
				writeFile(t, dir, "server.go", "package main\nfunc main() { r := chi.NewRouter() }")
			},
			expected: true,
		},
		{
			name: "go file with mux.NewRouter",
			setup: func(t *testing.T, dir string) {
				writeFile(t, dir, "server.go", "package main\nfunc main() { r := mux.NewRouter() }")
			},
			expected: true,
		},
		{
			name: "go file without http patterns",
			setup: func(t *testing.T, dir string) {
				writeFile(t, dir, "main.go", "package main\nfunc main() { println(\"hello\") }")
			},
			expected: false,
		},
		{
			name: "non-go file with ListenAndServe ignored",
			setup: func(t *testing.T, dir string) {
				writeFile(t, dir, "main.txt", "http.ListenAndServe(\":8080\", nil)")
			},
			expected: false,
		},
		{
			name: "cmd subdir with ListenAndServe",
			setup: func(t *testing.T, dir string) {
				writeFile(t, dir, "cmd/server.go", "package main\nfunc main() { http.ListenAndServe(\":8080\", nil) }")
			},
			expected: true,
		},
		{
			name: "cmd subdir with ListenAndServeTLS",
			setup: func(t *testing.T, dir string) {
				writeFile(t, dir, "cmd/server.go", "package main\nfunc main() { http.ListenAndServeTLS(\":8443\", \"c\", \"k\", nil) }")
			},
			expected: true,
		},
		{
			name: "cmd subdir non-go file ignored",
			setup: func(t *testing.T, dir string) {
				writeFile(t, dir, "cmd/server.txt", "http.ListenAndServe(\":8080\", nil)")
			},
			expected: false,
		},
		{
			name: "cmd subdir directory entry skipped",
			setup: func(t *testing.T, dir string) {
				writeFile(t, dir, "cmd/sub/main.go", "package main\nfunc main() { http.ListenAndServe(\":8080\", nil) }")
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			tt.setup(t, dir)
			got := hasHTTPServer(dir)
			if got != tt.expected {
				t.Errorf("hasHTTPServer() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestHasPkgScript(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(t *testing.T, dir string)
		script   string
		expected bool
	}{
		{
			name:     "no package.json",
			setup:    func(t *testing.T, dir string) {},
			script:   "start",
			expected: false,
		},
		{
			name: "has target script",
			setup: func(t *testing.T, dir string) {
				writeFile(t, dir, "package.json", `{"scripts": {"start": "node index.js"}}`)
			},
			script:   "start",
			expected: true,
		},
		{
			name: "missing target script",
			setup: func(t *testing.T, dir string) {
				writeFile(t, dir, "package.json", `{"scripts": {"test": "jest"}}`)
			},
			script:   "start",
			expected: false,
		},
		{
			name: "invalid json but contains script string",
			setup: func(t *testing.T, dir string) {
				writeFile(t, dir, "package.json", `{not json "start" blah}`)
			},
			script:   "start",
			expected: true,
		},
		{
			name: "invalid json without script string",
			setup: func(t *testing.T, dir string) {
				writeFile(t, dir, "package.json", `{not json blah}`)
			},
			script:   "start",
			expected: false,
		},
		{
			name: "dev script present",
			setup: func(t *testing.T, dir string) {
				writeFile(t, dir, "package.json", `{"scripts": {"dev": "vite"}}`)
			},
			script:   "dev",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			tt.setup(t, dir)
			got := hasPkgScript(dir, tt.script)
			if got != tt.expected {
				t.Errorf("hasPkgScript(_, %q) = %v, want %v", tt.script, got, tt.expected)
			}
		})
	}
}

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}
