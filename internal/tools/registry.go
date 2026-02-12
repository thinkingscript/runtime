package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/bradgessler/agent-exec/internal/approval"
	"github.com/bradgessler/agent-exec/internal/provider"
)

type Handler func(ctx context.Context, input json.RawMessage) (string, error)

type Registry struct {
	tools    map[string]provider.ToolDefinition
	handlers map[string]Handler
	order    []string
}

func NewRegistry(approver *approval.Approver, stdinData string) *Registry {
	r := &Registry{
		tools:    make(map[string]provider.ToolDefinition),
		handlers: make(map[string]Handler),
	}

	r.registerStdio()
	r.registerStdin(stdinData)
	r.registerEnv(approver)
	r.registerCommand(approver)

	return r
}

func (r *Registry) register(def provider.ToolDefinition, handler Handler) {
	r.tools[def.Name] = def
	r.handlers[def.Name] = handler
	r.order = append(r.order, def.Name)
}

func (r *Registry) Definitions() []provider.ToolDefinition {
	defs := make([]provider.ToolDefinition, 0, len(r.order))
	for _, name := range r.order {
		defs = append(defs, r.tools[name])
	}
	return defs
}

func (r *Registry) Execute(ctx context.Context, name string, input json.RawMessage) (string, error) {
	handler, ok := r.handlers[name]
	if !ok {
		return "", fmt.Errorf("unknown tool: %s", name)
	}
	return handler(ctx, input)
}
