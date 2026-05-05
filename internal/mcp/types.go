package mcp

import (
	"context"
	"time"
)

// ToolHandler is a function that handles an MCP tool call.
type ToolHandler func(ctx context.Context, args map[string]interface{}) (interface{}, error)

// RunResult is the structured result from smoke_run.
type RunResult struct {
	Project   string        `json:"project"`
	Total     int           `json:"total"`
	Passed    int           `json:"passed"`
	Failed    int           `json:"failed"`
	Skipped   int           `json:"skipped"`
	Duration  time.Duration `json:"duration_ms"`
	Tests     []TestResult  `json:"tests"`
	ConfigPath string       `json:"config_path"`
}

// TestResult is a single test's result in MCP format.
type TestResult struct {
	Name           string             `json:"name"`
	Passed         bool               `json:"passed"`
	Skipped        bool               `json:"skipped"`
	AllowedFailure bool               `json:"allowed_failure,omitempty"`
	DurationMs     int64              `json:"duration_ms"`
	Assertions     []AssertionResult  `json:"assertions,omitempty"`
	FixSuggestions []string           `json:"fix_suggestions,omitempty"`
	Error          string             `json:"error,omitempty"`
}

// AssertionResult is a single assertion's result in MCP format.
type AssertionResult struct {
	Type     string `json:"type"`
	Expected string `json:"expected"`
	Actual   string `json:"actual"`
	Passed   bool   `json:"passed"`
}

// ValidateResult is the result from smoke_validate.
type ValidateResult struct {
	Valid  bool     `json:"valid"`
	Tests  []string `json:"tests,omitempty"`
	Errors []string `json:"errors,omitempty"`
}

// ListResult is the result from smoke_list.
type ListResult struct {
	ConfigPath string       `json:"config_path"`
	Tests      []ListedTest `json:"tests"`
}

// ListedTest is a single test entry in smoke_list output.
type ListedTest struct {
	Name           string   `json:"name"`
	Tags           []string `json:"tags,omitempty"`
	RunCommand     string   `json:"run_command,omitempty"`
	AssertionTypes []string `json:"assertion_types,omitempty"`
	SkipIf         string   `json:"skip_if,omitempty"`
}

// DiscoverResult is the result from smoke_discover.
type DiscoverResult struct {
	Configs []DiscoveredConfig `json:"configs"`
}

// DiscoveredConfig is a single discovered .smokesig.yaml.
type DiscoveredConfig struct {
	Path        string `json:"path"`
	Directory   string `json:"directory"`
	ProjectName string `json:"project_name"`
}

// ExplainResult is the result from smoke_explain.
type ExplainResult struct {
	Type        string         `json:"type"`
	Description string         `json:"description"`
	Fields      []ExplainField `json:"fields"`
	Example     string         `json:"example_yaml"`
	Notes       string         `json:"notes,omitempty"`
}

// ExplainField describes one field of an assertion type.
type ExplainField struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Required    bool   `json:"required"`
	Default     string `json:"default,omitempty"`
	Description string `json:"description"`
}

// InitResult is the result from smoke_init.
type InitResult struct {
	YAML      string `json:"yaml"`
	Written   bool   `json:"written"`
	WritePath string `json:"write_path,omitempty"`
}

// GenerateTestResult is the result from smoke_generate_test.
type GenerateTestResult struct {
	YAML string `json:"yaml"`
}
