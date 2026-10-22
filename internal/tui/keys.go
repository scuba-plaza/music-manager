package tui

import "github.com/charmbracelet/bubbles/key"

// keyMap holds every key binding. The footer and the help screen are built
// from the descriptions here, so a key is documented where it is defined.
type keyMap struct {
	Quit key.Binding
	Help key.Binding

	Up, Down         key.Binding
	PageUp, PageDown key.Binding
	Top, Bottom      key.Binding
	Open, Close      key.Binding
	Toggle           key.Binding
	Filter           key.Binding

	// while typing in the filter
	FilterKeep, FilterClear key.Binding
	// with a filter kept
	ClearFilter key.Binding
}

func newKeyMap() keyMap {
	return keyMap{
		Quit: key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
		Help: key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),

		Up:       key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑↓", "move")),
		Down:     key.NewBinding(key.WithKeys("down", "j")),
		PageUp:   key.NewBinding(key.WithKeys("pgup", "ctrl+u")),
		PageDown: key.NewBinding(key.WithKeys("pgdown", "ctrl+d")),
		Top:      key.NewBinding(key.WithKeys("g", "home")),
		Bottom:   key.NewBinding(key.WithKeys("G", "end")),
		Open:     key.NewBinding(key.WithKeys("right", "l"), key.WithHelp("→", "open")),
		Close:    key.NewBinding(key.WithKeys("left", "h"), key.WithHelp("←", "close")),
		Toggle:   key.NewBinding(key.WithKeys("enter")),
		Filter:   key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "filter")),

		FilterKeep:  key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "keep")),
		FilterClear: key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "clear")),
		ClearFilter: key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "clear filter")),
	}
}
