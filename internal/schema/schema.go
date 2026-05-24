package schema

import (
	"bytes"
	"fmt"
	"os"
	"text/template"
	"time"

	"gopkg.in/yaml.v3"
)

// SmokeConfig is the top-level configuration parsed from .smokesig.yaml.
type SmokeConfig struct {
	Version     int             `yaml:"version"`
	Project     string          `yaml:"project"`
	Description string          `yaml:"description,omitempty"`
	Extends     string          `yaml:"extends,omitempty"`
	Includes    []string        `yaml:"includes,omitempty"`
	Settings    Settings        `yaml:"settings,omitempty"`
	OTel        OTelConfig      `yaml:"otel,omitempty"`
	Prereqs     []Prerequisite  `yaml:"prerequisites,omitempty"`
	Lifecycle   LifecycleConfig `yaml:"lifecycle,omitempty"`
	Tests       []Test          `yaml:"tests"`
}

// Settings controls global test behavior.
type Settings struct {
	Timeout         Duration `yaml:"timeout,omitempty"`
	FailFast        bool     `yaml:"fail_fast,omitempty"`
	Parallel        bool     `yaml:"parallel,omitempty"`
	Monorepo        bool     `yaml:"monorepo,omitempty"`
	MonorepoExclude []string `yaml:"monorepo_exclude,omitempty"`
}

// OTelConfig configures OpenTelemetry trace context propagation and telemetry export.
type OTelConfig struct {
	Enabled          bool   `yaml:"enabled,omitempty"`
	JaegerURL        string `yaml:"jaeger_url,omitempty"`
	ServiceName      string `yaml:"service_name,omitempty"`
	TracePropagation bool   `yaml:"trace_propagation,omitempty"`
	ExportURL        string `yaml:"export_url,omitempty"` // OTLP HTTP endpoint for emitting telemetry (defaults to jaeger_url + /v1/traces)
	ExportHeaders    map[string]string `yaml:"export_headers,omitempty"` // Additional headers for OTLP export (e.g., auth)
}

// Prerequisite is a command that must succeed before tests run.
type Prerequisite struct {
	Name  string `yaml:"name"`
	Check string `yaml:"check"`
	Hint  string `yaml:"hint,omitempty"`
}

// LifecycleConfig defines setup/teardown hooks for the test suite.
type LifecycleConfig struct {
	BeforeAll  []LifecycleHook `yaml:"before_all,omitempty"`
	AfterAll   []LifecycleHook `yaml:"after_all,omitempty"`
	BeforeEach []LifecycleHook `yaml:"before_each,omitempty"`
	AfterEach  []LifecycleHook `yaml:"after_each,omitempty"`
}

// LifecycleHook is a command that runs at a specific lifecycle point.
type LifecycleHook struct {
	Command        string   `yaml:"command"`
	Timeout        Duration `yaml:"timeout,omitempty"`
	AlwaysRun      bool     `yaml:"always_run,omitempty"`
	EnvPass        bool     `yaml:"env_pass,omitempty"`
	Background     bool     `yaml:"background,omitempty"`
	WaitForPort    int      `yaml:"wait_for_port,omitempty"`
	StartupTimeout Duration `yaml:"startup_timeout,omitempty"`
}

// RetryPolicy configures automatic retry for flaky tests.
type RetryPolicy struct {
	Count           int      `yaml:"count"`
	Backoff         Duration `yaml:"backoff"`
	RetryOnTraceOnly bool    `yaml:"retry_on_trace_only,omitempty"`
}

// Test defines a single smoke test.
type Test struct {
	Name         string       `yaml:"name"`
	Run          string       `yaml:"run"`
	Expect       Expect       `yaml:"expect"`
	Tags         []string     `yaml:"tags,omitempty"`
	Timeout      Duration     `yaml:"timeout,omitempty"`
	Cleanup      string       `yaml:"cleanup,omitempty"`
	AllowFailure bool         `yaml:"allow_failure,omitempty"`
	Retry        *RetryPolicy `yaml:"retry,omitempty"`
	SkipIf       *SkipIf      `yaml:"skip_if,omitempty"`
}

