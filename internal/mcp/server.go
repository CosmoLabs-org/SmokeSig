package mcp

import (
	"context"
	"encoding/json"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// Server wraps the mcp-go MCPServer with cosmo-smoke tool registration.
type Server struct {
	mcp       *server.MCPServer
	handlers  map[string]ToolHandler
}

// NewServer creates a new MCP server with all smoke tools registered.
func NewServer() *Server {
	mcpServer := server.NewMCPServer(
		"cosmo-smoke",
		"0.9.0",
		server.WithToolCapabilities(true),
	)

	s := &Server{
		mcp:      mcpServer,
		handlers: make(map[string]ToolHandler),
	}
	s.registerTools()
	return s
}

// Handler returns the handler for a named tool, or nil if not found.
func (s *Server) Handler(name string) ToolHandler {
	return s.handlers[name]
}

// ToolNames returns the names of all registered tools.
func (s *Server) ToolNames() []string {
	names := make([]string, 0, len(s.handlers))
	for name := range s.handlers {
		names = append(names, name)
	}
	return names
}

// MCPServer returns the underlying mcp-go server for stdio serving.
func (s *Server) MCPServer() *server.MCPServer {
	return s.mcp
}

// ServeStdio runs the MCP server over stdin/stdout.
func (s *Server) ServeStdio() error {
	return server.ServeStdio(s.mcp)
}

// registerTools adds all 7 smoke tool handlers.
func (s *Server) registerTools() {
	s.addTool("smoke_run", smokeRunTool(), handleSmokeRun)
	s.addTool("smoke_init", smokeInitTool(), handleSmokeInit)
	s.addTool("smoke_validate", smokeValidateTool(), handleSmokeValidate)
	s.addTool("smoke_list", smokeListTool(), handleSmokeList)
	s.addTool("smoke_discover", smokeDiscoverTool(), handleSmokeDiscover)
	s.addTool("smoke_explain", smokeExplainTool(), handleSmokeExplain)
	s.addTool("smoke_generate_test", smokeGenerateTestTool(), handleSmokeGenerateTest)
}

// addTool registers both the mcp-go tool definition and our internal handler.
func (s *Server) addTool(name string, tool mcplib.Tool, handler ToolHandler) {
	s.handlers[name] = handler
	s.mcp.AddTool(tool, s.adaptHandler(handler))
}

// adaptHandler wraps our ToolHandler to match mcp-go's ToolHandlerFunc signature.
func (s *Server) adaptHandler(h ToolHandler) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
		args := make(map[string]interface{})
		if req.Params.Arguments != nil {
			if m, ok := req.Params.Arguments.(map[string]interface{}); ok {
				args = m
			}
		}

		result, err := h(ctx, args)
		if err != nil {
			return mcplib.NewToolResultErrorFromErr("smoke tool error", err), nil
		}

		// Return structured JSON result
		jsonBytes, err := json.Marshal(result)
		if err != nil {
			return mcplib.NewToolResultErrorFromErr("marshalling result", err), nil
		}

		return mcplib.NewToolResultStructured(result, string(jsonBytes)), nil
	}
}

// --- Tool Definitions ---

func smokeRunTool() mcplib.Tool {
	return mcplib.NewTool("smoke_run",
		mcplib.WithDescription("Run smoke tests from a .smokesig.yaml config file. Returns pass/fail results with assertion details for each test. Use this to verify services are healthy, check endpoints, validate configs, or debug failures."),
		mcplib.WithString("config_path",
			mcplib.Description("Path to .smokesig.yaml (default: .smokesig.yaml in working directory)"),
		),
		mcplib.WithArray("tags",
			mcplib.Description("Include only tests with these tags"),
			mcplib.Items(map[string]any{"type": "string"}),
		),
		mcplib.WithArray("exclude_tags",
			mcplib.Description("Exclude tests with these tags"),
			mcplib.Items(map[string]any{"type": "string"}),
		),
		mcplib.WithBoolean("fail_fast",
			mcplib.Description("Stop on first failure (default: false)"),
		),
		mcplib.WithString("timeout",
			mcplib.Description("Per-test timeout override, e.g. '30s'"),
		),
		mcplib.WithBoolean("dry_run",
			mcplib.Description("List tests without running them (default: false)"),
		),
		mcplib.WithBoolean("monorepo",
			mcplib.Description("Discover and run .smokesig.yaml in subdirectories (default: false)"),
		),
	)
}

