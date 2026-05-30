//go:build tui

package tui

import "github.com/charmbracelet/lipgloss"

var (
	passStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	failStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	skipStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	dimStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	headerStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("6"))
	cursorStyle = lipgloss.NewStyle().Reverse(true)
	footerStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	searchStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("5"))
)
