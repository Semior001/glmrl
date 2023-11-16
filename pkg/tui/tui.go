package tui

import (
	"fmt"
	"github.com/charmbracelet/lipgloss"
	"github.com/pkg/browser"
)

// set up browser logging
func init() {
	browser.Stdout = loggingWriter("browser-stdout")
	browser.Stderr = loggingWriter("browser-stderr")
}

// Version makes a prettified view of the version.
func Version(version string) string {
	return lipgloss.NewStyle().
		MarginLeft(1).
		Bold(true).
		Foreground(lipgloss.Color("#FFA500")).
		Render(fmt.Sprintf("GLMRL version: %s", version))
}
