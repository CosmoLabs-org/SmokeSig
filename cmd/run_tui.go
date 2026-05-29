//go:build tui

package cmd

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/CosmoLabs-org/SmokeSig/internal/reporter"
	"github.com/CosmoLabs-org/SmokeSig/internal/runner"
	"github.com/CosmoLabs-org/SmokeSig/internal/tui"
)

var useTUI bool

func init() {
	runCmd.Flags().BoolVar(&useTUI, "tui", false, "interactive TUI mode")
}

func runWithTUI(rn *runner.Runner, opts runner.RunOptions) error {
	m := tui.NewModel()
	p := tea.NewProgram(&m, tea.WithAltScreen())

	rep := tui.NewTUIReporter(p.Send)
	rn.Reporter = rep
	m.SetRerunFunc(func(name string) reporter.TestResultData {
		result, err := rn.RunSingle(name, opts)
		if err != nil {
			return reporter.TestResultData{Name: name, Passed: false, Error: err}
		}
		return tuiConvertResult(*result)
	})

	go func() {
		_, err := rn.Run(opts)
		p.Send(tui.RunnerDoneMsg{Err: err})
	}()

	_, err := p.Run()
	return err
}

func tuiConvertResult(tr runner.TestResult) reporter.TestResultData {
	var assertions []reporter.AssertionDetail
	for _, a := range tr.Assertions {
		assertions = append(assertions, reporter.AssertionDetail{
			Type:     a.Type,
			Expected: a.Expected,
			Actual:   a.Actual,
			Passed:   a.Passed,
		})
	}
	return reporter.TestResultData{
		Name:           tr.Name,
		Passed:         tr.Passed,
		Skipped:        tr.Skipped,
		AllowedFailure: tr.AllowedFailure,
		Duration:       tr.Duration,
		Assertions:     assertions,
		Error:          tr.Error,
	}
}
