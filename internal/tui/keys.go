package tui

import (
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

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

	// on a cut
	Less, More, LessBig, MoreBig, ClearCut key.Binding
	Play                                   key.Binding

	// on the cover
	RemoveCover key.Binding

	Remove key.Binding

	// looking up
	Identify, IdentifyAll, Adopt key.Binding

	Add  key.Binding
	Sync key.Binding

	// marking, and acting on several tracks
	Mark, MarkAll, ClearMarks key.Binding
	EditMany                  key.Binding
	MoveDown, MoveUp          key.Binding
	Renumber                  key.Binding

	// in a dialog
	Apply, Close2, NextField, PrevField key.Binding
}

// isNudge reports whether msg is one of the keys that nudge a cut.
func (k keyMap) isNudge(msg tea.KeyMsg) bool {
	return key.Matches(msg, k.Less, k.More, k.LessBig, k.MoreBig)
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

		Less:     key.NewBinding(key.WithKeys("["), key.WithHelp("[ ]", "nudge")),
		More:     key.NewBinding(key.WithKeys("]")),
		LessBig:  key.NewBinding(key.WithKeys("{")),
		MoreBig:  key.NewBinding(key.WithKeys("}")),
		ClearCut: key.NewBinding(key.WithKeys("0"), key.WithHelp("0", "no cut")),
		Play:     key.NewBinding(key.WithKeys("p"), key.WithHelp("p", "hear")),

		RemoveCover: key.NewBinding(key.WithKeys("x"), key.WithHelp("x", "remove")),

		Add:  key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "add")),
		Sync: key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "sync")),

		Identify:    key.NewBinding(key.WithKeys("i"), key.WithHelp("i", "identify")),
		IdentifyAll: key.NewBinding(key.WithKeys("I"), key.WithHelp("I", "identify all")),
		Adopt:       key.NewBinding(key.WithKeys("o"), key.WithHelp("o", "use old tags")),

		Remove: key.NewBinding(key.WithKeys("d", "delete"), key.WithHelp("d", "remove")),

		Mark:       key.NewBinding(key.WithKeys(" "), key.WithHelp("space", "mark")),
		MarkAll:    key.NewBinding(key.WithKeys("ctrl+a"), key.WithHelp("ctrl+a", "mark all")),
		ClearMarks: key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "clear marks")),
		EditMany:   key.NewBinding(key.WithKeys("e"), key.WithHelp("e", "edit")),
		MoveDown:   key.NewBinding(key.WithKeys("J"), key.WithHelp("J K", "move")),
		MoveUp:     key.NewBinding(key.WithKeys("K")),
		Renumber:   key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "number")),

		Apply:     key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "apply")),
		Close2:    key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "cancel")),
		NextField: key.NewBinding(key.WithKeys("tab", "down"), key.WithHelp("tab", "next")),
		PrevField: key.NewBinding(key.WithKeys("shift+tab", "up")),
	}
}
