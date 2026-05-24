# MCP Server

SmokeSig includes a built-in [Model Context Protocol](https://modelcontextprotocol.io/) (MCP) server that exposes smoke test operations as tools. This allows AI assistants like Claude Desktop to run, validate, discover, and generate smoke tests through a structured interface.

## Overview

The MCP server communicates over stdio using the MCP JSON-RPC protocol. It registers 7 tools that cover the full smoke testing workflow: running tests, generating configs, validating configs, listing tests, discovering configs across a workspace, explaining assertion types, and generating test snippets.

All tool results are returned as structured JSON, and failed assertions include automatic fix suggestions based on pattern matching against common failure modes.

## Setup

### Claude Desktop

Add to your Claude Desktop configuration (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "SmokeSig": {
      "command": "smokesig",
      "args": ["mcp"]
    }
  }
}
```

### CLI

Start the MCP server manually (useful for debugging):

```bash
smokesig mcp
```

The server reads from stdin and writes to stdout using the MCP JSON-RPC protocol. It advertises tool capabilities on initialization.

## Tools Reference

### smoke_run

Run smoke tests from a `.smokesig.yaml` config file. Returns pass/fail results with assertion details for each test.

**Parameters:**

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `config_path` | string | No | `.smokesig.yaml` | Path to config file |
| `tags` | string[] | No | - | Include only tests with these tags |
| `exclude_tags` | string[] | No | - | Exclude tests with these tags |
| `fail_fast` | boolean | No | `false` | Stop on first failure |
| `timeout` | string | No | - | Per-test timeout override (e.g. `"30s"`) |
| `dry_run` | boolean | No | `false` | List tests without running them |
| `monorepo` | boolean | No | `false` | Discover and run configs in subdirectories |

**Response shape:**

```json
{
  "project": "my-api",
  "total": 5,
  "passed": 4,
  "failed": 1,
  "skipped": 0,
  "duration_ms": 1234,
  "config_path": "/path/to/.smokesig.yaml",
  "tests": [
    {
      "name": "API health check",
      "passed": true,
      "duration_ms": 45,
      "assertions": [
        {
          "type": "http",
          "expected": "status 200",
          "actual": "status 200",
          "passed": true
        }
      ]
    },
    {
      "name": "Redis connectivity",
      "passed": false,
      "duration_ms": 5002,
      "assertions": [
        {
          "type": "redis_ping",
          "expected": "+PONG",
          "actual": "connection refused",
          "passed": false
        }
      ],
      "fix_suggestions": [
        "Redis is not running or not listening on the configured host/port. Start Redis: docker run -d -p 6379:6379 redis:alpine"
      ]
    }
  ]
}
```

### smoke_init

Generate a `.smokesig.yaml` config for a project. Auto-detects Go, Node, Python, Docker, Rust, and 26 other project types. Can also inspect a running Docker container.

**Parameters:**

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `directory` | string | No | `.` | Project directory to scan |
| `from_container` | string | No | - | Inspect a running Docker container by name |
| `write` | boolean | No | `false` | Write `.smokesig.yaml` to disk |
| `force` | boolean | No | `false` | Overwrite existing config file |

**Response shape:**

```json
{
  "yaml": "project: my-api\ntests:\n  ...",
  "written": false,
  "write_path": ""
}
```

When `write=true`, the generated YAML is written to `<directory>/.smokesig.yaml` and `write_path` is populated.

### smoke_validate

Validate a `.smokesig.yaml` config without running tests. Checks required fields, assertion consistency, regex validity, and structural correctness. Returns all errors at once.

**Parameters:**

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `config_path` | string | No | `.smokesig.yaml` | Path to config file |

**Response shape (valid):**

```json
{
  "valid": true,
  "tests": ["health check", "redis ping", "build succeeds"]
}
```

**Response shape (invalid):**

```json
{
  "valid": false,
  "errors": [
    "test 0: name is required",
    "test 2: invalid regex in stdout_matches: missing closing )"
  ]
}
```

### smoke_list

List all smoke tests defined in a config. Shows test names, tags, command, and assertion types. Useful for understanding what is configured before running.

**Parameters:**

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `config_path` | string | No | `.smokesig.yaml` | Path to config file |
| `tags` | string[] | No | - | Filter to tests with these tags |
| `monorepo` | boolean | No | `false` | Discover configs in subdirectories |

**Response shape:**

```json
{
  "config_path": "/path/to/.smokesig.yaml",
  "tests": [
    {
      "name": "API health",
      "tags": ["core", "http"],
      "run_command": "curl -s http://localhost:8080/health",
      "assertion_types": ["http", "response_time_ms"],
      "skip_if": "env_unset:API_URL"
    }
  ]
}
```

### smoke_discover

Find all `.smokesig.yaml` config files in a directory tree. Returns paths and project names. Useful for understanding the test landscape of a workspace or monorepo.

**Parameters:**

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `directory` | string | No | `.` | Root directory to search |
| `depth` | number | No | unlimited | Maximum search depth |

**Response shape:**

```json
{
  "configs": [
    {
      "path": "/workspace/api/.smokesig.yaml",
      "directory": "/workspace/api",
      "project_name": "api"
    },
    {
      "path": "/workspace/web/.smokesig.yaml",
      "directory": "/workspace/web",
      "project_name": "web"
    }
  ]
}
```

### smoke_explain

Explain a smoke test assertion type and its configuration. Returns the assertion's fields, defaults, and an example YAML snippet. Use when you need to understand or construct assertion configurations.

**Parameters:**

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `assertion_type` | string | **Yes** | - | Assertion type to explain (e.g. `http`, `redis_ping`) |

**Response shape:**

```json
{
  "type": "redis_ping",
  "description": "Checks Redis connectivity by sending PING and expecting +PONG",
  "fields": [
    {"name": "host", "type": "string", "required": false, "default": "localhost", "description": "Redis host"},
    {"name": "port", "type": "int", "required": false, "default": "6379", "description": "Redis port"},
    {"name": "password", "type": "string", "required": false, "description": "Redis AUTH password"}
  ],
  "example_yaml": "tests:\n  - name: redis available\n    expect:\n      redis_ping:\n        host: localhost\n        port: 6379"
}
```

All 40+ assertion types are documented. See the main README for the full assertion type list.

### smoke_generate_test

Generate a single smoke test YAML snippet. Provide what you want to test and get back valid YAML to add to `.smokesig.yaml`. Supports all 40 assertion types.

**Parameters:**

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `name` | string | **Yes** | - | Test name |
| `assertion_type` | string | **Yes** | - | Primary assertion type (e.g. `http`, `port_listening`) |
| `description` | string | No | - | What this test should verify (natural language) |
| `params` | object | No | - | Assertion parameters as key-value pairs |
| `tags` | string[] | No | - | Tags for this test |

**Response shape:**

```json
{
  "yaml": "  - name: API health check\n    tags:\n      - core\n    expect:\n      http:\n        url: \"http://localhost:8080/health\"\n        status_code: 200\n"
}
```

**Example invocation:**

```json
{
  "name": "Redis available",
  "assertion_type": "redis_ping",
  "params": {"host": "redis.internal", "port": 6380},
  "tags": ["infra", "redis"]
}
```

Assertion types that require a `run:` command (like `exit_code`, `stdout_contains`) will include a `run: <command>` placeholder. Standalone assertion types (like `http`, `port_listening`, `redis_ping`) omit it.

## Fix Suggestions

When tests fail, the MCP server automatically attaches fix suggestions to the response. Suggestions are pattern-matched against the actual assertion output. Coverage includes common failure patterns for:

- **Infrastructure:** `redis_ping`, `postgres_ping`, `mysql_ping`, `memcached_version`
- **HTTP/Network:** `http`, `port_listening`, `url_reachable`, `grpc_health`, `websocket`
- **Security:** `ssl_cert`, `credential_check`, `s3_bucket`
- **Output:** `exit_code`, `stdout_contains`, `stderr_contains`
- **Docker:** `docker_container_running`, `docker_image_exists`
- **Observability:** `otel_trace`, `graphql`, `version_check`

For example, a `redis_ping` failure with "connection refused" in the output produces: `"Redis is not running or not listening on the configured host/port. Start Redis: docker run -d -p 6379:6379 redis:alpine"`.

## Architecture

```
internal/mcp/
  server.go       # MCP server setup, tool registration, stdio serving
  handlers.go     # Tool handler implementations (7 handlers)
  types.go        # Request/response type definitions
  assertions.go   # Assertion type documentation database (for smoke_explain)
  suggestions.go  # Fix suggestion pattern matching engine
```

The server is built on `github.com/mark3labs/mcp-go` and uses the `server.MCPServer` with tool capabilities enabled. Each tool handler receives parsed arguments as `map[string]interface{}` and returns a structured result that is serialized to JSON.