// SkipIf defines conditions under which a test should be skipped.
type SkipIf struct {
	EnvUnset  string         `yaml:"env_unset,omitempty"`
	EnvEquals *EnvEqualsCond `yaml:"env_equals,omitempty"`
	FileMissing string       `yaml:"file_missing,omitempty"`
}

// EnvEqualsCond checks if an env var equals a specific value.
type EnvEqualsCond struct {
	Var   string `yaml:"var"`
	Value string `yaml:"value"`
}

// Expect defines the assertions for a test.
type Expect struct {
	ExitCode       *int            `yaml:"exit_code,omitempty"`
	StdoutContains string          `yaml:"stdout_contains,omitempty"`
	StdoutMatches  string          `yaml:"stdout_matches,omitempty"`
	StderrContains string          `yaml:"stderr_contains,omitempty"`
	StderrMatches  string          `yaml:"stderr_matches,omitempty"`
	FileExists     string          `yaml:"file_exists,omitempty"`
	FileSize       *FileSizeCheck  `yaml:"file_size,omitempty"`
	EnvExists      string          `yaml:"env_exists,omitempty"`
	PortListening  *PortCheck      `yaml:"port_listening,omitempty"`
	ProcessRunning string          `yaml:"process_running,omitempty"`
	HTTP           *HTTPCheck      `yaml:"http,omitempty"`
	JSONField      *JSONFieldCheck  `yaml:"json_field,omitempty"`
	ResponseTimeMs *int             `yaml:"response_time_ms,omitempty"` // Fail if test duration exceeds this many ms
	SSLCert        *SSLCertCheck    `yaml:"ssl_cert,omitempty"`
	Redis          *RedisCheck      `yaml:"redis_ping,omitempty"`
	Memcached      *MemcachedCheck  `yaml:"memcached_version,omitempty"`
	Postgres       *PostgresCheck   `yaml:"postgres_ping,omitempty"`
	MySQL          *MySQLCheck      `yaml:"mysql_ping,omitempty"`
	GRPCHealth      *GRPCHealthCheck      `yaml:"grpc_health,omitempty"`
	DockerContainer  *DockerContainerCheck  `yaml:"docker_container_running,omitempty"`
	DockerImage      *DockerImageCheck      `yaml:"docker_image_exists,omitempty"`
	URLReachable     *URLReachableCheck     `yaml:"url_reachable,omitempty"`
	ServiceReachable *ServiceReachableCheck `yaml:"service_reachable,omitempty"`
	S3Bucket         *S3BucketCheck         `yaml:"s3_bucket,omitempty"`
	VersionCheck     *VersionCheck          `yaml:"version_check,omitempty"`
	WebSocket        *WebSocketCheck        `yaml:"websocket,omitempty"`
	OTelTrace        *OTelTraceCheck        `yaml:"otel_trace,omitempty"`
	Credential       *CredentialCheck       `yaml:"credential_check,omitempty"`
	GraphQL          *GraphQLCheck          `yaml:"graphql,omitempty"`
	DeepLink         *DeepLinkCheck         `yaml:"deep_link,omitempty"`
	DNS              *DNSCheck              `yaml:"dns_resolve,omitempty"`
	SMTP             *SMTPCheck             `yaml:"smtp_ping,omitempty"`
	DockerCompose    *DockerComposeCheck    `yaml:"docker_compose_healthy,omitempty"`
	Ping             *PingCheck             `yaml:"ping,omitempty"`
	Mongo            *MongoCheck            `yaml:"mongo_ping,omitempty"`
	Kafka            *KafkaCheck            `yaml:"kafka_broker,omitempty"`
	LDAP             *LDAPCheck             `yaml:"ldap_bind,omitempty"`
	MQTT             *MQTTCheck             `yaml:"mqtt_ping,omitempty"`
	NTP              *NTPCheck              `yaml:"ntp_check,omitempty"`
	K8sResource      *K8sResourceCheck      `yaml:"k8s_resource,omitempty"`
	IOSSimulator     *IOSSimulatorCheck     `yaml:"ios_simulator,omitempty"`
	AndroidEmulator  *AndroidEmulatorCheck  `yaml:"android_emulator,omitempty"`
	DocIntegrity     *DocIntegrityCheck     `yaml:"doc_integrity,omitempty"`
	Extract          string                 `yaml:"extract,omitempty"` // Variable name to capture from stdout_matches
}

