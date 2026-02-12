package approval

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

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
)

type StoredApprovals struct {
	Commands map[string]Decision `json:"commands"`
	EnvVars  map[string]Decision `json:"env_vars"`
}

type Approver struct {
	wreckless bool
	cacheDir  string
	stored    *StoredApprovals
	isTTY     bool
}

func NewApprover(wreckless bool, cacheDir string) *Approver {
	a := &Approver{
		wreckless: wreckless,
		cacheDir:  cacheDir,
		isTTY:     term.IsTerminal(int(os.Stderr.Fd())),
		stored: &StoredApprovals{
			Commands: make(map[string]Decision),
			EnvVars:  make(map[string]Decision),
		},
	}
	a.loadStored()
	return a
}

func (a *Approver) ApproveCommand(command string) (bool, error) {
	if a.wreckless {
		return true, nil
	}

	if d, ok := a.stored.Commands[command]; ok {
		return d == DecisionAllow || d == DecisionAlwaysYes, nil
	}

	if !a.isTTY {
		return false, nil
	}

	decision, err := a.prompt("run_command", command)
	if err != nil {
		return false, err
	}

	if decision == DecisionAlwaysYes {
		a.stored.Commands[command] = DecisionAlwaysYes
		a.saveStored()
	}

	return decision == DecisionAllow || decision == DecisionAlwaysYes, nil
}

func (a *Approver) ApproveEnvRead(varName string) (bool, error) {
	if a.wreckless {
		return true, nil
	}

	if d, ok := a.stored.EnvVars[varName]; ok {
		return d == DecisionAllow || d == DecisionAlwaysYes, nil
	}

	if !a.isTTY {
		return false, nil
	}

	decision, err := a.prompt("read_env", varName)
	if err != nil {
		return false, err
	}

	if decision == DecisionAlwaysYes {
		a.stored.EnvVars[varName] = DecisionAlwaysYes
		a.saveStored()
	}

	return decision == DecisionAllow || decision == DecisionAlwaysYes, nil
}

var (
	warningStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Bold(true)
	detailStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
)

func (a *Approver) prompt(toolName, detail string) (Decision, error) {
	label := toolName
	switch toolName {
	case "run_command":
		label = "Execute command"
	case "read_env":
		label = "Read environment variable"
	}

	// Print the request to stderr
	fmt.Fprintf(os.Stderr, "\n%s %s\n", warningStyle.Render(label+":"), detailStyle.Render(truncate(detail, 200)))

	var choice string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Allow this action?").
				Options(
					huh.NewOption("Yes", "yes"),
					huh.NewOption("No", "no"),
					huh.NewOption("Always (remember for this script)", "always"),
				).
				Value(&choice),
		),
	).WithOutput(os.Stderr)

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
	if a.cacheDir == "" {
		return
	}
	path := filepath.Join(a.cacheDir, "approvals.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	_ = json.Unmarshal(data, a.stored)
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
