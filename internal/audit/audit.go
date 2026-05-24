package audit

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/CosmoLabs-org/SmokeSig/internal/detector"
	"github.com/CosmoLabs-org/SmokeSig/internal/schema"
)

// Severity levels for recommendations.
type Severity string

const (
	SeverityWarning Severity = "warning"
	SeverityInfo    Severity = "info"
)

// Recommendation is a single audit finding.
type Recommendation struct {
	Type       string   `json:"type"`
	Severity   Severity `json:"severity"`
	Message    string   `json:"message"`
	FixCommand string   `json:"fix_command,omitempty"`
}

// Report is the full audit result.
type Report struct {
	ConfigExists    bool              `json:"config_exists"`
	ConfigPath      string            `json:"config_path"`
	ProjectType     string            `json:"project_type"`
	TestCount       int               `json:"test_count"`
	AssertionsUsed  int               `json:"assertions_used"`
	TotalAssertions int               `json:"total_assertions"`
	Recommendations []Recommendation  `json:"recommendations"`
	Passes          []string          `json:"passes"`
	Score           int               `json:"score"`
}

// TotalAssertionTypes is the number of available assertion types in SmokeSig.
const TotalAssertionTypes = 45

// Run performs a full audit of the project at dir using the given config file path.
func Run(dir, configPath string) (*Report, error) {
	report := &Report{
		TotalAssertions: TotalAssertionTypes,
	}

	// Step 1: Check config existence.
	cfg, configExists := loadConfig(configPath)
	report.ConfigExists = configExists
	report.ConfigPath = configPath

	// Step 2: Detect project type.
	types := detector.Detect(dir)
	if len(types) > 0 {
		names := make([]string, len(types))
		for i, t := range types {
			names[i] = string(t)
		}
		report.ProjectType = strings.Join(names, ", ")
	} else {
		report.ProjectType = "unknown"
	}

	if !configExists {
		report.Recommendations = append(report.Recommendations, Recommendation{
			Type:       "missing_config",
			Severity:   SeverityWarning,
			Message:    "No .smokesig.yaml found — run smokesig init to generate one",
			FixCommand: "smokesig init",
		})
		report.Score = 0
		return report, nil
	}

	if cfg == nil {
		// Config file exists but failed to parse.
		report.Recommendations = append(report.Recommendations, Recommendation{
			Type:     "invalid_config",
			Severity: SeverityWarning,
			Message:  "Config file exists but failed to parse — run smokesig validate",
		})
		report.Score = 0
		return report, nil
	}

	report.TestCount = len(cfg.Tests)

	// Collect which assertion types are in use.
	used := collectUsedAssertions(cfg)
	report.AssertionsUsed = len(used)

	// Step 3: Check for stale config references.
	checkStaleReferences(dir, report)

	// Step 4: Check assertion coverage based on project type.
	checkAssertionCoverage(dir, cfg, types, used, report)

	// Step 5: Check for common baseline tests.
	checkBaselineTests(cfg, report)

	// Step 6: Calculate score.
	report.Score = calculateScore(report)

	return report, nil
}

// loadConfig attempts to load the config. Returns (config, exists).
func loadConfig(path string) (*schema.SmokeConfig, bool) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, false
	}
	cfg, err := schema.Load(path)
	if err != nil {
		return nil, true // file exists but can't load
	}
	return cfg, true
}

