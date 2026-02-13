package ui

import (
	"os"

	"github.com/charmbracelet/lipgloss"
)

// Renderer is bound to stderr so colors work even when stdout is piped.
var Renderer = lipgloss.NewRenderer(os.Stderr)
