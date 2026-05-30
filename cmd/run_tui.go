//go:build tui

package cmd

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/CosmoLabs-org/SmokeSig/internal/reporter"
	"github.com/CosmoLabs-org/SmokeSig/internal/runner"
	"github.com/CosmoLabs-org/SmokeSig/internal/tui"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/fsnotify/fsnotify"
	"golang.org/x/term"
)

// interactive is the --interactive flag, registered only with -tags tui.
var interactive bool

func init() {
	runCmd.Flags().BoolVar(&interactive, "interactive", false,
		"Full-screen TUI for navigating results and re-running tests")
}

func runInteractive(rc *runContext) error {
	// TTY guard: fall back to terminal reporter if stdout is not a terminal
	if !term.IsTerminal(int(os.Stdout.Fd())) {
		return fmt.Errorf("--interactive requires a TTY (stdout is not a terminal)")
	}

	// TERM=dumb guard
	if os.Getenv("TERM") == "dumb" {
		return fmt.Errorf("--interactive requires a capable terminal (TERM=dumb)")
	}

	// Create the Bubbletea model (rerunFunc set after program creation)
	model := tui.NewModel(rc.cfg.Project, nil)

	// Create the Bubbletea program with alternate screen
	program := tea.NewProgram(model, tea.WithAltScreen())

	// Create TUI reporter that sends events to the program
	tuiRep := tui.NewTUIReporter(program)

	// Build secondary reporters (json, junit, etc.) excluding terminal
	var finalRep reporter.Reporter = tuiRep
	if rc.formatStr != "" && rc.formatStr != "terminal" {
		secondaryRep, closers, err := reporter.ChainWithVerbosity(
			rc.formatStr, os.Stdout, verbosity,
		)
		if err == nil && secondaryRep != nil {
			finalRep = reporter.NewMultiReporter(tuiRep, secondaryRep)
			defer func() {
				for i := len(closers) - 1; i >= 0; i-- {
					closers[i].Close()
				}
			}()
		}
	}

	// Set the rerunFunc now that we have tuiRep
	model.SetRerunFunc(func(ctx context.Context, testNames []string) error {
		opts := rc.opts
		opts.TestNames = testNames
		r := &runner.Runner{
			Config:    rc.cfg,
			Reporter:  finalRep,
			ConfigDir: rc.configDir,
		}
		_, err := r.Run(opts)
		return err
	})

	// Wire the reporter into a runner and start tests in a goroutine
	r := &runner.Runner{
		Config:    rc.cfg,
		Reporter:  finalRep,
		ConfigDir: rc.configDir,
	}

	go func() {
		if _, err := r.Run(rc.opts); err != nil {
			program.Send(tui.RerunErrorEvent{Err: err})
		}
	}()

	// Start watch mode watcher if requested
	if rc.watch {
		go func() {
			if err := runWatchInteractive(rc, program); err != nil {
				program.Send(tui.RerunErrorEvent{Err: err})
			}
		}()
	}

	// Run the Bubbletea program (blocks until quit)
	_, err := program.Run()
	return err
}

// runWatchInteractive watches the config directory for file changes
// and sends WatchTriggerEvent to the Bubbletea program.
func runWatchInteractive(rc *runContext, program *tea.Program) error {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("fsnotify: %w", err)
	}
	defer w.Close()

	if err := w.Add(rc.configDir); err != nil {
		return fmt.Errorf("watching %s: %w", rc.configDir, err)
	}

	debounce := 500 * time.Millisecond
	var timer *time.Timer

	for {
		select {
		case ev, ok := <-w.Events:
			if !ok {
				return nil
			}
			if !isRelevantEvent(ev.Op) {
				continue
			}
			if timer != nil {
				timer.Stop()
			}
			timer = time.AfterFunc(debounce, func() {
				program.Send(tui.WatchTriggerEvent{})
			})
		case err, ok := <-w.Errors:
			if !ok {
				return nil
			}
			program.Send(tui.RerunErrorEvent{Err: fmt.Errorf("watch: %w", err)})
		}
	}
}