// FileSizeCheck verifies a file exists and optionally checks its size thresholds.
type FileSizeCheck struct {
	Path     string `yaml:"path"`
	MinBytes *int64 `yaml:"min_bytes,omitempty"`
	MaxBytes *int64 `yaml:"max_bytes,omitempty"`
}

// PortCheck defines parameters for checking if a port is open and listening.
type PortCheck struct {
	Port     int    `yaml:"port"`
	Protocol string `yaml:"protocol,omitempty"`
	Host     string `yaml:"host,omitempty"`
}

// SSLCertCheck defines parameters for TLS certificate validation.
type SSLCertCheck struct {
	Host             string `yaml:"host"`
	Port             int    `yaml:"port,omitempty"`               // defaults to 443
	MinDaysRemaining int    `yaml:"min_days_remaining,omitempty"` // 0 = any non-expired cert passes
	AllowSelfSigned  bool   `yaml:"allow_self_signed,omitempty"`
}

// RedisCheck pings a Redis server with PING and verifies PONG reply.
type RedisCheck struct {
	Host     string `yaml:"host,omitempty"`     // default "localhost"
	Port     int    `yaml:"port,omitempty"`     // default 6379
	Password string `yaml:"password,omitempty"` // optional AUTH
}

// MemcachedCheck issues `version` to a Memcached server and expects a VERSION reply.
type MemcachedCheck struct {
	Host string `yaml:"host,omitempty"` // default "localhost"
	Port int    `yaml:"port,omitempty"` // default 11211
}

// PostgresCheck pings a Postgres server via SSLRequest handshake.
type PostgresCheck struct {
	Host string `yaml:"host,omitempty"` // default "localhost"
	Port int    `yaml:"port,omitempty"` // default 5432
}

// MySQLCheck verifies a MySQL server sends a valid handshake on connection.
type MySQLCheck struct {
	Host string `yaml:"host,omitempty"` // default "localhost"
	Port int    `yaml:"port,omitempty"` // default 3306
}

// DockerContainerCheck verifies a named Docker container is running.
type DockerContainerCheck struct {
	Name string `yaml:"name"`
}

// DockerImageCheck verifies a Docker image exists locally.
type DockerImageCheck struct {
	Image string `yaml:"image"`
}

// HTTPCheck defines parameters for HTTP endpoint assertions.
type HTTPCheck struct {
	URL            string            `yaml:"url"`
	Method         string            `yaml:"method,omitempty"`
	Headers        map[string]string `yaml:"headers,omitempty"`
	Body           string            `yaml:"body,omitempty"`
	Timeout        Duration          `yaml:"timeout,omitempty"`
	StatusCode     *int              `yaml:"status_code,omitempty"`
	BodyContains   string            `yaml:"body_contains,omitempty"`
	BodyMatches    string            `yaml:"body_matches,omitempty"`
	HeaderContains map[string]string `yaml:"header_contains,omitempty"`
}

// GRPCHealthCheck queries the grpc.health.v1.Health/Check endpoint.
type GRPCHealthCheck struct {
	Address  string            `yaml:"address"`            // host:port
	Service  string            `yaml:"service,omitempty"`  // "" = overall server health
	UseTLS   bool              `yaml:"use_tls,omitempty"`  // default false (insecure)
	Timeout  Duration          `yaml:"timeout,omitempty"`  // default 5s
	Metadata map[string]string `yaml:"-"`                  // runtime-only, injected by runner
}

// JSONFieldCheck defines parameters for asserting on JSON fields in stdout.
type JSONFieldCheck struct {
	Path     string `yaml:"path"`
	Equals   string `yaml:"equals,omitempty"`
	Contains string `yaml:"contains,omitempty"`
	Matches  string `yaml:"matches,omitempty"`
	Extract  string `yaml:"extract,omitempty"` // Variable name to capture the matched value
}

