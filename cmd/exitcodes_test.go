package cmd

import (
	"errors"
	"fmt"
	"testing"

	"github.com/CosmoLabs-org/SmokeSig/internal/runner"
)

func TestExitCodeForError_Nil(t *testing.T) {
	if code := ExitCodeForError(nil); code != ExitPass {
		t.Errorf("ExitCodeForError(nil) = %d, want %d", code, ExitPass)
	}
}

func TestExitCodeForError_ConfigError(t *testing.T) {
	err := &ConfigError{Err: fmt.Errorf("bad yaml")}
	if code := ExitCodeForError(err); code != ExitConfigError {
		t.Errorf("ExitCodeForError(ConfigError) = %d, want %d", code, ExitConfigError)
	}
}

func TestExitCodeForError_WrappedConfigError(t *testing.T) {
	inner := &ConfigError{Err: fmt.Errorf("parse error")}
	wrapped := fmt.Errorf("loading config: %w", inner)
	if code := ExitCodeForError(wrapped); code != ExitConfigError {
		t.Errorf("ExitCodeForError(wrapped ConfigError) = %d, want %d", code, ExitConfigError)
	}
}

func TestExitCodeForError_PrereqError(t *testing.T) {
	err := &PrereqError{Err: fmt.Errorf("go not found")}
	if code := ExitCodeForError(err); code != ExitPrereqFailure {
		t.Errorf("ExitCodeForError(PrereqError) = %d, want %d", code, ExitPrereqFailure)
	}
}

func TestExitCodeForError_WrappedPrereqError(t *testing.T) {
	inner := &PrereqError{Err: fmt.Errorf("docker missing")}
	wrapped := fmt.Errorf("prereq check: %w", inner)
	if code := ExitCodeForError(wrapped); code != ExitPrereqFailure {
		t.Errorf("ExitCodeForError(wrapped PrereqError) = %d, want %d", code, ExitPrereqFailure)
	}
}

func TestExitCodeForError_GenericError(t *testing.T) {
	err := fmt.Errorf("test failed")
	if code := ExitCodeForError(err); code != ExitFail {
		t.Errorf("ExitCodeForError(generic) = %d, want %d", code, ExitFail)
	}
}

func TestExitCodeConstants(t *testing.T) {
	if ExitPass != 0 {
		t.Errorf("ExitPass = %d, want 0", ExitPass)
	}
	if ExitFail != 1 {
		t.Errorf("ExitFail = %d, want 1", ExitFail)
	}
	if ExitConfigError != 2 {
		t.Errorf("ExitConfigError = %d, want 2", ExitConfigError)
	}
	if ExitPrereqFailure != 3 {
		t.Errorf("ExitPrereqFailure = %d, want 3", ExitPrereqFailure)
	}
}

func TestConfigErrorUnwrap(t *testing.T) {
	inner := fmt.Errorf("underlying")
	err := &ConfigError{Err: inner}
	if !errors.Is(err, inner) {
		t.Error("ConfigError should unwrap to inner error")
	}
}

func TestPrereqErrorUnwrap(t *testing.T) {
	inner := fmt.Errorf("underlying")
	err := &PrereqError{Err: inner}
	if !errors.Is(err, inner) {
		t.Error("PrereqError should unwrap to inner error")
	}
}

func TestWrapRunnerError_PrereqFailed(t *testing.T) {
	err := &runner.ErrPrereqFailed{Name: "docker", Err: fmt.Errorf("not found")}
	wrapped := wrapRunnerError(err)
	var prereqErr *PrereqError
	if !errors.As(wrapped, &prereqErr) {
		t.Errorf("wrapRunnerError should wrap ErrPrereqFailed as PrereqError, got %T", wrapped)
	}
}

func TestWrapRunnerError_GenericError(t *testing.T) {
	err := fmt.Errorf("some runner error")
	wrapped := wrapRunnerError(err)
	var prereqErr *PrereqError
	if errors.As(wrapped, &prereqErr) {
		t.Error("wrapRunnerError should not wrap generic errors as PrereqError")
	}
	if wrapped != err {
		t.Error("wrapRunnerError should return generic errors unchanged")
	}
}

func TestWrapRunnerError_Nil(t *testing.T) {
	if err := wrapRunnerError(nil); err != nil {
		t.Errorf("wrapRunnerError(nil) = %v, want nil", err)
	}
}
