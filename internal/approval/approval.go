package approval

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/thinkingscript/cli/internal/ui"
	"golang.org/x/term"
	"gopkg.in/yaml.v3"
)

// ErrInterrupted is returned when the user presses Ctrl+C during a prompt.
var ErrInterrupted = errors.New("interrupted")

type Decision string

const (
	DecisionAllow Decision = "allow"
	DecisionDeny  Decision = "deny"

	decisionOnce    Decision = "once"    // one-time allow, not persisted
	decisionSession Decision = "session" // session-scoped, not persisted
)

// Policy represents the YAML policy file format.
type Policy struct {
	Net   Decision            `yaml:"net,omitempty"`
	Env   map[string]Decision `yaml:"env,omitempty"`
	Paths map[string]Decision `yaml:"paths,omitempty"`
}

type Approver struct {
	thoughtDir       string
	globalPolicyPath string
	thoughtPolicy    *Policy // read-write, saved to thoughtDir/policy.yaml
	globalPolicy     *Policy // read-only
	isTTY            bool
	ttyInput         *os.File
	sessionAllowEnv  bool // "Allow all env reads" for this session
	sessionAllowFS   bool // "Allow all path access" for this session
	sessionAllowNet  bool // "Allow all net access" for this session
}

func NewApprover(thoughtDir, globalPolicyPath string) *Approver {
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
		thoughtDir:       thoughtDir,
		globalPolicyPath: globalPolicyPath,
		isTTY:            isTTY,
		ttyInput:         ttyInput,
		thoughtPolicy:    &Policy{},
		globalPolicy:     &Policy{},
	}
	a.loadPolicies()
	return a
}

// Close releases resources held by the Approver.
func (a *Approver) Close() {
	if a.ttyInput != nil {
		a.ttyInput.Close()
		a.ttyInput = nil
	}
}

func (a *Approver) ApproveNet() (bool, error) {
	if a.sessionAllowNet {
		return true, nil
	}

	// Check thought policy
	if a.thoughtPolicy.Net == DecisionAllow {
		return true, nil
	}
	if a.thoughtPolicy.Net == DecisionDeny {
		return false, nil
	}

	// Check global policy
	if a.globalPolicy.Net == DecisionAllow {
		return true, nil
	}
	if a.globalPolicy.Net == DecisionDeny {
		return false, nil
	}

	if !a.isTTY {
		return false, nil
	}

	decision, err := a.promptWithBatch("net", "network access")
	if err != nil {
		return false, err
	}

	switch decision {
	case DecisionAllow:
		a.thoughtPolicy.Net = DecisionAllow
		a.saveThoughtPolicy()
	case DecisionDeny:
		a.thoughtPolicy.Net = DecisionDeny
		a.saveThoughtPolicy()
	case decisionSession:
		a.sessionAllowNet = true
	}

	return decision == DecisionAllow || decision == decisionOnce || decision == decisionSession, nil
}

func (a *Approver) ApprovePath(op, path string) (bool, error) {
	if a.sessionAllowFS {
		return true, nil
	}

	// Check if this path (or any parent) is already approved in thought policy.
	if a.pathApproved(a.thoughtPolicy, path) {
		return true, nil
	}

	// Check global policy.
	if a.pathApproved(a.globalPolicy, path) {
		return true, nil
	}

	// Check if explicitly denied in thought policy.
	if a.pathDenied(a.thoughtPolicy, path) {
		return false, nil
	}

	if !a.isTTY {
		return false, nil
	}

	decision, err := a.promptWithBatch(op, path)
	if err != nil {
		return false, err
	}

	switch decision {
	case DecisionAllow:
		if a.thoughtPolicy.Paths == nil {
			a.thoughtPolicy.Paths = make(map[string]Decision)
		}
		a.thoughtPolicy.Paths[path] = DecisionAllow
		a.saveThoughtPolicy()
	case DecisionDeny:
		if a.thoughtPolicy.Paths == nil {
			a.thoughtPolicy.Paths = make(map[string]Decision)
		}
		a.thoughtPolicy.Paths[path] = DecisionDeny
		a.saveThoughtPolicy()
	case decisionSession:
		a.sessionAllowFS = true
	}

	return decision == DecisionAllow || decision == decisionOnce || decision == decisionSession, nil
}