// collectUsedAssertions returns a set of assertion type names used in the config.
func collectUsedAssertions(cfg *schema.SmokeConfig) map[string]bool {
	used := make(map[string]bool)
	for _, t := range cfg.Tests {
		e := t.Expect
		if e.ExitCode != nil {
			used["exit_code"] = true
		}
		if e.StdoutContains != "" {
			used["stdout_contains"] = true
		}
		if e.StdoutMatches != "" {
			used["stdout_matches"] = true
		}
		if e.StderrContains != "" {
			used["stderr_contains"] = true
		}
		if e.StderrMatches != "" {
			used["stderr_matches"] = true
		}
		if e.FileExists != "" {
			used["file_exists"] = true
		}
		if e.FileSize != nil {
			used["file_size"] = true
		}
		if e.EnvExists != "" {
			used["env_exists"] = true
		}
		if e.PortListening != nil {
			used["port_listening"] = true
		}
		if e.ProcessRunning != "" {
			used["process_running"] = true
		}
		if e.HTTP != nil {
			used["http"] = true
		}
		if e.JSONField != nil {
			used["json_field"] = true
		}
		if e.ResponseTimeMs != nil {
			used["response_time_ms"] = true
		}
		if e.SSLCert != nil {
			used["ssl_cert"] = true
		}
		if e.Redis != nil {
			used["redis_ping"] = true
		}
		if e.Memcached != nil {
			used["memcached_version"] = true
		}
		if e.Postgres != nil {
			used["postgres_ping"] = true
		}
		if e.MySQL != nil {
			used["mysql_ping"] = true
		}
		if e.GRPCHealth != nil {
			used["grpc_health"] = true
		}
		if e.DockerContainer != nil {
			used["docker_container_running"] = true
		}
		if e.DockerImage != nil {
			used["docker_image_exists"] = true
		}
		if e.URLReachable != nil {
			used["url_reachable"] = true
		}
		if e.ServiceReachable != nil {
			used["service_reachable"] = true
		}
		if e.S3Bucket != nil {
			used["s3_bucket"] = true
		}
		if e.VersionCheck != nil {
			used["version_check"] = true
		}
		if e.WebSocket != nil {
			used["websocket"] = true
		}
		if e.OTelTrace != nil {
			used["otel_trace"] = true
		}
		if e.Credential != nil {
			used["credential_check"] = true
		}
		if e.GraphQL != nil {
			used["graphql"] = true
		}
		if e.DeepLink != nil {
			used["deep_link"] = true
		}
		if e.DNS != nil {
			used["dns_resolve"] = true
		}
		if e.SMTP != nil {
			used["smtp_ping"] = true
		}
		if e.DockerCompose != nil {
			used["docker_compose_healthy"] = true
		}
		if e.Ping != nil {
			used["ping"] = true
		}
		if e.Mongo != nil {
			used["mongo_ping"] = true
		}
		if e.Kafka != nil {
			used["kafka_broker"] = true
		}
		if e.LDAP != nil {
			used["ldap_bind"] = true
		}
		if e.MQTT != nil {
			used["mqtt_ping"] = true
		}
		if e.NTP != nil {
			used["ntp_check"] = true
		}
		if e.K8sResource != nil {
			used["k8s_resource"] = true
		}
		if e.IOSSimulator != nil {
			used["ios_simulator"] = true
		}
		if e.AndroidEmulator != nil {
			used["android_emulator"] = true
		}
		if e.DocIntegrity != nil {
			used["doc_integrity"] = true
		}
	}
	return used
}

// checkStaleReferences looks for legacy config file names.
func checkStaleReferences(dir string, report *Report) {
	if _, err := os.Stat(filepath.Join(dir, ".smoke.yaml")); err == nil {
		report.Recommendations = append(report.Recommendations, Recommendation{
			Type:       "stale_config",
			Severity:   SeverityWarning,
			Message:    "Legacy .smoke.yaml found — rename to .smokesig.yaml",
			FixCommand: "mv .smoke.yaml .smokesig.yaml",
		})
	}
}

