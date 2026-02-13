package approval

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/thinkingscript/cli/internal/arguments"
	"github.com/thinkingscript/cli/internal/ui"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"golang.org/x/term"
)

// ErrInterrupted is returned when the user presses Ctrl+C during a prompt.
var ErrInterrupted = errors.New("interrupted")

type Decision string

const (
	DecisionAllow         Decision = "allow"
	DecisionDeny          Decision = "deny"
	DecisionAlwaysYes     Decision = "always_yes"
	DecisionAlwaysSimilar Decision = "always_similar"
)

type StoredApprovals struct {
	Commands        map[string]Decision `json:"commands"`
	EnvVars         map[string]Decision `json:"env_vars"`
	Arguments       map[string]Decision `json:"arguments"`
	CommandPatterns map[string]Decision `json:"command_patterns,omitempty"`
}

type Approver struct {
	wreckless  bool
	cacheDir   string
	stored     *StoredApprovals
	isTTY      bool
	argStore   *arguments.Store
	scriptName string
	ttyInput   *os.File
}

func NewApprover(wreckless bool, cacheDir string, argStore *arguments.Store, scriptName string) *Approver {
	isTTY := term.IsTerminal(int(os.Stderr.Fd()))

	// When stdin is a pipe but stderr is a terminal, open /dev/tty
	// for interactive input so approval prompts work in pipelines.
	var ttyInput *os.File
	if isTTY && !term.IsTerminal(int(os.Stdin.Fd())) {
		ttyInput = openTTY()
		if ttyInput == nil {
			isTTY = false
		}
	}

	a := &Approver{
		wreckless:  wreckless,
		cacheDir:   cacheDir,
		isTTY:      isTTY,
		argStore:   argStore,
		scriptName: scriptName,
		ttyInput:   ttyInput,
		stored:     &StoredApprovals{},
	}
	a.loadStored()
	return a
}

func (a *Approver) ApproveCommand(command string) (bool, error) {
	if a.wreckless {
		return true, nil
	}

	// Check exact match
	if d, ok := a.stored.Commands[command]; ok {
		return d == DecisionAllow || d == DecisionAlwaysYes, nil
	}

	// Check pattern matches
	args := a.argStore.Snapshot()
	for pattern, d := range a.stored.CommandPatterns {
		if patternMatchesCommand(pattern, command, args) {
			return d == DecisionAlwaysSimilar, nil
		}
	}

	if !a.isTTY {
		return false, nil
	}

	// Determine if named arguments appear in the command (enables "similar" option)
	hasArgs := commandContainsArgument(command, args)

	decision, err := a.promptCommand(command, hasArgs, args)
	if err != nil {
		return false, err
	}

	switch decision {
	case DecisionAlwaysYes:
		a.stored.Commands[command] = DecisionAlwaysYes
		a.saveStored()
	case DecisionAlwaysSimilar:
		pattern := commandToPattern(command, args)
		a.stored.CommandPatterns[pattern] = DecisionAlwaysSimilar
		a.saveStored()
	}

	return decision == DecisionAllow || decision == DecisionAlwaysYes || decision == DecisionAlwaysSimilar, nil
}

func (a *Approver) ApproveEnvRead(varName string) (bool, error) {
	return a.approveSimple(a.stored.EnvVars, "read_env", varName)
}

func (a *Approver) ApproveArgument(detail string) (bool, error) {
	return a.approveSimple(a.stored.Arguments, "set_argument", detail)
}

// approveSimple handles the standard approval flow for simple tool actions
// (env reads, argument assignments). Commands use their own flow with pattern matching.
func (a *Approver) approveSimple(stored map[string]Decision, label, key string) (bool, error) {
	if a.wreckless {
		return true, nil
	}

	if d, ok := stored[key]; ok {
		return d == DecisionAlwaysYes, nil
	}

	if !a.isTTY {
		return false, nil
	}

	decision, err := a.prompt(label, key)
	if err != nil {
		return false, err
	}

	if decision == DecisionAlwaysYes {
		stored[key] = DecisionAlwaysYes
		a.saveStored()
	}

	return decision == DecisionAllow || decision == DecisionAlwaysYes, nil
}

var (
	cmdStyle   = ui.Renderer.NewStyle().Foreground(lipgloss.Color("255"))
	labelStyle = ui.Renderer.NewStyle().Foreground(lipgloss.Color("245"))
)

// indentedTheme creates a huh theme indented to align with the tool name
// after the "● " prefix (2 chars).
func indentedTheme() *huh.Theme {
	t := huh.ThemeBase()
	t.Focused.Base = lipgloss.NewStyle().PaddingTop(1)
	t.Focused.SelectSelector = lipgloss.NewStyle().SetString("❯ ")
	t.Blurred.Base = lipgloss.NewStyle().PaddingTop(1)
	t.Blurred.SelectSelector = lipgloss.NewStyle().SetString("  ")
	return t
}

