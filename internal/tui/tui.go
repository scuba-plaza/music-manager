// Package tui is the full-screen interface for browsing and editing the
// library.
package tui

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"
)

// Run shows the interface on lib until the user quits or ctx ends. It takes
// over the whole terminal and puts it back as it was afterwards.
func Run(ctx context.Context, lib Library) error {
	p := tea.NewProgram(New(lib),
		tea.WithContext(ctx),
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)
	_, err := p.Run()
	if ctx.Err() != nil { // Ctrl+C from outside the program is not a failure
		return nil
	}
	return err
}
