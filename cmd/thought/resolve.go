package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/thinkingscript/cli/internal/config"
	"github.com/thinkingscript/cli/internal/ui"
	"golang.org/x/term"
)

// ResolveTarget represents what type of target was resolved.
type ResolveTarget int

const (
	TargetFile ResolveTarget = iota
	TargetInstalled
	TargetURL
)

// ResolveResult holds the resolved path and its type.
type ResolveResult struct {
	Path   string
	Target ResolveTarget
	Name   string // For installed thoughts, the thought name
}

// ErrAmbiguous is returned when both a file and installed thought exist
// and the user cannot be prompted (non-interactive).
var ErrAmbiguous = errors.New("ambiguous target")

// ResolveThought resolves a reference to either a file, URL, or installed thought.
// If both file and installed thought exist and we're in a TTY, prompts user to choose.
// If both exist and not TTY, returns ErrAmbiguous.
//
// Resolution rules:
//   - Starts with "http://" or "https://" → URL (passed through directly)
//   - Contains "/" or starts with "." → explicit file path (./foo, ../foo, /path/to/foo)
//   - Otherwise → check both filesystem and installed thoughts
func ResolveThought(arg, cmdName string) (*ResolveResult, error) {
	// URL - pass through directly
	if strings.HasPrefix(arg, "http://") || strings.HasPrefix(arg, "https://") {
		return &ResolveResult{
			Path:   arg,
			Target: TargetURL,
		}, nil
	}

	// Explicit path (contains / or starts with .) - treat as file
	isExplicitFile := strings.Contains(arg, "/") || strings.HasPrefix(arg, ".")

	if isExplicitFile {
		if _, err := os.Stat(arg); err != nil {
			return nil, fmt.Errorf("file not found: %s", arg)
		}
		return &ResolveResult{
			Path:   arg,
			Target: TargetFile,
		}, nil
	}

	// Check for both file and installed thought
	fileExists := false
	if _, err := os.Stat(arg); err == nil {
		fileExists = true
	}

	binPath := filepath.Join(config.BinDir(), arg)
	binExists := false
	if info, err := os.Stat(binPath); err == nil && !info.IsDir() {
		binExists = true
	}

	// Neither exists
	if !fileExists && !binExists {
		return nil, fmt.Errorf("'%s' not found (no file or installed thought)", arg)
	}

	// Only file exists
	if fileExists && !binExists {
		return &ResolveResult{
			Path:   arg,
			Target: TargetFile,
		}, nil
	}

	// Only installed thought exists
	if binExists && !fileExists {
		return &ResolveResult{
			Path:   binPath,
			Target: TargetInstalled,
			Name:   arg,
		}, nil
	}

	// Both exist - ambiguous
	return resolveAmbiguous(arg, binPath, cmdName)
}

func resolveAmbiguous(arg, binPath, cmdName string) (*ResolveResult, error) {
	isTTY := term.IsTerminal(int(os.Stderr.Fd()))

	if !isTTY {
		return nil, fmt.Errorf("%w: both file './%s' and installed thought '%s' exist.\nRun one of:\n  thought %s ./%s\n  thought %s %s",
			ErrAmbiguous, arg, arg, cmdName, arg, cmdName, arg)
	}

	// TTY - prompt user
	var choice string

	fileOpt := fmt.Sprintf("File ./%s", arg)
	installedOpt := fmt.Sprintf("Installed thought '%s'", arg)

	headerStyle := ui.Renderer.NewStyle().Foreground(lipgloss.Color("214")).Bold(true)
	fmt.Fprintln(os.Stderr, headerStyle.Render(fmt.Sprintf("  ◆ Ambiguous: '%s' exists as both file and installed thought", arg)))

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Options(
					huh.NewOption(fileOpt, "file"),
					huh.NewOption(installedOpt, "installed"),
				).
				Value(&choice),
		),
	).WithTheme(resolveTheme()).WithOutput(os.Stderr)

	if err := form.Run(); err != nil {
		return nil, fmt.Errorf("prompt cancelled")
	}

	if choice == "file" {
		return &ResolveResult{
			Path:   arg,
			Target: TargetFile,
		}, nil
	}

	return &ResolveResult{
		Path:   binPath,
		Target: TargetInstalled,
		Name:   arg,
	}, nil
}

func resolveTheme() *huh.Theme {
	t := huh.ThemeBase()
	amber := lipgloss.Color("214")

	t.Focused.Base = lipgloss.NewStyle().PaddingLeft(4)
	t.Focused.SelectSelector = lipgloss.NewStyle().Foreground(amber).SetString("❯ ")
	t.Focused.SelectedOption = lipgloss.NewStyle().Foreground(amber)
	t.Focused.UnselectedOption = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))

	return t
}
