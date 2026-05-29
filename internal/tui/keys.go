//go:build tui

package tui

import "github.com/charmbracelet/bubbles/key"

type keyMap struct {
	Up     key.Binding
	Down   key.Binding
	Expand key.Binding
	Rerun  key.Binding
	Filter key.Binding
	Search key.Binding
	Escape key.Binding
	Quit   key.Binding
}

var keys = keyMap{
	Up:     key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "up")),
	Down:   key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "down")),
	Expand: key.NewBinding(key.WithKeys("enter", " "), key.WithHelp("⏎", "expand")),
	Rerun:  key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "re-run")),
	Filter: key.NewBinding(key.WithKeys("tab"), key.WithHelp("⇥", "filter")),
	Search: key.NewBinding(key.WithKeys("/")),
	Escape: key.NewBinding(key.WithKeys("esc")),
	Quit:   key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
}
