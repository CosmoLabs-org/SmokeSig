//go:build tui

package tui

import (
	"github.com/CosmoLabs-org/SmokeSig/internal/reporter"
)

// Events sent from TUIReporter to the Bubbletea program.
type testStartEvent struct{ Name string }
type testResultEvent struct{ Data reporter.TestResultData }
type prereqStartEvent struct{ Name string }
type prereqResultEvent struct{ Data reporter.PrereqResultData }
type summaryEvent struct{ Data reporter.SuiteResultData }

// Events for re-run lifecycle.
type rerunStartEvent struct{}

// RerunErrorEvent is sent when a re-run returns an error.
type RerunErrorEvent struct{ Err error }

// WatchTriggerEvent is sent by the fsnotify watcher on file changes.
type WatchTriggerEvent struct{}
