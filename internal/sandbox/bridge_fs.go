package sandbox

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/dop251/goja"
)

const maxGlobMatches = 1000000

func (s *Sandbox) registerFS(vm *goja.Runtime) {
	fs := vm.NewObject()

	fs.Set("readFile", func(call goja.FunctionCall) goja.Value {
		path := call.Argument(0).String()
		resolved, err := s.resolvePath("read", path)
		if err != nil {
			throwError(vm, err.Error())
		}
		data, err := os.ReadFile(resolved)
		if err != nil {
			throwError(vm, fmt.Sprintf("fs.readFile: %s not found", path))
		}
		return vm.ToValue(string(data))
	})

	fs.Set("writeFile", func(call goja.FunctionCall) goja.Value {
		path := call.Argument(0).String()
		content := call.Argument(1).String()
		resolved, err := s.resolvePath("write", path)
		if err != nil {
			throwError(vm, err.Error())
		}
		if err := os.WriteFile(resolved, []byte(content), 0644); err != nil {
			throwError(vm, fmt.Sprintf("fs.writeFile: cannot write %s", path))
		}
		if s.cfg.OnWrite != nil {
			s.cfg.OnWrite(resolved, content)
		}
		return goja.Undefined()
	})

	fs.Set("readDir", func(call goja.FunctionCall) goja.Value {
		path := call.Argument(0).String()
		resolved, err := s.resolvePath("read", path)
		if err != nil {
			throwError(vm, err.Error())
		}
		entries, err := os.ReadDir(resolved)
		if err != nil {
			throwError(vm, fmt.Sprintf("fs.readDir: cannot read %s", path))
		}
		result := make([]map[string]any, 0, len(entries))
		for _, e := range entries {
			info, err := e.Info()
			if err != nil {
				continue
			}
			result = append(result, map[string]any{
				"name":  e.Name(),
				"isDir": e.IsDir(),
				"size":  info.Size(),
			})
		}
		return vm.ToValue(result)
	})

	fs.Set("stat", func(call goja.FunctionCall) goja.Value {
		path := call.Argument(0).String()
		resolved, err := s.resolvePath("read", path)
		if err != nil {
			throwError(vm, err.Error())
		}
		info, err := os.Stat(resolved)
		if err != nil {
			throwError(vm, fmt.Sprintf("fs.stat: %s not found", path))
		}
		return vm.ToValue(map[string]any{
			"name":    info.Name(),
			"isDir":   info.IsDir(),
			"size":    info.Size(),
			"modTime": info.ModTime().Unix(),
		})
	})

	fs.Set("delete", func(call goja.FunctionCall) goja.Value {
		path := call.Argument(0).String()
		resolved, err := s.resolvePath("delete", path)
		if err != nil {
			throwError(vm, err.Error())
		}
		// Block deletion of sandbox root directories themselves
		for _, allowed := range s.allowedPaths {
			if resolved == allowed {
				throwError(vm, fmt.Sprintf("fs.delete: cannot delete sandbox root %s", path))
			}
		}
		if err := os.RemoveAll(resolved); err != nil {
			throwError(vm, fmt.Sprintf("fs.delete: cannot delete %s", path))
		}
		return goja.Undefined()
	})

	fs.Set("exists", func(call goja.FunctionCall) goja.Value {
		path := call.Argument(0).String()
		resolved, err := s.resolvePath("read", path)
		if err != nil {
			return vm.ToValue(false)
		}
		_, err = os.Stat(resolved)
		return vm.ToValue(err == nil)
	})

	fs.Set("mkdir", func(call goja.FunctionCall) goja.Value {
		path := call.Argument(0).String()
		resolved, err := s.resolvePath("write", path)
		if err != nil {
			throwError(vm, err.Error())
		}
		if err := os.MkdirAll(resolved, 0755); err != nil {
			throwError(vm, fmt.Sprintf("fs.mkdir: cannot create %s", path))
		}
		return goja.Undefined()
	})

	fs.Set("copy", func(call goja.FunctionCall) goja.Value {
		src := call.Argument(0).String()
		dst := call.Argument(1).String()
		resolvedSrc, err := s.resolvePath("read", src)
		if err != nil {
			throwError(vm, err.Error())
		}
		resolvedDst, err := s.resolvePath("write", dst)
		if err != nil {
			throwError(vm, err.Error())
		}
		in, err := os.Open(resolvedSrc)
		if err != nil {
			throwError(vm, fmt.Sprintf("fs.copy: cannot read %s", src))
		}
		defer in.Close()
		out, err := os.Create(resolvedDst)
		if err != nil {
			throwError(vm, fmt.Sprintf("fs.copy: cannot write %s", dst))
		}
		defer out.Close()
		if _, err := io.Copy(out, in); err != nil {
			throwError(vm, fmt.Sprintf("fs.copy: failed copying %s to %s", src, dst))
		}
		if err := out.Sync(); err != nil {
			throwError(vm, fmt.Sprintf("fs.copy: failed syncing %s", dst))
		}
		return goja.Undefined()
	})

	fs.Set("move", func(call goja.FunctionCall) goja.Value {
		src := call.Argument(0).String()
		dst := call.Argument(1).String()
		resolvedSrc, err := s.resolvePath("delete", src)
		if err != nil {
			throwError(vm, err.Error())
		}
		resolvedDst, err := s.resolvePath("write", dst)
		if err != nil {
			throwError(vm, err.Error())
		}
		if err := os.Rename(resolvedSrc, resolvedDst); err != nil {
			throwError(vm, fmt.Sprintf("fs.move: cannot move %s to %s", src, dst))
		}
		return goja.Undefined()
	})

	fs.Set("appendFile", func(call goja.FunctionCall) goja.Value {
		path := call.Argument(0).String()
		content := call.Argument(1).String()
		resolved, err := s.resolvePath("write", path)
		if err != nil {
			throwError(vm, err.Error())
		}
		f, err := os.OpenFile(resolved, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			throwError(vm, fmt.Sprintf("fs.appendFile: cannot open %s", path))
		}
		defer f.Close()
		if _, err := f.WriteString(content); err != nil {
			throwError(vm, fmt.Sprintf("fs.appendFile: cannot write to %s", path))
		}
		return goja.Undefined()
	})

	fs.Set("glob", func(call goja.FunctionCall) goja.Value {
		pattern := call.Argument(0).String()

		// Resolve the base directory (everything before the first wildcard)
		base := pattern
		for i, c := range base {
			if c == '*' || c == '?' || c == '[' {
				base = base[:i]
				break
			}
		}
		base = filepath.Dir(base)
		if base == "" {
			base = "."
		}

		// Resolve the base through sandbox path checks
		resolvedBase, err := s.resolvePath("list", base)
		if err != nil {
			throwError(vm, err.Error())
		}

		// If pattern uses **, walk recursively; otherwise use filepath.Glob
		var matches []string
		if strings.Contains(pattern, "**") {
			// Replace base in pattern with resolved base for matching
			relPattern := strings.TrimPrefix(pattern, base)
			relPattern = strings.TrimPrefix(relPattern, string(filepath.Separator))

			filepath.WalkDir(resolvedBase, func(path string, d os.DirEntry, err error) error {
				if err != nil {
					return nil // skip errors
				}
				if len(matches) >= maxGlobMatches {
					return fmt.Errorf("glob match limit exceeded")
				}
				// Get path relative to resolved base for matching
				rel, err := filepath.Rel(resolvedBase, path)
				if err != nil {
					return nil
				}
				// Match against the ** pattern by trying each path segment depth
				if globMatch(relPattern, rel) {
					matches = append(matches, path)
				}
				return nil
			})
		} else {
			// Resolve absolute pattern for filepath.Glob
			absPattern := pattern
			if !filepath.IsAbs(absPattern) {
				absPattern = filepath.Join(s.cfg.WorkDir, absPattern)
			}
			raw, _ := filepath.Glob(absPattern)
			// Re-validate each match through sandbox path checks
			for _, m := range raw {
				if _, err := s.resolvePath("read", m); err == nil {
					matches = append(matches, m)
				}
				if len(matches) >= maxGlobMatches {
					break
				}
			}
		}

		if len(matches) >= maxGlobMatches {
			throwError(vm, fmt.Sprintf("fs.glob: pattern %q returned too many matches (limit %d)", pattern, maxGlobMatches))
		}

		return vm.ToValue(matches)
	})

	vm.Set("fs", fs)
}

// globMatch matches a path against a pattern supporting ** for recursive matching.
func globMatch(pattern, path string) bool {
	// Split pattern and path into segments
	patParts := strings.Split(filepath.ToSlash(pattern), "/")
	pathParts := strings.Split(filepath.ToSlash(path), "/")
	return globMatchParts(patParts, pathParts)
}

func globMatchParts(pat, path []string) bool {
	for len(pat) > 0 && len(path) > 0 {
		if pat[0] == "**" {
			// ** matches zero or more directories
			rest := pat[1:]
			for i := 0; i <= len(path); i++ {
				if globMatchParts(rest, path[i:]) {
					return true
				}
			}
			return false
		}
		matched, _ := filepath.Match(pat[0], path[0])
		if !matched {
			return false
		}
		pat = pat[1:]
		path = path[1:]
	}
	// Handle trailing **
	if len(pat) == 1 && pat[0] == "**" {
		return true
	}
	return len(pat) == 0 && len(path) == 0
}
