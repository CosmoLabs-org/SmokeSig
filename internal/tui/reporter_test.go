//go:build tui

package tui

import "github.com/CosmoLabs-org/SmokeSig/internal/reporter"

// Compile-time interface check.
var _ reporter.Reporter = (*TUIReporter)(nil)
