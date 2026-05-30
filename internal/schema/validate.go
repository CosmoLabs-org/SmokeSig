package schema

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"
)

// ValidationError collects multiple validation failures.
type ValidationError struct {
	Errors []string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation failed:\n  - %s", strings.Join(e.Errors, "\n  - "))
}

// Validate checks a SmokeConfig for required fields and consistency.
// Returns all errors at once rather than stopping at the first.
func Validate(cfg *SmokeConfig) error {
	var errs []string

	if cfg.Version != 1 {
		errs = append(errs, fmt.Sprintf("unsupported version %d (expected 1)", cfg.Version))
	}

	if cfg.Project == "" {
		errs = append(errs, "project name is required")
	}

	if len(cfg.Tests) == 0 {
		errs = append(errs, "at least one test is required")
	}

	for i, t := range cfg.Tests {
		prefix := fmt.Sprintf("tests[%d]", i)
		if t.Name == "" {
			errs = append(errs, fmt.Sprintf("%s: name is required", prefix))
		}
		if t.Run == "" && !hasStandaloneAssertions(t.Expect) {
			errs = append(errs, fmt.Sprintf("%s: run command is required (or add a network/storage assertion)", prefix))
		}
		if t.Retry != nil {
			if t.Retry.Count < 1 {
				errs = append(errs, fmt.Sprintf("tests[%d] retry.count must be >= 1", i))
			}
			if t.Retry.Backoff.Duration <= 0 {
				errs = append(errs, fmt.Sprintf("tests[%d] retry.backoff must be > 0", i))
			}
			if t.Retry.RetryOnTraceOnly && t.Expect.OTelTrace == nil {
				errs = append(errs, fmt.Sprintf("tests[%d] retry.retry_on_trace_only requires otel_trace assertion", i))
			}
		}
		if t.Expect.DockerContainer != nil && t.Expect.DockerContainer.Name == "" {
			errs = append(errs, fmt.Sprintf("%s: docker_container_running.name is required", prefix))
		}
		if t.Expect.DockerImage != nil && t.Expect.DockerImage.Image == "" {
			errs = append(errs, fmt.Sprintf("%s: docker_image_exists.image is required", prefix))
		}
		if e := t.Expect.URLReachable; e != nil {
			if e.URL == "" {
				errs = append(errs, fmt.Sprintf("%s: url_reachable.url is required", prefix))
			} else if !strings.HasPrefix(e.URL, "http://") && !strings.HasPrefix(e.URL, "https://") {
				errs = append(errs, fmt.Sprintf("%s: url_reachable.url must start with http:// or https://", prefix))
			}
		}
		if e := t.Expect.ServiceReachable; e != nil {
			if e.URL == "" {
				errs = append(errs, fmt.Sprintf("%s: service_reachable.url is required", prefix))
			} else if !strings.HasPrefix(e.URL, "http://") && !strings.HasPrefix(e.URL, "https://") {
				errs = append(errs, fmt.Sprintf("%s: service_reachable.url must start with http:// or https://", prefix))
			}
		}
		if e := t.Expect.S3Bucket; e != nil {
			if e.Bucket == "" {
				errs = append(errs, fmt.Sprintf("%s: s3_bucket.bucket is required", prefix))
			}
		}
		if e := t.Expect.VersionCheck; e != nil {
			if e.Command == "" {
				errs = append(errs, fmt.Sprintf("%s: version_check.command is required", prefix))
			}
			if e.Pattern == "" {
				errs = append(errs, fmt.Sprintf("%s: version_check.pattern is required", prefix))
			} else if _, err := regexp.Compile(e.Pattern); err != nil {
				errs = append(errs, fmt.Sprintf("%s: version_check.pattern is invalid regex: %v", prefix, err))
			}
		}
		if e := t.Expect.WebSocket; e != nil {
			if e.URL == "" {
				errs = append(errs, fmt.Sprintf("%s: websocket.url is required", prefix))
			} else if !strings.HasPrefix(e.URL, "ws://") && !strings.HasPrefix(e.URL, "wss://") {
				errs = append(errs, fmt.Sprintf("%s: websocket.url must start with ws:// or wss://", prefix))
			}
			if e.ExpectMatches != "" {
				if _, err := regexp.Compile(e.ExpectMatches); err != nil {
					errs = append(errs, fmt.Sprintf("%s: websocket.expect_matches is invalid regex: %v", prefix, err))
				}
			}
		}
		if e := t.Expect.Credential; e != nil {
			if e.Source == "" {
				errs = append(errs, fmt.Sprintf("%s: credential_check.source is required", prefix))
			} else if e.Source != "env" && e.Source != "file" && e.Source != "exec" {
				errs = append(errs, fmt.Sprintf("%s: credential_check.source must be env, file, or exec", prefix))
			}
			if e.Name == "" {
				errs = append(errs, fmt.Sprintf("%s: credential_check.name is required", prefix))
			}
		}
		if e := t.Expect.GraphQL; e != nil {
			if e.URL == "" {
				errs = append(errs, fmt.Sprintf("%s: graphql.url is required", prefix))
			}
			if e.Query == "" && len(e.ExpectTypes) == 0 && e.ExpectContains == "" {
				// Standard introspection — nothing else required
			}
		}
		if e := t.Expect.OTelTrace; e != nil {
			if e.Backend != "" && e.Backend != "jaeger" && e.Backend != "tempo" && e.Backend != "honeycomb" && e.Backend != "datadog" {
				errs = append(errs, fmt.Sprintf("%s: otel_trace.backend must be jaeger, tempo, honeycomb, or datadog", prefix))
			}
			if e.Backend == "" || e.Backend == "jaeger" || e.Backend == "tempo" {
				if e.JaegerURL == "" && cfg.OTel.JaegerURL == "" {
					errs = append(errs, fmt.Sprintf("%s: otel_trace.jaeger_url is required (or set otel.jaeger_url globally)", prefix))
				} else if e.JaegerURL != "" && !strings.HasPrefix(e.JaegerURL, "http://") && !strings.HasPrefix(e.JaegerURL, "https://") {
					errs = append(errs, fmt.Sprintf("%s: otel_trace.jaeger_url must start with http:// or https://", prefix))
				}
			}
			if (e.Backend == "honeycomb" || e.Backend == "datadog") && e.JaegerURL == "" && cfg.OTel.JaegerURL == "" {
				errs = append(errs, fmt.Sprintf("%s: otel_trace.jaeger_url (collector URL) is required", prefix))
			}
			if e.Backend == "honeycomb" && e.APIKey == "" {
				errs = append(errs, fmt.Sprintf("%s: otel_trace.api_key is required for honeycomb backend", prefix))
			}
			if e.Backend == "datadog" && e.APIKey == "" {
				errs = append(errs, fmt.Sprintf("%s: otel_trace.api_key is required for datadog backend", prefix))
			}
			if e.MinSpans < 0 {
				errs = append(errs, fmt.Sprintf("%s: otel_trace.min_spans must be >= 0", prefix))
			}
		}

		if e := t.Expect.Ping; e != nil {
			if e.Host == "" {
				errs = append(errs, fmt.Sprintf("%s: ping.host is required", prefix))
			}
		}
		if e := t.Expect.Kafka; e != nil {
			if len(e.Brokers) == 0 {
				errs = append(errs, fmt.Sprintf("%s: kafka_broker.brokers is required", prefix))
			}
		}
		if e := t.Expect.LDAP; e != nil {
			if e.Host == "" {
				errs = append(errs, fmt.Sprintf("%s: ldap_bind.host is required", prefix))
			}
		}
		if e := t.Expect.MQTT; e != nil {
			if e.Broker == "" {
				errs = append(errs, fmt.Sprintf("%s: mqtt_ping.broker is required", prefix))
			}
		}
		if e := t.Expect.K8sResource; e != nil {
			if e.Namespace == "" {
				errs = append(errs, fmt.Sprintf("%s: k8s_resource.namespace is required", prefix))
			}
			if e.Kind == "" {
				errs = append(errs, fmt.Sprintf("%s: k8s_resource.kind is required", prefix))
			}
			if e.Name == "" {
				errs = append(errs, fmt.Sprintf("%s: k8s_resource.name is required", prefix))
			}
		}
			if e := t.Expect.FileSize; e != nil {
				if e.Path == "" {
					errs = append(errs, fmt.Sprintf("%s: file_size.path is required", prefix))
				}
				if e.MinBytes != nil && e.MaxBytes != nil && *e.MinBytes > *e.MaxBytes {
					errs = append(errs, fmt.Sprintf("%s: file_size.min_bytes must be <= max_bytes", prefix))
				}
			}
		if e := t.Expect.DocIntegrity; e != nil {
			if e.Binary == "" {
				errs = append(errs, fmt.Sprintf("%s: doc_integrity.binary is required", prefix))
			}
			if len(e.Docs) == 0 {
				errs = append(errs, fmt.Sprintf("%s: doc_integrity.docs is required (at least one doc file)", prefix))
			}
		}
	}
	if cfg.OTel.Enabled && cfg.OTel.JaegerURL == "" {
		errs = append(errs, "otel.jaeger_url is required when otel is enabled")
	}
	if cfg.OTel.Enabled && cfg.OTel.JaegerURL != "" && !strings.HasPrefix(cfg.OTel.JaegerURL, "http://") && !strings.HasPrefix(cfg.OTel.JaegerURL, "https://") {
		errs = append(errs, "otel.jaeger_url must start with http:// or https://")
	}

	// --- Auth validation ---
	if len(cfg.Auth.Profiles) > 0 {
		if cfg.Auth.Fallback != "" && cfg.Auth.Fallback != "env" && cfg.Auth.Fallback != "fail" {
			errs = append(errs, fmt.Sprintf("auth.fallback: invalid value %q (must be env or fail)", cfg.Auth.Fallback))
		}
		profileNames := make(map[string]bool)
		for i, p := range cfg.Auth.Profiles {
			prefix := fmt.Sprintf("auth.profiles[%d]", i)
			name := p.Name
			if name == "" {
				name = "default"
			}
			if profileNames[name] {
				errs = append(errs, fmt.Sprintf("auth.profiles: duplicate name %q", name))
			}
			profileNames[name] = true

			if p.Provider != "aws" && p.Provider != "gcp" {
				errs = append(errs, fmt.Sprintf("%s: unsupported provider %q (must be aws or gcp)", prefix, p.Provider))
			}
			if p.Provider == "aws" {
				if p.RoleARN == "" {
					errs = append(errs, fmt.Sprintf("%s: aws provider requires role_arn", prefix))
				} else if matched, _ := regexp.MatchString(`^arn:aws:iam::\d+:role/.+$`, p.RoleARN); !matched {
					errs = append(errs, fmt.Sprintf("%s: invalid role_arn format (expected arn:aws:iam::ACCOUNT:role/NAME)", prefix))
				}
			}
			if p.Provider == "gcp" {
				if p.WorkloadIdentityProvider == "" {
					errs = append(errs, fmt.Sprintf("%s: gcp provider requires workload_identity_provider", prefix))
				}
				if p.ServiceAccountEmail == "" {
					errs = append(errs, fmt.Sprintf("%s: gcp provider requires service_account_email", prefix))
				}
				if p.GCPCredentialFormat != "" && p.GCPCredentialFormat != "env" && p.GCPCredentialFormat != "keyfile" {
					errs = append(errs, fmt.Sprintf("%s: gcp_credential_format must be env or keyfile", prefix))
				}
			}
			if p.SessionDuration != "" {
				d, err := time.ParseDuration(p.SessionDuration)
				if err != nil {
					errs = append(errs, fmt.Sprintf("%s: invalid session_duration %q", prefix, p.SessionDuration))
				} else if p.Provider == "aws" && (d < 15*time.Minute || d > 12*time.Hour) {
					errs = append(errs, fmt.Sprintf("%s: session_duration must be between 15m and 12h for AWS", prefix))
				}
			}
		}

		// Validate test auth profile references
		for i, t := range cfg.Tests {
			if t.Auth != "" && !profileNames[t.Auth] {
				errs = append(errs, fmt.Sprintf("tests[%d]: auth profile %q not found", i, t.Auth))
			}
		}
	}

	for i, n := range cfg.Notifications {
		prefix := fmt.Sprintf("notifications[%d]", i)
		if n.URL == "" {
			errs = append(errs, fmt.Sprintf("%s: url is required", prefix))
		}
		if n.Format == "" {
			errs = append(errs, fmt.Sprintf("%s: format is required", prefix))
		} else if n.Format != "slack" && n.Format != "pagerduty" && n.Format != "json" {
			errs = append(errs, fmt.Sprintf("%s: format must be slack, pagerduty, or json", prefix))
		}
		if n.On != "" && n.On != "failure" && n.On != "always" && n.On != "change" {
			errs = append(errs, fmt.Sprintf("%s: on must be failure, always, or change", prefix))
		}
	}

	for i, h := range cfg.Lifecycle.BeforeAll {
		if h.Command == "" {
			errs = append(errs, fmt.Sprintf("lifecycle.before_all[%d]: command is required", i))
		}
		if msg := validateBackgroundHook(h, fmt.Sprintf("lifecycle.before_all[%d]", i)); msg != "" {
			errs = append(errs, msg)
		}
	}
	for i, h := range cfg.Lifecycle.AfterAll {
		if h.Command == "" {
			errs = append(errs, fmt.Sprintf("lifecycle.after_all[%d]: command is required", i))
		}
	}
	for i, h := range cfg.Lifecycle.BeforeEach {
		if h.Command == "" {
			errs = append(errs, fmt.Sprintf("lifecycle.before_each[%d]: command is required", i))
		}
		if msg := validateBackgroundHook(h, fmt.Sprintf("lifecycle.before_each[%d]", i)); msg != "" {
			errs = append(errs, msg)
		}
	}
	for i, h := range cfg.Lifecycle.AfterEach {
		if h.Command == "" {
			errs = append(errs, fmt.Sprintf("lifecycle.after_each[%d]: command is required", i))
		}
	}

	// Validate plugin assertions reference registered plugins
	for i, t := range cfg.Tests {
		for pluginName := range t.Expect.Plugin {
			if _, ok := cfg.Plugins[pluginName]; !ok {
				available := make([]string, 0, len(cfg.Plugins))
				for name := range cfg.Plugins {
					available = append(available, name)
				}
				sort.Strings(available)
				errs = append(errs, fmt.Sprintf(
					"tests[%d] %q: references unregistered plugin %q (available: %s)",
					i, t.Name, pluginName, strings.Join(available, ", "),
				))
			}
		}
	}

	// Validate plugin entries
	for name, entry := range cfg.Plugins {
		if entry.Path == "" {
			errs = append(errs, fmt.Sprintf("plugins.%s: path is required", name))
		}
		for _, cap := range entry.Capabilities {
			if !isValidPluginCapability(cap) {
				errs = append(errs, fmt.Sprintf("plugins.%s: unknown capability %q (valid: network, env, time, fs_read, exec)", name, cap))
			}
		}
		if hasPluginCapability(entry.Capabilities, "exec") && !cfg.Settings.AllowPluginExec {
			errs = append(errs, fmt.Sprintf("plugins.%s: exec capability requires settings.allow_plugin_exec: true", name))
		}
	}

	if len(errs) > 0 {
		return &ValidationError{Errors: errs}
	}
	return nil
}