// URLReachableCheck verifies an HTTP/HTTPS endpoint is accessible.
type URLReachableCheck struct {
	URL        string   `yaml:"url"`
	Timeout    Duration `yaml:"timeout,omitempty"`
	StatusCode *int     `yaml:"status_code,omitempty"`
}

// ServiceReachableCheck verifies an external service dependency is accessible.
type ServiceReachableCheck struct {
	URL     string   `yaml:"url"`
	Timeout Duration `yaml:"timeout,omitempty"`
}

// S3BucketCheck verifies an S3-compatible bucket is accessible via anonymous HEAD.
type S3BucketCheck struct {
	Bucket   string `yaml:"bucket"`
	Region   string `yaml:"region,omitempty"`
	Endpoint string `yaml:"endpoint,omitempty"`
}

// VersionCheck verifies an installed tool matches a required version pattern.
type VersionCheck struct {
	Command string `yaml:"command"`
	Pattern string `yaml:"pattern"`
}

// WebSocketCheck verifies a WebSocket endpoint is reachable and responds as expected.
type WebSocketCheck struct {
	URL            string            `yaml:"url"`
	Send           string            `yaml:"send,omitempty"`
	ExpectContains string            `yaml:"expect_contains,omitempty"`
	ExpectMatches  string            `yaml:"expect_matches,omitempty"`
	Timeout        Duration          `yaml:"timeout,omitempty"`
	Headers        map[string]string `yaml:"headers,omitempty"`
}

// OTelTraceCheck verifies that a trace arrived at a trace collector.
// Supported backends: jaeger (default), tempo, honeycomb, datadog.
type OTelTraceCheck struct {
	Backend     string   `yaml:"backend,omitempty"` // jaeger | tempo | honeycomb | datadog
	JaegerURL   string   `yaml:"jaeger_url,omitempty"`
	ServiceName string   `yaml:"service_name,omitempty"`
	MinSpans    int      `yaml:"min_spans,omitempty"`
	Timeout     Duration `yaml:"timeout,omitempty"`
	APIKey      string   `yaml:"api_key,omitempty"`      // Honeycomb/Datadog API key
	DDAppKey    string   `yaml:"dd_app_key,omitempty"`   // Datadog application key (optional)
}

// CredentialCheck verifies a credential is accessible without leaking its value.
// Source can be "env" (env var), "file" (file path), or "exec" (shell command).
type CredentialCheck struct {
	Source   string `yaml:"source"`             // env | file | exec
	Name     string `yaml:"name"`               // env var name, file path, or command
	Contains string `yaml:"contains,omitempty"` // optional: value must contain this substring
}

// GraphQLCheck verifies a GraphQL endpoint is introspectable and returns expected types.
type GraphQLCheck struct {
	URL            string   `yaml:"url"`
	Query          string   `yaml:"query,omitempty"`            // custom query (default: full introspection)
	StatusCode     *int     `yaml:"status_code,omitempty"`      // expected HTTP status (default 200)
	ExpectTypes    []string `yaml:"expect_types,omitempty"`     // types that must exist in schema
	ExpectContains string   `yaml:"expect_contains,omitempty"`  // response body must contain substring
	Timeout        Duration `yaml:"timeout,omitempty"`          // HTTP client timeout
}

// DeepLinkCheck verifies mobile deep link / universal link configuration.
// Two-tier: Tier 1 uses HTTP/config checks (zero-dep); Tier 2 uses adb/xcrun when available.
type DeepLinkCheck struct {
	URL                  string   `yaml:"url"`
	AndroidPackage       string   `yaml:"android_package,omitempty"`
	IOSBundleID          string   `yaml:"ios_bundle_id,omitempty"`
	IOSAssociatedDomains []string `yaml:"ios_associated_domains,omitempty"`
	CheckAssetlinks      *bool    `yaml:"check_assetlinks,omitempty"`
	CheckAASA            *bool    `yaml:"check_aasa,omitempty"`
	Tier                 string   `yaml:"tier,omitempty"` // auto (default) | config-only | full-resolve
}

