package teax

import (
	"context"
	"fmt"
	tea "github.com/charmbracelet/bubbletea"
	"log"
)

// Run prepares the terminal and runs the TUI model.
func Run(ctx context.Context, model tea.Model) error {
	p := tea.NewProgram(model, tea.WithAltScreen(), tea.WithContext(ctx))

	log.Printf("[DEBUG] starting bubbletea program")
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("run bubbletea program: %w", err)
	}

	return nil
}

type tickMsg struct{}
