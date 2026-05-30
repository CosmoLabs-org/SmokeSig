//go:build tui

package tui

import "github.com/charmbracelet/lipgloss"

// Styles mirror the ANSI 16-color palette from terminal.go.
var (
	stylePass    = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))  // green
	styleFail    = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))  // red
	styleSkip    = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))  // yellow
	styleAllowed = lipgloss.NewStyle().Foreground(lipgloss.Color("6"))  // cyan
	styleDim     = lipgloss.NewStyle().Faint(true)
	styleBold    = lipgloss.NewStyle().Bold(true)
	styleCursor  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("4")) // blue
	styleHeader  = lipgloss.NewStyle().Bold(true).Padding(0, 1)
	styleSummary = lipgloss.NewStyle().Padding(0, 1)
	styleHelp    = lipgloss.NewStyle().Faint(true).Padding(0, 1)
	styleError   = lipgloss.NewStyle().Foreground(lipgloss.Color("1")).Bold(true)
)
