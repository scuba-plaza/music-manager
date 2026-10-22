package tui

import "github.com/charmbracelet/bubbles/key"

// keyMap holds every key binding. The footer and the help screen are built
// from the descriptions here, so a key is documented where it is defined.
type keyMap struct {
	Quit key.Binding
	Help key.Binding
	Undo key.Binding
	Redo key.Binding

	// moving about
	Up, Down         key.Binding
	PageUp, PageDown key.Binding
	Top, Bottom      key.Binding
	Open, Close      key.Binding
	Toggle           key.Binding
	Switch           key.Binding

	// filtering
	Filter                  key.Binding
	FilterKeep, FilterClear key.Binding
	ClearFilter             key.Binding

	// in the inspector
	Field key.Binding
	Edit  key.Binding
	Back  key.Binding

	// while editing a field
	Save, Cancel, NextInput key.Binding
}

func newKeyMap() keyMap {
	return keyMap{
		Quit: key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
		Help: key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
		Undo: key.NewBinding(key.WithKeys("u"), key.WithHelp("u", "undo")),
		Redo: key.NewBinding(key.WithKeys("ctrl+r"), key.WithHelp("ctrl+r", "redo")),

		Up:       key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑↓", "move")),
		Down:     key.NewBinding(key.WithKeys("down", "j")),
		PageUp:   key.NewBinding(key.WithKeys("pgup", "ctrl+u")),
		PageDown: key.NewBinding(key.WithKeys("pgdown", "ctrl+d")),
		Top:      key.NewBinding(key.WithKeys("g", "home")),
		Bottom:   key.NewBinding(key.WithKeys("G", "end")),
		Open:     key.NewBinding(key.WithKeys("right", "l"), key.WithHelp("→", "open")),
		Close:    key.NewBinding(key.WithKeys("left", "h"), key.WithHelp("←", "close")),
		Toggle:   key.NewBinding(key.WithKeys("enter")),
		Switch:   key.NewBinding(key.WithKeys("tab", "shift+tab")),

		Filter:      key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "filter")),
		FilterKeep:  key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "keep")),
		FilterClear: key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "clear")),
		ClearFilter: key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "clear filter")),

		Field: key.NewBinding(key.WithKeys("up", "k", "down", "j"), key.WithHelp("↑↓", "field")),
		Edit:  key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "edit")),
		Back:  key.NewBinding(key.WithKeys("esc", "left", "h"), key.WithHelp("←", "back")),

		Save:      key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "save")),
		Cancel:    key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "cancel")),
		NextInput: key.NewBinding(key.WithKeys("tab", "shift+tab"), key.WithHelp("tab", "other form")),
	}
}
