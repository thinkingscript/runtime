package sandbox

import (
	"fmt"
	"os"
	"time"

	"github.com/dop251/goja"
)

func (s *Sandbox) registerProcess(vm *goja.Runtime) {
	process := vm.NewObject()

	process.Set("cwd", func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(s.cfg.WorkDir)
	})

	process.Set("args", vm.ToValue(s.cfg.Args))

	process.Set("exit", func(call goja.FunctionCall) goja.Value {
		code := 0
		if len(call.Arguments) > 0 {
			code = int(call.Argument(0).ToInteger())
		}
		panic(&exitError{code: code})
	})

	process.Set("sleep", func(call goja.FunctionCall) goja.Value {
		ms := call.Argument(0).ToInteger()
		if ms <= 0 {
			return goja.Undefined()
		}
		select {
		case <-time.After(time.Duration(ms) * time.Millisecond):
		case <-s.ctx.Done():
			throwError(vm, "sleep interrupted")
		}
		return goja.Undefined()
	})

	stdout := vm.NewObject()
	stdout.Set("write", func(call goja.FunctionCall) goja.Value {
		text := call.Argument(0).String()
		fmt.Fprint(os.Stdout, text)
		return goja.Undefined()
	})
	process.Set("stdout", stdout)

	vm.Set("process", process)
}
