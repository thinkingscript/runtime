package sandbox

import (
	"os"

	"github.com/dop251/goja"
)

func (s *Sandbox) registerEnv(vm *goja.Runtime) {
	env := vm.NewObject()

	env.Set("get", func(call goja.FunctionCall) goja.Value {
		name := call.Argument(0).String()

		if s.cfg.ApproveEnv != nil {
			approved, err := s.cfg.ApproveEnv(name)
			if err != nil {
				s.checkInterrupted(err)
				throwError(vm, "env.get: "+err.Error())
			}
			if !approved {
				throwError(vm, "env.get: access denied for "+name)
			}
		}

		return vm.ToValue(os.Getenv(name))
	})

	vm.Set("env", env)
}
