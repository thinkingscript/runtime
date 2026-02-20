package sandbox

import (
	"github.com/dop251/goja"
)

// ResumeError signals that the agent should take over execution.
// It's used when memory.js needs help or can't complete the task.
type ResumeError struct {
	Context string
}

func (e *ResumeError) Error() string {
	if e.Context == "" {
		return "agent.resume"
	}
	return "agent.resume: " + e.Context
}

func (s *Sandbox) registerAgent(vm *goja.Runtime) {
	agent := vm.NewObject()

	agent.Set("resume", func(call goja.FunctionCall) goja.Value {
		ctx := ""
		if len(call.Arguments) > 0 {
			ctx = call.Argument(0).String()
		}
		panic(&ResumeError{Context: ctx})
	})

	vm.Set("agent", agent)
}
