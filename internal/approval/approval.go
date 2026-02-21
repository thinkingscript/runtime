package approval

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/thinkingscript/cli/internal/ui"
	"golang.org/x/term"
)

// ErrInterrupted is returned when the user presses Ctrl+C during a prompt.
var ErrInterrupted = errors.New("interrupted")

// promptDecision represents the user's choice from a prompt.
type promptDecision string

const (
	promptOnce   promptDecision = "once"   // one-time allow, not persisted
	promptAlways promptDecision = "always" // persist allow to policy
	promptDeny   promptDecision = "deny"   // persist deny to policy
)

// Approver handles permission checks against policies.
type Approver struct {
	thoughtDir       string
	globalPolicyPath string
	thoughtPolicy    *Policy // read-write, saved to thoughtDir/policy.json
	globalPolicy     *Policy // read-only
	isTTY            bool
	ttyInput         *os.File
}

// NewApprover creates an Approver that checks policies and prompts for approval.
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
		thoughtPolicy:    NewPolicy(),
		globalPolicy:     NewPolicy(),
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

// ApproveNet checks if network access to a specific host is allowed.
func (a *Approver) ApproveNet(host string) (bool, error) {
	// Check thought policy
	if entry := a.thoughtPolicy.Net.Hosts.MatchHost(host); entry != nil {
		if entry.Approval == ApprovalAllow {
			return true, nil
		}
		if entry.Approval == ApprovalDeny {
			return false, nil
		}
		// ApprovalPrompt falls through to prompt
	}

	// Check global policy
	if entry := a.globalPolicy.Net.Hosts.MatchHost(host); entry != nil {
		if entry.Approval == ApprovalAllow {
			return true, nil
		}
		if entry.Approval == ApprovalDeny {
			return false, nil
		}
	}

	// Check defaults
	if a.thoughtPolicy.Net.Hosts.Default == ApprovalAllow {
		return true, nil
	}
	if a.thoughtPolicy.Net.Hosts.Default == ApprovalDeny {
		return false, nil
	}
	if a.globalPolicy.Net.Hosts.Default == ApprovalAllow {
		return true, nil
	}
	if a.globalPolicy.Net.Hosts.Default == ApprovalDeny {
		return false, nil
	}

	if !a.isTTY {
		return false, nil
	}

	decision, err := a.prompt("net", host)
	if err != nil {
		return false, err
	}

	switch decision {
	case promptAlways:
		a.thoughtPolicy.AddHostEntry(host, ApprovalAllow, SourcePrompt)
		a.saveThoughtPolicy()
	case promptDeny:
		a.thoughtPolicy.AddHostEntry(host, ApprovalDeny, SourcePrompt)
		a.saveThoughtPolicy()
	}

	return decision == promptAlways || decision == promptOnce, nil
}

// ApprovePath checks if a filesystem operation on a path is allowed.
// The op parameter is one of "read", "write", "delete".
func (a *Approver) ApprovePath(op, path string) (bool, error) {
	// SECURITY: Never allow modifying the thought's own policy file
	if a.thoughtDir != "" {
		policyPath := filepath.Join(a.thoughtDir, "policy.json")
		if path == policyPath || strings.HasPrefix(path, policyPath) {
			return false, nil
		}
	}

	modeChar := opToModeChar(op)

	// Check global protected entries FIRST - these cannot be overridden
	for _, entry := range a.globalPolicy.Paths.Protected {
		if pathMatches(entry.Path, path) && hasMode(entry.Mode, modeChar) {
			if entry.Approval == ApprovalDeny {
				return false, nil // Protected deny cannot be overridden
			}
			if entry.Approval == ApprovalAllow {
				return true, nil
			}
		}
	}

	// Check thought policy
	if entry := a.thoughtPolicy.Paths.MatchPath(path); entry != nil {
		if hasMode(entry.Mode, modeChar) {
			if entry.Approval == ApprovalAllow {
				return true, nil
			}
			if entry.Approval == ApprovalDeny {
				return false, nil
			}
		}
	}

	// Check regular global policy entries
	if entry := a.globalPolicy.Paths.MatchPath(path); entry != nil {
		if hasMode(entry.Mode, modeChar) {
			if entry.Approval == ApprovalAllow {
				return true, nil
			}
			if entry.Approval == ApprovalDeny {
				return false, nil
			}
		}
	}

	// Check defaults
	if a.thoughtPolicy.Paths.Default == ApprovalAllow {
		return true, nil
	}
	if a.thoughtPolicy.Paths.Default == ApprovalDeny {
		return false, nil
	}
	if a.globalPolicy.Paths.Default == ApprovalAllow {
		return true, nil
	}
	if a.globalPolicy.Paths.Default == ApprovalDeny {
		return false, nil
	}

	if !a.isTTY {
		return false, nil
	}

	decision, err := a.prompt(op, path)
	if err != nil {
		return false, err
	}

	switch decision {
	case promptAlways:
		// When approving, grant the specific mode requested
		a.thoughtPolicy.AddPathEntry(path, modeChar, ApprovalAllow, SourcePrompt)
		a.saveThoughtPolicy()
	case promptDeny:
		a.thoughtPolicy.AddPathEntry(path, modeChar, ApprovalDeny, SourcePrompt)
		a.saveThoughtPolicy()
	}

	return decision == promptAlways || decision == promptOnce, nil
}