// checkAssertionCoverage compares detected project characteristics against configured assertions.
func checkAssertionCoverage(dir string, cfg *schema.SmokeConfig, types []detector.ProjectType, used map[string]bool, report *Report) {
	hasType := func(t detector.ProjectType) bool {
		for _, pt := range types {
			if pt == t {
				return true
			}
		}
		return false
	}

	// Go project: recommend doc_integrity for CLI projects (has cmd/ directory).
	if hasType(detector.Go) {
		if _, err := os.Stat(filepath.Join(dir, "cmd")); err == nil {
			if !used["doc_integrity"] {
				report.Recommendations = append(report.Recommendations, Recommendation{
					Type:     "missing_assertion",
					Severity: SeverityWarning,
					Message:  "Add doc_integrity assertion — Go CLI project with cmd/ directory",
				})
			}
		}
	}

	// Docker project: recommend docker_container_running or docker_image_exists.
	if hasType(detector.Docker) {
		if !used["docker_container_running"] && !used["docker_image_exists"] {
			report.Recommendations = append(report.Recommendations, Recommendation{
				Type:     "missing_assertion",
				Severity: SeverityWarning,
				Message:  "Add docker_container_running or docker_image_exists assertion — Docker project detected",
			})
		}
	}

	// Dockerfile present but no docker_image_exists.
	if _, err := os.Stat(filepath.Join(dir, "Dockerfile")); err == nil {
		if !used["docker_image_exists"] {
			report.Recommendations = append(report.Recommendations, Recommendation{
				Type:     "missing_assertion",
				Severity: SeverityInfo,
				Message:  "Add docker_image_exists assertion — Dockerfile found in project",
			})
		}
	}

	// .env.example present but no env_exists assertions.
	if _, err := os.Stat(filepath.Join(dir, ".env.example")); err == nil {
		if !used["env_exists"] {
			report.Recommendations = append(report.Recommendations, Recommendation{
				Type:     "missing_assertion",
				Severity: SeverityWarning,
				Message:  "Add env_exists assertions — .env.example found but no env checks configured",
			})
		}
	}

	// HTTP server detection: check if any Go file references http.ListenAndServe or net/http.
	if hasType(detector.Go) && !used["http"] {
		if hasHTTPServer(dir) {
			report.Recommendations = append(report.Recommendations, Recommendation{
				Type:     "missing_assertion",
				Severity: SeverityWarning,
				Message:  "Add http assertion — HTTP server detected in Go source",
			})
		}
	}

	// Node project with start script but no http assertion.
	if hasType(detector.Node) && !used["http"] {
		if hasPkgScript(dir, "start") || hasPkgScript(dir, "dev") {
			report.Recommendations = append(report.Recommendations, Recommendation{
				Type:     "missing_assertion",
				Severity: SeverityInfo,
				Message:  "Consider adding http assertion — Node project has start/dev script",
			})
		}
	}

	// docker-compose.yml present but no docker_compose_healthy.
	if _, err := os.Stat(filepath.Join(dir, "docker-compose.yml")); err == nil {
		if !used["docker_compose_healthy"] {
			report.Recommendations = append(report.Recommendations, Recommendation{
				Type:     "missing_assertion",
				Severity: SeverityInfo,
				Message:  "Add docker_compose_healthy assertion — docker-compose.yml found",
			})
		}
	}

	// Helm chart but no k8s_resource.
	if hasType(detector.Helm) && !used["k8s_resource"] {
		report.Recommendations = append(report.Recommendations, Recommendation{
			Type:     "missing_assertion",
			Severity: SeverityInfo,
			Message:  "Consider adding k8s_resource assertion — Helm chart project detected",
		})
	}

	// React Native but no deep_link or ios_simulator/android_emulator.
	if hasType(detector.ReactNative) {
		if !used["deep_link"] {
			report.Recommendations = append(report.Recommendations, Recommendation{
				Type:     "missing_assertion",
				Severity: SeverityInfo,
				Message:  "Consider adding deep_link assertion — React Native project detected",
			})
		}
	}

	// iOS project but no ios_simulator.
	if hasType(detector.IOS) && !used["ios_simulator"] {
		report.Recommendations = append(report.Recommendations, Recommendation{
			Type:     "missing_assertion",
			Severity: SeverityInfo,
			Message:  "Consider adding ios_simulator assertion — iOS project detected",
		})
	}

	// Android project but no android_emulator.
	if hasType(detector.Android) && !used["android_emulator"] {
		report.Recommendations = append(report.Recommendations, Recommendation{
			Type:     "missing_assertion",
			Severity: SeverityInfo,
			Message:  "Consider adding android_emulator assertion — Android project detected",
		})
	}
}

