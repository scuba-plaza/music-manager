// Package tui is the full-screen interface for browsing and editing the
// library.
package tui

import (
	"context"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"golang.org/x/term"
)

// Run shows the interface on lib until the user quits or ctx ends. It takes
// over the whole terminal and puts it back as it was afterwards.
func Run(ctx context.Context, lib Library, opts ...Option) error {
	program := []tea.ProgramOption{
		tea.WithContext(ctx),
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	}
	if wantsKittyGraphics(os.Getenv) && term.IsTerminal(int(os.Stdout.Fd())) {
		// Covers are drawn as pictures. They are written to the terminal by
		// the interface, so everything goes through one lock.
		tty := &lockedTTY{File: os.Stdout}
		program = append(program, tea.WithOutput(tty))
		opts = append(opts, WithGraphics(tty, func() (int, int) { return cellPixels(os.Stdout) }))
	}
	p := tea.NewProgram(New(lib, opts...), program...)
	_, err := p.Run()
	if ctx.Err() != nil { // Ctrl+C from outside the program is not a failure
		return nil
	}
	return err
}
