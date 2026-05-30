//go:build !tui

package cmd

import "fmt"

// interactive is always false when built without the tui tag.
var interactive bool

func runInteractive(_ *runContext) error {
	return fmt.Errorf("interactive mode requires building with -tags tui\n  go build -tags tui -o smokesig .")
}