func isValidPluginCapability(cap string) bool {
	switch cap {
	case "network", "env", "time", "fs_read", "exec":
		return true
	}
	return false
}

func hasPluginCapability(caps []string, target string) bool {
	for _, c := range caps {
		if c == target {
			return true
		}
	}
	return false
}

// hasStandaloneAssertions returns true if the test has assertions that don't
// require command output (stdout/stderr). These can run without a run command.
func hasStandaloneAssertions(e Expect) bool {
	return e.PortListening != nil ||
		e.ProcessRunning != "" ||
		e.HTTP != nil ||
		e.SSLCert != nil ||
		e.Redis != nil ||
		e.Memcached != nil ||
		e.Postgres != nil ||
		e.MySQL != nil ||
		e.GRPCHealth != nil ||
		e.DockerContainer != nil ||
		e.DockerImage != nil ||
		e.URLReachable != nil ||
		e.ServiceReachable != nil ||
		e.S3Bucket != nil ||
		e.VersionCheck != nil ||
		e.WebSocket != nil ||
		e.OTelTrace != nil ||
		e.Credential != nil ||
		e.GraphQL != nil ||
		e.DNS != nil ||
		e.SMTP != nil ||
		e.DockerCompose != nil ||
		e.Ping != nil ||
		e.Mongo != nil ||
		e.Kafka != nil ||
		e.LDAP != nil ||
		e.MQTT != nil ||
		e.NTP != nil ||
		e.K8sResource != nil ||
		e.IOSSimulator != nil ||
		e.AndroidEmulator != nil ||
		e.FileSize != nil ||
		e.DeepLink != nil ||
		e.DocIntegrity != nil ||
		len(e.Plugin) > 0
}

// validateBackgroundHook checks that background hooks have valid configuration.
func validateBackgroundHook(h LifecycleHook, path string) string {
	if h.Background && h.WaitForPort == 0 && h.Timeout.Duration == 0 && h.StartupTimeout.Duration == 0 {
		return fmt.Sprintf("%s: background=true requires either wait_for_port or a timeout", path)
	}
	return ""
}
