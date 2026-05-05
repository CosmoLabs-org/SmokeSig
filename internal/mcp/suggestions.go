package mcp

import "strings"

// suggestionRule maps a failure pattern to a fix suggestion.
type suggestionRule struct {
	Match  string // substring to match in Actual
	Action string // concrete fix suggestion
}

// assertionSuggestions maps assertion types to their fix suggestion rules.
var assertionSuggestions = map[string][]suggestionRule{
	"redis_ping": {
		{Match: "connection refused", Action: "Redis is not running or not listening on the configured host/port. Start Redis: docker run -d -p 6379:6379 redis:alpine"},
		{Match: "auth", Action: "Redis requires authentication. Add redis_ping.password to .smokesig.yaml"},
		{Match: "timeout", Action: "Redis connection timed out. Check if Redis is reachable and not behind a firewall"},
	},
	"http": {
		{Match: "connection refused", Action: "Server is not listening on the expected address. Check if the service is running and the port is correct"},
		{Match: "status code", Action: "Unexpected HTTP status code. Verify the endpoint returns the expected status (check for redirects, auth requirements, or error responses)"},
		{Match: "timeout", Action: "HTTP request timed out. Increase timeout or check if the service is overloaded"},
		{Match: "tls", Action: "TLS handshake failed. Check certificate validity and protocol version"},
		{Match: "body", Action: "Response body doesn't contain expected content. Check if the API response format changed"},
	},
	"port_listening": {
		{Match: "not open", Action: "Start the service that should listen on this port"},
		{Match: "connection refused", Action: "Port is not open. The service may not be started or is listening on a different address"},
		{Match: "timeout", Action: "Connection attempt timed out. Check firewall rules and service status"},
	},
	"postgres_ping": {
		{Match: "connection refused", Action: "Postgres is not running. Start it: docker run -d -p 5432:5432 -e POSTGRES_PASSWORD=postgres postgres:alpine"},
		{Match: "timeout", Action: "Postgres connection timed out. Check if the service is reachable"},
	},
	"mysql_ping": {
		{Match: "connection refused", Action: "MySQL is not running. Start it: docker run -d -p 3306:3306 -e MYSQL_ROOT_PASSWORD=root mysql:8"},
		{Match: "timeout", Action: "MySQL connection timed out. Check if the service is reachable"},
	},
	"memcached_version": {
		{Match: "connection refused", Action: "Memcached is not running. Start it: docker run -d -p 11211:11211 memcached:alpine"},
	},
	"ssl_cert": {
		{Match: "expired", Action: "TLS certificate has expired. Renew the certificate"},
		{Match: "self-signed", Action: "Certificate is self-signed. Add allow_self_signed: true if this is expected"},
		{Match: "days remaining", Action: "Certificate is close to expiry. Renew it before it expires"},
		{Match: "not yet valid", Action: "Certificate is not yet valid. Check system clock synchronization"},
	},
	"exit_code": {
		{Match: "exit code", Action: "Command returned unexpected exit code. Run the command manually to see the error output"},
	},
	"stdout_contains": {
		{Match: "not found", Action: "Expected text not found in stdout. Run the command manually to inspect output"},
	},
	"stderr_contains": {
		{Match: "not found", Action: "Expected text not found in stderr. Run the command manually to inspect error output"},
	},
	"docker_container_running": {
		{Match: "not found", Action: "Container is not running. Start it with docker start <name> or docker compose up"},
	},
	"docker_image_exists": {
		{Match: "not found", Action: "Docker image not found locally. Pull it with docker pull <image>"},
	},
	"grpc_health": {
		{Match: "connection refused", Action: "gRPC server is not listening. Check if the service is running"},
		{Match: "not serving", Action: "gRPC health check returned non-SERVING status. The service may be initializing or degraded"},
	},
	"url_reachable": {
		{Match: "connection refused", Action: "URL is not reachable. Check if the service is running and the URL is correct"},
		{Match: "timeout", Action: "Request timed out. Check network connectivity and service responsiveness"},
	},
	"websocket": {
		{Match: "connection refused", Action: "WebSocket server is not reachable. Check if the service is running"},
		{Match: "timeout", Action: "WebSocket response timed out. The server may not be sending the expected message"},
	},
	"credential_check": {
		{Match: "not set", Action: "Credential is not set. Set the environment variable or create the file"},
		{Match: "not found", Action: "Credential file not found. Create it or update the path"},
	},
	"s3_bucket": {
		{Match: "no such bucket", Action: "S3 bucket does not exist. Create it or check the bucket name"},
		{Match: "access denied", Action: "Access denied to S3 bucket. Check IAM permissions or endpoint configuration"},
	},
	"version_check": {
		{Match: "not found", Action: "Command not found. Install the tool or check the command path"},
		{Match: "no match", Action: "Version pattern didn't match. Run the command manually to check output format"},
	},
	"graphql": {
		{Match: "connection refused", Action: "GraphQL server is not reachable. Check if the service is running"},
		{Match: "not found", Action: "Expected types not found in GraphQL schema. Check if the schema has been updated"},
	},
	"otel_trace": {
		{Match: "no traces", Action: "No traces found. Check if the service is instrumented and sending traces to the collector"},
		{Match: "timeout", Action: "Trace lookup timed out. The collector may be slow or unreachable"},
	},
}

// GetSuggestions returns fix suggestions for a failed assertion.
func GetSuggestions(assertionType, actual string) []string {
	rules, ok := assertionSuggestions[assertionType]
	if !ok {
		return []string{"Check the configuration for this assertion type"}
	}

	var suggestions []string
	lower := strings.ToLower(actual)
	for _, rule := range rules {
		if strings.Contains(lower, strings.ToLower(rule.Match)) {
			suggestions = append(suggestions, rule.Action)
		}
	}

	if len(suggestions) == 0 {
		suggestions = append(suggestions, "Check the configuration for this assertion type")
	}

	return suggestions
}
