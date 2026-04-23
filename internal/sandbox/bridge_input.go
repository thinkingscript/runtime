package sandbox

import (
	"github.com/dop251/goja"
)

func (s *Sandbox) registerInput(vm *goja.Runtime) {
	input := vm.NewObject()

	input.Set("prompt", func(call goja.FunctionCall) goja.Value {
		question := call.Argument(0).String()

		defaultValue := ""
		if len(call.Arguments) > 1 {
			opts := call.Argument(1)
			if !goja.IsUndefined(opts) && !goja.IsNull(opts) {
				if obj := opts.ToObject(vm); obj != nil {
					if d := obj.Get("default"); d != nil && !goja.IsUndefined(d) && !goja.IsNull(d) {
						defaultValue = d.String()
					}
				}
			}
		}

		if s.cfg.PromptInput == nil {
			throwError(vm, "input.prompt: no interactive input available")
		}

		answer, err := s.cfg.PromptInput(question, defaultValue)
		if err != nil {
			s.checkInterrupted(err)
			throwError(vm, "input.prompt: "+err.Error())
		}
		return vm.ToValue(answer)
	})

	vm.Set("input", input)
}
