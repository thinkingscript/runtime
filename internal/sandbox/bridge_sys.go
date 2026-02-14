package sandbox

import (
	"os"
	"runtime"

	"github.com/dop251/goja"
	"golang.org/x/term"
)

func (s *Sandbox) registerSys(vm *goja.Runtime) {
	sys := vm.NewObject()

	sys.Set("platform", func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(runtime.GOOS)
	})

	sys.Set("arch", func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(runtime.GOARCH)
	})

	sys.Set("cpus", func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(runtime.NumCPU())
	})

	sys.Set("totalmem", func(call goja.FunctionCall) goja.Value {
		mem, err := totalMemory()
		if err != nil {
			throwError(vm, "sys.totalmem: "+err.Error())
		}
		return vm.ToValue(mem)
	})

	sys.Set("freemem", func(call goja.FunctionCall) goja.Value {
		mem, err := freeMemory()
		if err != nil {
			throwError(vm, "sys.freemem: "+err.Error())
		}
		return vm.ToValue(mem)
	})

	sys.Set("uptime", func(call goja.FunctionCall) goja.Value {
		up, err := systemUptime()
		if err != nil {
			throwError(vm, "sys.uptime: "+err.Error())
		}
		return vm.ToValue(up)
	})

	sys.Set("loadavg", func(call goja.FunctionCall) goja.Value {
		avg, err := systemLoadavg()
		if err != nil {
			throwError(vm, "sys.loadavg: "+err.Error())
		}
		return vm.ToValue(avg)
	})

	sys.Set("terminal", func(call goja.FunctionCall) goja.Value {
		isTTY := term.IsTerminal(int(os.Stdout.Fd()))
		columns, rows := 80, 24
		if w, h, err := term.GetSize(int(os.Stderr.Fd())); err == nil {
			columns, rows = w, h
		}
		colorSupport := isTTY && os.Getenv("NO_COLOR") == ""
		return vm.ToValue(map[string]any{
			"columns": columns,
			"rows":    rows,
			"isTTY":   isTTY,
			"color":   colorSupport,
		})
	})

	vm.Set("sys", sys)
}
