package tui

import "github.com/charmbracelet/bubbles/key"

// keyMap holds every key binding. The footer and the help screen are built
// from the descriptions here, so a key is documented where it is defined.
type keyMap struct {
	Quit key.Binding
	Help key.Binding
}

func newKeyMap() keyMap {
	return keyMap{
		Quit: key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
		Help: key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
	}
}
