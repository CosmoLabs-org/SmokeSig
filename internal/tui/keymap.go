//go:build tui

package tui

import "github.com/charmbracelet/bubbles/key"

type keyMap struct {
	Up         key.Binding
	Down       key.Binding
	Top        key.Binding
	Bottom     key.Binding
	Toggle     key.Binding
	ToggleAll  key.Binding
	Filter     key.Binding
	Rerun      key.Binding
	RerunFails key.Binding
	RerunAll   key.Binding
	Help       key.Binding
	Quit       key.Binding
}

func defaultKeyMap() keyMap {
	return keyMap{
		Up:         key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("k/Up", "move up")),
		Down:       key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("j/Down", "move down")),
		Top:        key.NewBinding(key.WithKeys("home", "g"), key.WithHelp("g/Home", "first test")),
		Bottom:     key.NewBinding(key.WithKeys("end", "G"), key.WithHelp("G/End", "last test")),
		Toggle:     key.NewBinding(key.WithKeys("enter", " "), key.WithHelp("Enter", "expand/collapse")),
		ToggleAll:  key.NewBinding(key.WithKeys("tab"), key.WithHelp("Tab", "expand/collapse all")),
		Filter:     key.NewBinding(key.WithKeys("f"), key.WithHelp("f", "cycle filter")),
		Rerun:      key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "rerun current")),
		RerunFails: key.NewBinding(key.WithKeys("R"), key.WithHelp("R", "rerun failures")),
		RerunAll:   key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "rerun all")),
		Help:       key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
		Quit:       key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
	}
}
