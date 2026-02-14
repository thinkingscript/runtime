package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/thinkingscript/cli/internal/approval"
	"github.com/thinkingscript/cli/internal/provider"
)

// ApproveFunc inspects tool input and decides whether the action is allowed.
// Return (true, nil) to proceed, (false, nil) to deny.
// Tools that don't need approval pass nil instead.
type ApproveFunc func(input json.RawMessage) (bool, error)

// Handler executes a tool after approval has been granted.
type Handler func(ctx context.Context, input json.RawMessage) (string, error)

type registration struct {
	def     provider.ToolDefinition
	approve ApproveFunc
	handler Handler
}

type Registry struct {
	regs  map[string]registration
	order []string
}

func NewRegistry(approver *approval.Approver, workDir, workspaceDir, memoriesDir, scriptName string) *Registry {
	r := &Registry{
		regs: make(map[string]registration),
	}

	r.registerStdio()
	r.registerScript(approver, workDir, workspaceDir, memoriesDir, scriptName)

	return r
}

func (r *Registry) register(def provider.ToolDefinition, handler Handler, approve ApproveFunc) {
	r.regs[def.Name] = registration{def: def, handler: handler, approve: approve}
	r.order = append(r.order, def.Name)
}

func (r *Registry) Definitions() []provider.ToolDefinition {
	defs := make([]provider.ToolDefinition, 0, len(r.order))
	for _, name := range r.order {
		defs = append(defs, r.regs[name].def)
	}
	return defs
}

// Execute runs a tool by name. If the tool has an ApproveFunc, it is called
// first — this is the single security chokepoint for all tool execution.
func (r *Registry) Execute(ctx context.Context, name string, input json.RawMessage) (string, error) {
	reg, ok := r.regs[name]
	if !ok {
		return "", fmt.Errorf("unknown tool: %s", name)
	}

	if reg.approve != nil {
		approved, err := reg.approve(input)
		if err != nil {
			return "", err
		}
		if !approved {
			return "", fmt.Errorf("denied: %s", name)
		}
	}

	return reg.handler(ctx, input)
}
