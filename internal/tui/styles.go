package tui

import "github.com/charmbracelet/lipgloss"

// The palette adapts to light and dark terminals. There is one accent
// colour; everything else is text, dim text, or a status colour.
var (
	colAccent = lipgloss.AdaptiveColor{Light: "#0F766E", Dark: "#5EEAD4"}
	colDim    = lipgloss.AdaptiveColor{Light: "#6B7280", Dark: "#7F8798"}
	colText   = lipgloss.AdaptiveColor{Light: "#111827", Dark: "#E5E7EB"}
	colBad    = lipgloss.AdaptiveColor{Light: "#B91C1C", Dark: "#F87171"}
	colWarn   = lipgloss.AdaptiveColor{Light: "#B45309", Dark: "#FBBF24"}
	colGood   = lipgloss.AdaptiveColor{Light: "#15803D", Dark: "#4ADE80"}
	colSelBG  = lipgloss.AdaptiveColor{Light: "#CCFBF1", Dark: "#134E4A"}
)

type theme struct {
	text, dim, accent, bold       lipgloss.Style
	bad, warn, good               lipgloss.Style
	border, borderFocus           lipgloss.Style
	borderTitle, borderTitleFocus lipgloss.Style
	header, headerRight           lipgloss.Style
	selected, selectedDim, marked lipgloss.Style
	key, keyDesc                  lipgloss.Style
}

var styles = newTheme()

func newTheme() theme {
	fg := func(c lipgloss.TerminalColor) lipgloss.Style { return lipgloss.NewStyle().Foreground(c) }
	return theme{
		text:   fg(colText),
		dim:    fg(colDim),
		accent: fg(colAccent),
		bold:   lipgloss.NewStyle().Bold(true),
		bad:    fg(colBad),
		warn:   fg(colWarn),
		good:   fg(colGood),

		border:           fg(colDim),
		borderFocus:      fg(colAccent),
		borderTitle:      fg(colDim),
		borderTitleFocus: fg(colAccent).Bold(true),

		header:      fg(colAccent).Bold(true),
		headerRight: fg(colDim),

		selected:    lipgloss.NewStyle().Background(colSelBG).Foreground(colText).Bold(true),
		selectedDim: lipgloss.NewStyle().Background(colSelBG).Foreground(colDim),
		marked:      fg(colAccent).Bold(true),

		key:     fg(colAccent),
		keyDesc: fg(colDim),
	}
}