// DNSCheck verifies DNS resolution for a hostname.
type DNSCheck struct {
	Hostname    string   `yaml:"hostname"`
	RecordType  string   `yaml:"record_type,omitempty"` // A (default), AAAA, TXT, MX, CNAME
	ExpectedIP  string   `yaml:"expected_ip,omitempty"`
	Timeout     Duration `yaml:"timeout,omitempty"`
}

// SMTPCheck verifies an SMTP server is accepting connections.
type SMTPCheck struct {
	Host    string   `yaml:"host"`
	Port    int      `yaml:"port,omitempty"`
	Timeout Duration `yaml:"timeout,omitempty"`
}

// DockerComposeCheck verifies Docker Compose services are healthy.
type DockerComposeCheck struct {
	ComposeFile string   `yaml:"compose_file,omitempty"`
	Services    []string `yaml:"services,omitempty"`
	Timeout     Duration `yaml:"timeout,omitempty"`
}

// PingCheck verifies a host responds to ICMP echo requests.
type PingCheck struct {
	Host    string   `yaml:"host"`
	Count   int      `yaml:"count,omitempty"`
	Timeout Duration `yaml:"timeout,omitempty"`
}

// MongoCheck verifies a MongoDB server responds to the isMaster command.
type MongoCheck struct {
	Host        string `yaml:"host,omitempty"`
	Port        int    `yaml:"port,omitempty"`
	Username    string `yaml:"username,omitempty"`
	PasswordEnv string `yaml:"password_env,omitempty"`
}

// KafkaCheck verifies a Kafka broker responds to a metadata request.
type KafkaCheck struct {
	Brokers []string `yaml:"brokers"`
	Topic   string   `yaml:"topic,omitempty"`
	Timeout Duration `yaml:"timeout,omitempty"`
}

// LDAPCheck verifies an LDAP server accepts bind requests.
type LDAPCheck struct {
	Host        string   `yaml:"host"`
	Port        int      `yaml:"port,omitempty"`
	BindDN      string   `yaml:"bind_dn,omitempty"`
	PasswordEnv string   `yaml:"password_env,omitempty"`
	UseTLS      bool     `yaml:"use_tls,omitempty"`
	Timeout     Duration `yaml:"timeout,omitempty"`
}

// MQTTCheck verifies an MQTT broker accepts connections.
type MQTTCheck struct {
	Broker      string   `yaml:"broker"`
	ClientID    string   `yaml:"client_id,omitempty"`
	Username    string   `yaml:"username,omitempty"`
	PasswordEnv string   `yaml:"password_env,omitempty"`
	Timeout     Duration `yaml:"timeout,omitempty"`
}

// NTPCheck verifies an NTP server responds with valid time data.
type NTPCheck struct {
	Server       string   `yaml:"server,omitempty"`
	MaxOffsetMs  int      `yaml:"max_offset_ms,omitempty"`
	Timeout      Duration `yaml:"timeout,omitempty"`
}

// IOSSimulatorCheck verifies an iOS simulator is booted and ready.
type IOSSimulatorCheck struct {
	DeviceName string   `yaml:"device_name" json:"device_name,omitempty"` // optional filter by device name
	OS         string   `yaml:"os" json:"os,omitempty"`                   // optional filter by OS version
	Timeout    Duration `yaml:"timeout" json:"timeout,omitempty"`
}

// AndroidEmulatorCheck verifies an Android emulator has completed booting.
type AndroidEmulatorCheck struct {
	Serial  string   `yaml:"serial" json:"serial,omitempty"`   // optional ADB serial for specific device
	Timeout Duration `yaml:"timeout" json:"timeout,omitempty"`
}