// hasHTTPServer does a lightweight scan of Go source files for HTTP server patterns.
func hasHTTPServer(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		content := string(data)
		if strings.Contains(content, "http.ListenAndServe") ||
			strings.Contains(content, "http.ListenAndServeTLS") ||
			strings.Contains(content, "gin.Default()") ||
			strings.Contains(content, "echo.New()") ||
			strings.Contains(content, "fiber.New()") ||
			strings.Contains(content, "chi.NewRouter()") ||
			strings.Contains(content, "mux.NewRouter()") {
			return true
		}
	}
	// Also check cmd/ subdirectory.
	cmdDir := filepath.Join(dir, "cmd")
	entries, err = os.ReadDir(cmdDir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if !strings.HasSuffix(e.Name(), ".go") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(cmdDir, e.Name()))
		if err != nil {
			continue
		}
		content := string(data)
		if strings.Contains(content, "http.ListenAndServe") ||
			strings.Contains(content, "http.ListenAndServeTLS") {
			return true
		}
	}
	return false
}

// hasPkgScript checks if package.json has a named script.
func hasPkgScript(dir, script string) bool {
	data, err := os.ReadFile(filepath.Join(dir, "package.json"))
	if err != nil {
		return false
	}
	// Quick check without full JSON parse.
	return strings.Contains(string(data), fmt.Sprintf(`"%s"`, script))
}

// checkBaselineTests checks that the config has at least a build and a basic run test.
func checkBaselineTests(cfg *schema.SmokeConfig, report *Report) {
	hasBuild := false
	hasExitCode := false

	for _, t := range cfg.Tests {
		nameLower := strings.ToLower(t.Name)
		runLower := strings.ToLower(t.Run)

		// Build test: name or command references "build" or "compile".
		if strings.Contains(nameLower, "build") || strings.Contains(nameLower, "compil") ||
			strings.Contains(runLower, "build") || strings.Contains(runLower, "compil") {
			hasBuild = true
		}

		if t.Expect.ExitCode != nil {
			hasExitCode = true
		}
	}

	if hasBuild {
		report.Passes = append(report.Passes, "Build test present")
	} else {
		report.Recommendations = append(report.Recommendations, Recommendation{
			Type:     "missing_baseline",
			Severity: SeverityWarning,
			Message:  "Add a build test — no test with build/compile in name or command",
		})
	}

	if hasExitCode {
		report.Passes = append(report.Passes, "Exit code assertions present")
	} else {
		report.Recommendations = append(report.Recommendations, Recommendation{
			Type:     "missing_baseline",
			Severity: SeverityWarning,
			Message:  "Add exit_code assertions — no tests verify exit codes",
		})
	}
}

// calculateScore produces a 0-10 score based on findings.
func calculateScore(report *Report) int {
	if !report.ConfigExists {
		return 0
	}

	score := 10

	for _, r := range report.Recommendations {
		switch r.Severity {
		case SeverityWarning:
			score -= 2
		case SeverityInfo:
			score -= 1
		}
	}

	// Bonus: if tests exist.
	if report.TestCount == 0 {
		score -= 3
	}

	if score < 0 {
		score = 0
	}
	if score > 10 {
		score = 10
	}

	return score
}

// FormatTerminal renders the report as a human-readable terminal string.
func FormatTerminal(r *Report) string {
	var b strings.Builder

	b.WriteString("SmokeSig Audit Report\n")
	b.WriteString(strings.Repeat("━", 51) + "\n")

	b.WriteString(fmt.Sprintf("Project type:    %s (detected)\n", r.ProjectType))
	if r.ConfigExists {
		b.WriteString(fmt.Sprintf("Config:          %s (found)\n", filepath.Base(r.ConfigPath)))
	} else {
		b.WriteString(fmt.Sprintf("Config:          %s (not found)\n", filepath.Base(r.ConfigPath)))
	}
	b.WriteString(fmt.Sprintf("Tests:           %d defined\n", r.TestCount))
	b.WriteString(fmt.Sprintf("Assertions used: %d of %d available\n", r.AssertionsUsed, r.TotalAssertions))

	if len(r.Recommendations) > 0 || len(r.Passes) > 0 {
		b.WriteString("\nRecommendations:\n")
		for _, rec := range r.Recommendations {
			b.WriteString(fmt.Sprintf("  ⚠ %s\n", rec.Message))
		}
		for _, pass := range r.Passes {
			b.WriteString(fmt.Sprintf("  ✓ %s\n", pass))
		}
	}

	b.WriteString(fmt.Sprintf("\nScore: %d/10\n", r.Score))

	return b.String()
}