// pathApproved checks if the given path or any of its parent directories
// has been approved. This means approving /Users/brad covers /Users/brad/foo.jpg.
func (a *Approver) pathApproved(policy *Policy, path string) bool {
	for approved, d := range policy.Paths {
		if d != DecisionAllow {
			continue
		}
		if path == approved || strings.HasPrefix(path, approved+string(filepath.Separator)) {
			return true
		}
	}
	return false
}

func (a *Approver) pathDenied(policy *Policy, path string) bool {
	for denied, d := range policy.Paths {
		if d != DecisionDeny {
			continue
		}
		if path == denied || strings.HasPrefix(path, denied+string(filepath.Separator)) {
			return true
		}
	}
	return false
}

func (a *Approver) ApproveEnvRead(varName string) (bool, error) {
	if a.sessionAllowEnv {
		return true, nil
	}

	// Check thought policy
	if d, ok := a.thoughtPolicy.Env[varName]; ok {
		return d == DecisionAllow, nil
	}

	// Check global policy
	if d, ok := a.globalPolicy.Env[varName]; ok {
		return d == DecisionAllow, nil
	}

	if !a.isTTY {
		return false, nil
	}

	decision, err := a.promptWithBatch("env", varName)
	if err != nil {
		return false, err
	}

	switch decision {
	case DecisionAllow:
		if a.thoughtPolicy.Env == nil {
			a.thoughtPolicy.Env = make(map[string]Decision)
		}
		a.thoughtPolicy.Env[varName] = DecisionAllow
		a.saveThoughtPolicy()
	case DecisionDeny:
		if a.thoughtPolicy.Env == nil {
			a.thoughtPolicy.Env = make(map[string]Decision)
		}
		a.thoughtPolicy.Env[varName] = DecisionDeny
		a.saveThoughtPolicy()
	case decisionSession:
		a.sessionAllowEnv = true
	}

	return decision == DecisionAllow || decision == decisionOnce || decision == decisionSession, nil
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

// promptWithBatch returns DecisionAllow (persist), decisionOnce (one-time),
// decisionSession (session-scoped), or DecisionDeny.
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
		return decisionOnce, nil
	case "allow_all":
		return decisionSession, nil
	case "always":
		return DecisionAllow, nil
	default:
		return DecisionDeny, nil
	}
}

func (a *Approver) loadPolicies() {
	// Load global policy (read-only)
	if a.globalPolicyPath != "" {
		if data, err := os.ReadFile(a.globalPolicyPath); err == nil {
			if err := yaml.Unmarshal(data, a.globalPolicy); err != nil {
				fmt.Fprintf(os.Stderr, "warning: corrupted global policy file, ignoring: %v\n", err)
			}
		}
	}

	// Load thought policy (read-write)
	if a.thoughtDir != "" {
		path := filepath.Join(a.thoughtDir, "policy.yaml")
		if data, err := os.ReadFile(path); err == nil {
			if err := yaml.Unmarshal(data, a.thoughtPolicy); err != nil {
				fmt.Fprintf(os.Stderr, "warning: corrupted thought policy file, re-prompting: %v\n", err)
			}
		}
	}

	// Ensure maps are initialized
	if a.thoughtPolicy.Env == nil {
		a.thoughtPolicy.Env = make(map[string]Decision)
	}
	if a.thoughtPolicy.Paths == nil {
		a.thoughtPolicy.Paths = make(map[string]Decision)
	}
	if a.globalPolicy.Env == nil {
		a.globalPolicy.Env = make(map[string]Decision)
	}
	if a.globalPolicy.Paths == nil {
		a.globalPolicy.Paths = make(map[string]Decision)
	}
}

func (a *Approver) saveThoughtPolicy() {
	if a.thoughtDir == "" {
		return
	}

	if err := os.MkdirAll(a.thoughtDir, 0700); err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to create thought dir: %v\n", err)
		return
	}

	data, err := yaml.Marshal(a.thoughtPolicy)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to marshal policy: %v\n", err)
		return
	}
	if err := os.WriteFile(filepath.Join(a.thoughtDir, "policy.yaml"), data, 0600); err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to save policy: %v\n", err)
	}
}

func truncate(s string, max int) string {
	s = strings.ReplaceAll(s, "\n", "\\n")
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}