// ApproveEnvRead checks if reading an environment variable is allowed.
func (a *Approver) ApproveEnvRead(varName string) (bool, error) {
	// Check thought policy
	if entry := a.thoughtPolicy.Env.MatchEnv(varName); entry != nil {
		if entry.Approval == ApprovalAllow {
			return true, nil
		}
		if entry.Approval == ApprovalDeny {
			return false, nil
		}
	}

	// Check global policy
	if entry := a.globalPolicy.Env.MatchEnv(varName); entry != nil {
		if entry.Approval == ApprovalAllow {
			return true, nil
		}
		if entry.Approval == ApprovalDeny {
			return false, nil
		}
	}

	// Check defaults
	if a.thoughtPolicy.Env.Default == ApprovalAllow {
		return true, nil
	}
	if a.thoughtPolicy.Env.Default == ApprovalDeny {
		return false, nil
	}
	if a.globalPolicy.Env.Default == ApprovalAllow {
		return true, nil
	}
	if a.globalPolicy.Env.Default == ApprovalDeny {
		return false, nil
	}

	if !a.isTTY {
		return false, nil
	}

	decision, err := a.prompt("env", varName)
	if err != nil {
		return false, err
	}

	switch decision {
	case promptAlways:
		a.thoughtPolicy.AddEnvEntry(varName, ApprovalAllow, SourcePrompt)
		a.saveThoughtPolicy()
	case promptDeny:
		a.thoughtPolicy.AddEnvEntry(varName, ApprovalDeny, SourcePrompt)
		a.saveThoughtPolicy()
	}

	return decision == promptAlways || decision == promptOnce, nil
}

// opToModeChar converts an operation name to a mode character.
func opToModeChar(op string) string {
	switch op {
	case "read", "list":
		return "r"
	case "write":
		return "w"
	case "delete":
		return "d"
	default:
		return "r"
	}
}

// hasMode checks if mode string contains the given mode character.
func hasMode(mode, char string) bool {
	return strings.Contains(mode, char)
}

// Prompt styles
var (
	amber    = lipgloss.Color("214")
	dimColor = lipgloss.Color("242")

	markerStyle  = ui.Renderer.NewStyle().Foreground(amber).Bold(true)
	opStyle      = ui.Renderer.NewStyle().Foreground(amber).Bold(true)
	detailStyle  = ui.Renderer.NewStyle().Foreground(lipgloss.Color("255"))
	numberStyle  = ui.Renderer.NewStyle().Foreground(dimColor)
	commandStyle = ui.Renderer.NewStyle().Bold(true).Foreground(lipgloss.Color("255"))
)

// approvalModel is a bubbletea model for the approval prompt
type approvalModel struct {
	cursor int
	choice string
	done   bool
}

var choices = []string{"allow", "deny"}

func (m approvalModel) Init() tea.Cmd {
	return nil
}

func (m approvalModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(choices)-1 {
				m.cursor++
			}
		case "enter", " ":
			m.choice = choices[m.cursor]
			m.done = true
			return m, tea.Quit
		case "1":
			m.choice = "allow"
			m.done = true
			return m, tea.Quit
		case "2":
			m.choice = "deny"
			m.done = true
			return m, tea.Quit
		case "ctrl+c", "esc":
			m.choice = ""
			m.done = true
			return m, tea.Quit
		}
	}
	return m, nil
}

var (
	selectedStyle   = ui.Renderer.NewStyle().Foreground(amber)
	unselectedStyle = ui.Renderer.NewStyle().Foreground(dimColor)
)

