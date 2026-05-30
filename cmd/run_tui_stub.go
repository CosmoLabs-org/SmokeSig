//go:build !tui

package cmd

import "github.com/CosmoLabs-org/SmokeSig/internal/runner"

var useTUI bool

func runWithTUI(_ *runner.Runner, _ runner.RunOptions) error {
	panic("tui not available: rebuild with -tags tui")
}
