package cmd

import "errors"

// Exit codes for SmokeSig CLI. These form a stable contract for consumers
// (CCS, CI systems) to distinguish failure modes without parsing stderr.
const (
	ExitPass          = 0
	ExitFail          = 1
	ExitConfigError   = 2
	ExitPrereqFailure = 3
)

// ConfigError wraps errors from config loading, parsing, or validation.
type ConfigError struct{ Err error }

func (e *ConfigError) Error() string { return e.Err.Error() }
func (e *ConfigError) Unwrap() error { return e.Err }

// PrereqError wraps errors from prerequisite check failures.
type PrereqError struct{ Err error }

func (e *PrereqError) Error() string { return e.Err.Error() }
func (e *PrereqError) Unwrap() error { return e.Err }

// ExitCodeForError returns the appropriate exit code for the given error.
func ExitCodeForError(err error) int {
	if err == nil {
		return ExitPass
	}
	var cfgErr *ConfigError
	var preErr *PrereqError
	switch {
	case errors.As(err, &cfgErr):
		return ExitConfigError
	case errors.As(err, &preErr):
		return ExitPrereqFailure
	default:
		return ExitFail
	}
}