func (m approvalModel) View() string {
	if m.done {
		return ""
	}

	type option struct {
		num string
		cmd string
	}
	options := []option{
		{"1", "Allow"},
		{"2", "Deny"},
	}

	// Layout: numbers under ◆, commands under NET label
	// Header is: "\n  ◆ NET  detail"
	// Options:   "❯ 1 Allow" or "  2 Deny"
	var b strings.Builder
	b.WriteString("\n")
	for i, opt := range options {
		if i == m.cursor {
			// Selected: amber arrow, bright text
			b.WriteString(fmt.Sprintf("%s %s %s\n",
				selectedStyle.Render("❯"),
				numberStyle.Render(opt.num),
				commandStyle.Render(opt.cmd)))
		} else {
			// Unselected: dim text, 2-space indent instead of arrow
			b.WriteString(fmt.Sprintf("  %s %s\n",
				unselectedStyle.Render(opt.num),
				unselectedStyle.Render(opt.cmd)))
		}
	}
	return b.String()
}

// prompt shows an approval dialog and returns the user's decision.
func (a *Approver) prompt(label, detail string) (promptDecision, error) {
	lock, err := acquirePromptLock()
	if err != nil {
		return promptDeny, fmt.Errorf("acquiring prompt lock: %w", err)
	}
	defer releasePromptLock(lock)

	fmt.Fprintf(os.Stderr, "\n  %s %s  %s\n",
		markerStyle.Render("◆"),
		opStyle.Render(strings.ToUpper(label)),
		detailStyle.Render(truncate(detail, 200)))

	// Set up bubbletea options
	opts := []tea.ProgramOption{
		tea.WithOutput(os.Stderr),
	}
	if a.ttyInput != nil {
		opts = append(opts, tea.WithInput(a.ttyInput))
	}

	p := tea.NewProgram(approvalModel{}, opts...)
	finalModel, err := p.Run()
	if err != nil {
		return promptDeny, err
	}

	m := finalModel.(approvalModel)
	if m.choice == "" {
		os.Exit(130)
	}

	switch m.choice {
	case "allow":
		return promptAlways, nil
	case "deny":
		return promptDeny, nil
	default:
		return promptDeny, nil
	}
}

func (a *Approver) loadPolicies() {
	// Load global policy (read-only)
	if a.globalPolicyPath != "" {
		if policy, err := LoadPolicy(a.globalPolicyPath); err == nil {
			a.globalPolicy = policy
		} else if !os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "warning: corrupted global policy file, ignoring: %v\n", err)
		}
	}

	// Load thought policy (read-write)
	if a.thoughtDir != "" {
		path := filepath.Join(a.thoughtDir, "policy.json")
		if policy, err := LoadPolicy(path); err == nil {
			a.thoughtPolicy = policy
		} else if !os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "warning: corrupted thought policy file, re-prompting: %v\n", err)
		}
	}
}

func (a *Approver) saveThoughtPolicy() {
	if a.thoughtDir == "" {
		return
	}

	path := filepath.Join(a.thoughtDir, "policy.json")
	if err := a.thoughtPolicy.Save(path); err != nil {
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

// BootstrapDefaults adds default policy entries for workspace and memories.
// These are auto-approved paths that the thought can always access.
// Policy.json is explicitly denied to prevent privilege escalation.
func (a *Approver) BootstrapDefaults(workspaceDir, memoriesDir, workDir string) {
	// Only bootstrap if no entries exist yet
	if len(a.thoughtPolicy.Paths.Entries) > 0 {
		return
	}

	// Workspace: full read/write/delete (agent's scratch space)
	if workspaceDir != "" {
		a.thoughtPolicy.AddPathEntry(workspaceDir, "rwd", ApprovalAllow, SourceDefault)
	}

	// Memories: full read/write/delete
	if memoriesDir != "" {
		a.thoughtPolicy.AddPathEntry(memoriesDir, "rwd", ApprovalAllow, SourceDefault)
	}

	// CWD: read-only by default
	if workDir != "" {
		a.thoughtPolicy.AddPathEntry(workDir, "r", ApprovalAllow, SourceDefault)
	}

	// policy.json: ALWAYS deny to prevent privilege escalation
	// The agent can never modify its own policy
	policyPath := filepath.Join(a.thoughtDir, "policy.json")
	a.thoughtPolicy.AddPathEntry(policyPath, "rwd", ApprovalDeny, SourceDefault)

	a.saveThoughtPolicy()
}
