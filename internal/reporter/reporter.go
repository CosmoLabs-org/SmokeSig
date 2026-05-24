package reporter

import "time"

// Verbosity controls how much detail reporters emit.
type Verbosity int

const (
	// VerbosityQuiet suppresses per-test progress; only failures and final summary.
	VerbosityQuiet Verbosity = -1
	// VerbosityNormal is the default behavior.
	VerbosityNormal Verbosity = 0
	// VerbosityVerbose shows assertion-level detail, timing breakdown, and full error context.
	VerbosityVerbose Verbosity = 1
)

// Reporter receives events during test execution.
type Reporter interface {
	PrereqStart(name string)
	PrereqResult(r PrereqResultData)
	TestStart(name string)
	TestResult(r TestResultData)
	Summary(s SuiteResultData)
}

// PrereqResultData holds the outcome of a prerequisite check.
type PrereqResultData struct {
	Name   string
	Passed bool
	Output string
	Hint   string
	Error  error
}

// AssertionDetail holds one assertion's outcome.
type AssertionDetail struct {
	Type     string `json:"type"`
	Expected string `json:"expected"`
	Actual   string `json:"actual"`
	Passed   bool   `json:"passed"`
}

// TestResultData holds the outcome of a single test.
type TestResultData struct {
	Name           string            `json:"name"`
	Passed         bool              `json:"passed"`
	Skipped        bool              `json:"skipped"`
	AllowedFailure bool              `json:"allowed_failure,omitempty"`
	Duration       time.Duration     `json:"duration"`
	Assertions     []AssertionDetail `json:"assertions"`
	Error          error             `json:"-"`
}

// SuiteResultData holds the aggregate results.
type SuiteResultData struct {
	Project         string           `json:"project"`
	Total           int              `json:"total"`
	Passed          int              `json:"passed"`
	Failed          int              `json:"failed"`
	Skipped         int              `json:"skipped"`
	AllowedFailures int              `json:"allowed_failures"`
	Duration        time.Duration    `json:"duration"`
	Tests           []TestResultData `json:"tests"`
	TraceHealthPct  float64          `json:"trace_health_pct,omitempty"`
	TraceDegraded   bool             `json:"trace_degraded,omitempty"`
}
