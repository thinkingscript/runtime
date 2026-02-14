package approval

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/thinkingscript/cli/internal/ui"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"golang.org/x/term"
)

// ErrInterrupted is returned when the user presses Ctrl+C during a prompt.
var ErrInterrupted = errors.New("interrupted")

type Decision string

const (
	DecisionAllow     Decision = "allow"
	DecisionDeny      Decision = "deny"
	DecisionAlwaysYes Decision = "always_yes"
	DecisionAllowAll  Decision = "allow_all" // session-scoped, not persisted
)

type StoredApprovals struct {
	EnvVars map[string]Decision `json:"env_vars"`
	Paths   map[string]Decision `json:"paths"`
}

type Approver struct {
	cacheDir        string
	stored          *StoredApprovals
	isTTY           bool
	ttyInput        *os.File
	sessionAllowEnv bool // "Allow all env reads" for this session
	sessionAllowFS  bool // "Allow all path access" for this session
}

func NewApprover(cacheDir string) *Approver {
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
		cacheDir: cacheDir,
		isTTY:    isTTY,
		ttyInput: ttyInput,
		stored:   &StoredApprovals{},
	}
	a.loadStored()
	return a
}

// Close releases resources held by the Approver.
func (a *Approver) Close() {
	if a.ttyInput != nil {
		a.ttyInput.Close()
		a.ttyInput = nil
	}
}

func (a *Approver) ApprovePath(op, path string) (bool, error) {
	if a.sessionAllowFS {
		return true, nil
	}

	// Check if this path (or any parent) is already approved.
	if a.pathApproved(path) {
		return true, nil
	}

	if !a.isTTY {
		return false, nil
	}

	decision, err := a.promptWithBatch(op, path)
	if err != nil {
		return false, err
	}

	switch decision {
	case DecisionAlwaysYes:
		a.stored.Paths[path] = DecisionAlwaysYes
		a.saveStored()
	case DecisionAllowAll:
		a.sessionAllowFS = true
	}

	return decision == DecisionAllow || decision == DecisionAlwaysYes || decision == DecisionAllowAll, nil
}

// pathApproved checks if the given path or any of its parent directories
// has been approved. This means approving /Users/brad covers /Users/brad/foo.jpg.
func (a *Approver) pathApproved(path string) bool {
	for approved, d := range a.stored.Paths {
		if d != DecisionAlwaysYes {
			continue
		}
		if path == approved || strings.HasPrefix(path, approved+string(filepath.Separator)) {
			return true
		}
	}
	return false
}

func (a *Approver) ApproveEnvRead(varName string) (bool, error) {
	if a.sessionAllowEnv {
		return true, nil
	}
	return a.approveWithBatch(a.stored.EnvVars, "env", varName, &a.sessionAllowEnv)
}

// approveWithBatch handles approval with a session-level "Allow all" option.
func (a *Approver) approveWithBatch(stored map[string]Decision, label, key string, sessionAllow *bool) (bool, error) {
	if d, ok := stored[key]; ok {
		return d == DecisionAlwaysYes, nil
	}

	if !a.isTTY {
		return false, nil
	}

	decision, err := a.promptWithBatch(label, key)
	if err != nil {
		return false, err
	}

	switch decision {
	case DecisionAlwaysYes:
		stored[key] = DecisionAlwaysYes
		a.saveStored()
	case DecisionAllowAll:
		*sessionAllow = true
	}

	return decision == DecisionAllow || decision == DecisionAlwaysYes || decision == DecisionAllowAll, nil
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

func (a *Approver) promptWithBatch(label, detail string) (Decision, error) {
	lock, err := acquirePromptLock()
	if err != nil {
		return DecisionDeny, fmt.Errorf("acquiring prompt lock: %w", err)
	}
	defer releasePromptLock(lock)

	fmt.Fprintf(os.Stderr, "\n  %s %s\n",
		labelStyle.Render(label),
		cmdStyle.Render(truncate(detail, 200)))

	var choice string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Options(
					huh.NewOption("Yes", "yes"),
					huh.NewOption("Allow all", "allow_all"),
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
		os.Exit(130)
	}

	switch choice {
	case "yes":
		return DecisionAllow, nil
	case "allow_all":
		return DecisionAllowAll, nil
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
			if err := json.Unmarshal(data, a.stored); err != nil {
				fmt.Fprintf(os.Stderr, "warning: corrupted approvals file, re-prompting for approvals: %v\n", err)
			}
		}
	}

	// Ensure all maps are initialized (covers fresh start and old JSON formats).
	if a.stored.EnvVars == nil {
		a.stored.EnvVars = make(map[string]Decision)
	}
	if a.stored.Paths == nil {
		a.stored.Paths = make(map[string]Decision)
	}
}

func (a *Approver) saveStored() {
	if a.cacheDir == "" {
		return
	}
	data, err := json.MarshalIndent(a.stored, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to marshal approvals: %v\n", err)
		return
	}
	if err := os.WriteFile(filepath.Join(a.cacheDir, "approvals.json"), data, 0600); err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to save approvals: %v\n", err)
	}
}

func truncate(s string, max int) string {
	s = strings.ReplaceAll(s, "\n", "\\n")
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}