func (a *Approver) promptCommand(command string, hasArgs bool, args []arguments.Argument) (Decision, error) {
	lock, err := acquirePromptLock()
	if err != nil {
		return DecisionDeny, fmt.Errorf("acquiring prompt lock: %w", err)
	}
	defer releasePromptLock(lock)

	fmt.Fprintf(os.Stderr, "\n  %s %s\n",
		labelStyle.Render("$"),
		cmdStyle.Render(truncate(command, 200)))

	options := []huh.Option[string]{
		huh.NewOption("Yes", "yes"),
		huh.NewOption("Always", "always"),
	}

	if hasArgs {
		pattern := commandToPattern(command, args)
		label := fmt.Sprintf("Always similar (%s)", truncate(pattern, 60))
		options = append(options, huh.NewOption(label, "always_similar"))
	}

	options = append(options, huh.NewOption("No", "no"))

	var choice string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Options(options...).
				Value(&choice),
		),
	).WithTheme(indentedTheme()).WithOutput(os.Stderr)

	if a.ttyInput != nil {
		form = form.WithInput(a.ttyInput)
	}

	if err := form.Run(); err != nil {
		return DecisionDeny, ErrInterrupted
	}

	switch choice {
	case "yes":
		return DecisionAllow, nil
	case "always":
		return DecisionAlwaysYes, nil
	case "always_similar":
		return DecisionAlwaysSimilar, nil
	default:
		return DecisionDeny, nil
	}
}

func (a *Approver) prompt(toolName, detail string) (Decision, error) {
	lock, err := acquirePromptLock()
	if err != nil {
		return DecisionDeny, fmt.Errorf("acquiring prompt lock: %w", err)
	}
	defer releasePromptLock(lock)

	fmt.Fprintf(os.Stderr, "\n  %s\n",
		cmdStyle.Render(truncate(detail, 200)))

	var choice string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Options(
					huh.NewOption("Yes", "yes"),
					huh.NewOption("Always", "always"),
					huh.NewOption("No", "no"),
				).
				Value(&choice),
		),
	).WithTheme(indentedTheme()).WithOutput(os.Stderr)

	if a.ttyInput != nil {
		form = form.WithInput(a.ttyInput)
	}

	if err := form.Run(); err != nil {
		return DecisionDeny, ErrInterrupted
	}

	switch choice {
	case "yes":
		return DecisionAllow, nil
	case "always":
		return DecisionAlwaysYes, nil
	default:
		return DecisionDeny, nil
	}
}

func (a *Approver) loadStored() {
	if a.cacheDir != "" {
		path := filepath.Join(a.cacheDir, "approvals.json")
		if data, err := os.ReadFile(path); err == nil {
			_ = json.Unmarshal(data, a.stored)
		}
	}

	// Ensure all maps are initialized (covers fresh start and old JSON formats).
	if a.stored.Commands == nil {
		a.stored.Commands = make(map[string]Decision)
	}
	if a.stored.EnvVars == nil {
		a.stored.EnvVars = make(map[string]Decision)
	}
	if a.stored.Arguments == nil {
		a.stored.Arguments = make(map[string]Decision)
	}
	if a.stored.CommandPatterns == nil {
		a.stored.CommandPatterns = make(map[string]Decision)
	}
}

func (a *Approver) saveStored() {
	if a.cacheDir == "" {
		return
	}
	data, err := json.MarshalIndent(a.stored, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(filepath.Join(a.cacheDir, "approvals.json"), data, 0644)
}

func truncate(s string, max int) string {
	s = strings.ReplaceAll(s, "\n", "\\n")
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}

// commandToPattern replaces named argument values in a command with {Name} placeholders.
// Arguments are processed longest-first to prevent partial matches.
func commandToPattern(command string, args []arguments.Argument) string {
	pattern := command
	for _, a := range args {
		if a.Value != "" {
			pattern = strings.ReplaceAll(pattern, a.Value, "{"+a.Name+"}")
		}
	}
	return pattern
}

// patternMatchesCommand checks if a stored pattern matches the given command
// by substituting current argument values into the pattern's placeholders.
func patternMatchesCommand(pattern, command string, args []arguments.Argument) bool {
	expanded := pattern
	for _, a := range args {
		expanded = strings.ReplaceAll(expanded, "{"+a.Name+"}", a.Value)
	}
	return expanded == command
}

// commandContainsArgument reports whether any named argument value appears in the command.
func commandContainsArgument(command string, args []arguments.Argument) bool {
	for _, a := range args {
		if a.Value != "" && strings.Contains(command, a.Value) {
			return true
		}
	}
	return false
}