// DocIntegrityCheck verifies CLI documentation stays in sync with actual commands and flags.
type DocIntegrityCheck struct {
	Binary         string   `yaml:"binary" json:"binary"`
	Docs           []string `yaml:"docs" json:"docs"`
	CheckExamples  bool     `yaml:"check_examples" json:"check_examples,omitempty"`
	IgnoreCommands []string `yaml:"ignore_commands" json:"ignore_commands,omitempty"`
	Timeout        Duration `yaml:"timeout" json:"timeout,omitempty"`
}

// K8sResourceCheck verifies a Kubernetes resource exists and optionally meets a condition.
type K8sResourceCheck struct {
	Context   string   `yaml:"context,omitempty"`
	Namespace string   `yaml:"namespace"`
	Kind      string   `yaml:"kind"`
	Name      string   `yaml:"name"`
	Condition string   `yaml:"condition,omitempty"`
	Timeout   Duration `yaml:"timeout,omitempty"`
}

// Duration wraps time.Duration for YAML unmarshaling from strings like "5s".
type Duration struct {
	time.Duration
}

func (d *Duration) UnmarshalYAML(value *yaml.Node) error {
	var s string
	if err := value.Decode(&s); err != nil {
		return err
	}
	parsed, err := time.ParseDuration(s)
	if err != nil {
		return fmt.Errorf("invalid duration %q: %w", s, err)
	}
	d.Duration = parsed
	return nil
}

func (d Duration) MarshalYAML() (interface{}, error) {
	return d.Duration.String(), nil
}

// Load reads and parses a .smokesig.yaml (or legacy .smoke.yaml) file from the given path.
// Supports Go templates ({{ .Env.FOO }}), extends, and includes.
func Load(path string) (*SmokeConfig, error) {
	return LoadWithResolver(path, nil)
}

// processTemplate expands Go templates in the config.
// Available: .Env (environment variables map)
func processTemplate(data []byte) ([]byte, error) {
	tmpl, err := template.New("config").Parse(string(data))
	if err != nil {
		return nil, err
	}

	// Build template data
	envMap := make(map[string]string)
	for _, e := range os.Environ() {
		if idx := bytes.IndexByte([]byte(e), '='); idx > 0 {
			envMap[e[:idx]] = e[idx+1:]
		}
	}

	var buf bytes.Buffer
	err = tmpl.Execute(&buf, map[string]any{
		"Env": envMap,
	})
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// Parse parses raw YAML bytes into a SmokeConfig.
func Parse(data []byte) (*SmokeConfig, error) {
	var cfg SmokeConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}
	return &cfg, nil
}

// LoadDefault finds and loads .smokesig.yaml from the current directory.
// Falls back to .smoke.yaml with a deprecation warning for backward compat.
func LoadDefault() (*SmokeConfig, error) {
	if _, err := os.Stat(".smokesig.yaml"); err == nil {
		return Load(".smokesig.yaml")
	}
	if _, err := os.Stat(".smoke.yaml"); err == nil {
		fmt.Fprintln(os.Stderr, "⚠ Config file .smoke.yaml is deprecated, rename to .smokesig.yaml")
		return Load(".smoke.yaml")
	}
	return nil, fmt.Errorf("no config file found: .smokesig.yaml or .smoke.yaml")
}

// MergeEnv loads an environment-specific config and deep-merges it onto base.
// Env-specific tests are appended (not replaced). Settings from env override base.
func MergeEnv(base *SmokeConfig, envPath string) (*SmokeConfig, error) {
	envCfg, err := Load(envPath)
	if err != nil {
		return nil, fmt.Errorf("loading env config %s: %w", envPath, err)
	}

	// Deep merge: env settings override base
	if envCfg.Settings.Timeout.Duration > 0 {
		base.Settings.Timeout = envCfg.Settings.Timeout
	}
	if envCfg.Settings.FailFast {
		base.Settings.FailFast = true
	}
	if envCfg.Settings.Parallel {
		base.Settings.Parallel = true
	}

	// Prepend env prereqs (they run before base prereqs)
	base.Prereqs = append(envCfg.Prereqs, base.Prereqs...)

	// Append env tests (they run after base tests)
	base.Tests = append(base.Tests, envCfg.Tests...)

	return base, nil
}
