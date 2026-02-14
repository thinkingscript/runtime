package sandbox

import (
	"fmt"
	"strings"

	"github.com/dop251/goja"
)

func (s *Sandbox) registerConsole(vm *goja.Runtime) {
	console := vm.NewObject()

	console.Set("log", func(call goja.FunctionCall) goja.Value {
		fmt.Fprintln(s.cfg.Stderr, formatArgs(vm, call.Arguments))
		return goja.Undefined()
	})

	console.Set("error", func(call goja.FunctionCall) goja.Value {
		fmt.Fprintln(s.cfg.Stderr, formatArgs(vm, call.Arguments))
		return goja.Undefined()
	})

	vm.Set("console", console)
}

func formatArgs(vm *goja.Runtime, args []goja.Value) string {
	parts := make([]string, len(args))
	for i, a := range args {
		parts[i] = stringify(vm, a)
	}
	return strings.Join(parts, " ")
}

// stringify converts a goja value to a readable string. Objects are
// JSON-serialized so the LLM (and user) can see actual data instead of
// "[object Object]".
func stringify(vm *goja.Runtime, v goja.Value) string {
	if v == nil || goja.IsUndefined(v) || goja.IsNull(v) {
		return v.String()
	}

	// Primitives: use String() directly
	switch v.ExportType().Kind().String() {
	case "map", "slice":
		// It's an object or array — JSON.stringify it
		jsonStringify := vm.Get("JSON").ToObject(vm).Get("stringify")
		fn, ok := goja.AssertFunction(jsonStringify)
		if ok {
			result, err := fn(goja.Undefined(), v)
			if err == nil && result != nil {
				return result.String()
			}
		}
	}

	return v.String()
}