func smokeInitTool() mcplib.Tool {
	return mcplib.NewTool("smoke_init",
		mcplib.WithDescription("Generate a .smokesig.yaml smoke test config for a project. Auto-detects Go, Node, Python, Docker, and Rust projects. Can also inspect a running Docker container. Returns the generated config without writing to disk unless write=true."),
		mcplib.WithString("directory",
			mcplib.Description("Project directory to scan (default: working directory)"),
		),
		mcplib.WithString("from_container",
			mcplib.Description("Generate config by inspecting a running Docker container name"),
		),
		mcplib.WithBoolean("write",
			mcplib.Description("Write .smokesig.yaml to disk (default: false, returns YAML as text)"),
		),
		mcplib.WithBoolean("force",
			mcplib.Description("Overwrite existing .smokesig.yaml (default: false)"),
		),
	)
}

func smokeValidateTool() mcplib.Tool {
	return mcplib.NewTool("smoke_validate",
		mcplib.WithDescription("Validate a .smokesig.yaml config file without running tests. Checks for required fields, assertion consistency, regex validity, and structural correctness. Returns all errors at once."),
		mcplib.WithString("config_path",
			mcplib.Description("Path to .smokesig.yaml (default: .smokesig.yaml)"),
		),
	)
}

func smokeListTool() mcplib.Tool {
	return mcplib.NewTool("smoke_list",
		mcplib.WithDescription("List all smoke tests defined in a .smokesig.yaml config. Shows test names, tags, command, and assertion types. Useful for understanding what's configured before running."),
		mcplib.WithString("config_path",
			mcplib.Description("Path to .smokesig.yaml (default: .smokesig.yaml)"),
		),
		mcplib.WithArray("tags",
			mcplib.Description("Filter to tests with these tags"),
			mcplib.Items(map[string]any{"type": "string"}),
		),
		mcplib.WithBoolean("monorepo",
			mcplib.Description("Discover configs in subdirectories (default: false)"),
		),
	)
}

func smokeDiscoverTool() mcplib.Tool {
	return mcplib.NewTool("smoke_discover",
		mcplib.WithDescription("Find all .smokesig.yaml config files in a directory tree. Returns paths and project names. Useful for understanding the test landscape of a workspace."),
		mcplib.WithString("directory",
			mcplib.Description("Root directory to search (default: working directory)"),
		),
		mcplib.WithNumber("depth",
			mcplib.Description("Maximum search depth (default: unlimited)"),
		),
	)
}

func smokeExplainTool() mcplib.Tool {
	return mcplib.NewTool("smoke_explain",
		mcplib.WithDescription("Explain a smoke test assertion type and its configuration. Returns the assertion's fields, defaults, and an example YAML snippet. Use when you need to understand or construct assertion configurations."),
		mcplib.WithString("assertion_type",
			mcplib.Description("Assertion type to explain"),
			mcplib.Required(),
		),
	)
}

func smokeGenerateTestTool() mcplib.Tool {
	return mcplib.NewTool("smoke_generate_test",
		mcplib.WithDescription("Generate a single smoke test YAML snippet. Provide what you want to test and get back valid YAML to add to .smokesig.yaml. Supports all 29 assertion types."),
		mcplib.WithString("name",
			mcplib.Description("Test name"),
			mcplib.Required(),
		),
		mcplib.WithString("assertion_type",
			mcplib.Description("Primary assertion type (e.g. 'http', 'port_listening', 'redis_ping')"),
			mcplib.Required(),
		),
		mcplib.WithString("description",
			mcplib.Description("What this test should verify (natural language)"),
		),
		mcplib.WithObject("params",
			mcplib.Description("Assertion parameters as key-value pairs"),
		),
		mcplib.WithArray("tags",
			mcplib.Description("Tags for this test"),
			mcplib.Items(map[string]any{"type": "string"}),
		),
	)
}
