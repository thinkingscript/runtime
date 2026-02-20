package boot

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNoMemoryJS(t *testing.T) {
	dir := t.TempDir()

	cfg := Config{
		MemoryJSPath: filepath.Join(dir, "memory.js"),
		WorkDir:      dir,
		ThoughtDir:   dir,
		LibDir:       filepath.Join(dir, "lib"),
		TmpDir:       filepath.Join(dir, "tmp"),
		MemoriesDir:  filepath.Join(dir, "memories"),
	}

	result := TryMemoryJS(context.Background(), cfg)

	if result.Success {
		t.Error("expected Success=false when memory.js doesn't exist")
	}
	if result.ResumeContext != "no memory.js exists, first run" {
		t.Errorf("ResumeContext = %q, want %q", result.ResumeContext, "no memory.js exists, first run")
	}
}

func TestMemoryJSSuccess(t *testing.T) {
	dir := t.TempDir()
	memoryJSPath := filepath.Join(dir, "memory.js")

	// Create a simple memory.js that returns "hello world"
	err := os.WriteFile(memoryJSPath, []byte(`"hello world"`), 0644)
	if err != nil {
		t.Fatalf("failed to create memory.js: %v", err)
	}

	cfg := Config{
		MemoryJSPath: memoryJSPath,
		WorkDir:      dir,
		ThoughtDir:   dir,
		LibDir:       filepath.Join(dir, "lib"),
		TmpDir:       filepath.Join(dir, "tmp"),
		MemoriesDir:  filepath.Join(dir, "memories"),
	}

	result := TryMemoryJS(context.Background(), cfg)

	if !result.Success {
		t.Errorf("expected Success=true, got ResumeContext=%q", result.ResumeContext)
	}
	if result.Output != "hello world" {
		t.Errorf("Output = %q, want %q", result.Output, "hello world")
	}
}

func TestMemoryJSWithProcessStdout(t *testing.T) {
	dir := t.TempDir()
	memoryJSPath := filepath.Join(dir, "memory.js")

	// Create memory.js that writes to stdout and returns
	code := `
		process.stdout.write("output from memory.js");
		"done"
	`
	err := os.WriteFile(memoryJSPath, []byte(code), 0644)
	if err != nil {
		t.Fatalf("failed to create memory.js: %v", err)
	}

	cfg := Config{
		MemoryJSPath: memoryJSPath,
		WorkDir:      dir,
		ThoughtDir:   dir,
		LibDir:       filepath.Join(dir, "lib"),
		TmpDir:       filepath.Join(dir, "tmp"),
		MemoriesDir:  filepath.Join(dir, "memories"),
	}

	result := TryMemoryJS(context.Background(), cfg)

	if !result.Success {
		t.Errorf("expected Success=true, got ResumeContext=%q", result.ResumeContext)
	}
	if result.Output != "done" {
		t.Errorf("Output = %q, want %q", result.Output, "done")
	}
}

func TestMemoryJSException(t *testing.T) {
	dir := t.TempDir()
	memoryJSPath := filepath.Join(dir, "memory.js")

	// Create memory.js that throws an error
	err := os.WriteFile(memoryJSPath, []byte(`throw new Error("something broke")`), 0644)
	if err != nil {
		t.Fatalf("failed to create memory.js: %v", err)
	}

	cfg := Config{
		MemoryJSPath: memoryJSPath,
		WorkDir:      dir,
		ThoughtDir:   dir,
		LibDir:       filepath.Join(dir, "lib"),
		TmpDir:       filepath.Join(dir, "tmp"),
		MemoriesDir:  filepath.Join(dir, "memories"),
	}

	result := TryMemoryJS(context.Background(), cfg)

	if result.Success {
		t.Error("expected Success=false when memory.js throws")
	}
	if !strings.Contains(result.ResumeContext, "memory.js error:") {
		t.Errorf("ResumeContext = %q, want to contain %q", result.ResumeContext, "memory.js error:")
	}
	if !strings.Contains(result.ResumeContext, "something broke") {
		t.Errorf("ResumeContext = %q, want to contain %q", result.ResumeContext, "something broke")
	}
}

func TestMemoryJSReferenceError(t *testing.T) {
	dir := t.TempDir()
	memoryJSPath := filepath.Join(dir, "memory.js")

	// Create memory.js with undefined variable
	err := os.WriteFile(memoryJSPath, []byte(`undefinedVariable`), 0644)
	if err != nil {
		t.Fatalf("failed to create memory.js: %v", err)
	}

	cfg := Config{
		MemoryJSPath: memoryJSPath,
		WorkDir:      dir,
		ThoughtDir:   dir,
		LibDir:       filepath.Join(dir, "lib"),
		TmpDir:       filepath.Join(dir, "tmp"),
		MemoriesDir:  filepath.Join(dir, "memories"),
	}

	result := TryMemoryJS(context.Background(), cfg)

	if result.Success {
		t.Error("expected Success=false when memory.js has ReferenceError")
	}
	if !strings.Contains(result.ResumeContext, "memory.js error:") {
		t.Errorf("ResumeContext = %q, want to contain %q", result.ResumeContext, "memory.js error:")
	}
}

func TestMemoryJSAgentResume(t *testing.T) {
	dir := t.TempDir()
	memoryJSPath := filepath.Join(dir, "memory.js")

	// Create memory.js that calls agent.resume()
	err := os.WriteFile(memoryJSPath, []byte(`agent.resume("need help with API")`), 0644)
	if err != nil {
		t.Fatalf("failed to create memory.js: %v", err)
	}

	cfg := Config{
		MemoryJSPath: memoryJSPath,
		WorkDir:      dir,
		ThoughtDir:   dir,
		LibDir:       filepath.Join(dir, "lib"),
		TmpDir:       filepath.Join(dir, "tmp"),
		MemoriesDir:  filepath.Join(dir, "memories"),
	}

	result := TryMemoryJS(context.Background(), cfg)

	if result.Success {
		t.Error("expected Success=false when memory.js calls agent.resume()")
	}
	if result.ResumeContext != "need help with API" {
		t.Errorf("ResumeContext = %q, want %q", result.ResumeContext, "need help with API")
	}
}

func TestMemoryJSAgentResumeAfterWork(t *testing.T) {
	dir := t.TempDir()
	memoryJSPath := filepath.Join(dir, "memory.js")

	// Create memory.js that does work then resumes
	code := `
		var x = 1 + 2;
		if (x === 3) {
			agent.resume("computed " + x + ", now what?");
		}
		"unreachable"
	`
	err := os.WriteFile(memoryJSPath, []byte(code), 0644)
	if err != nil {
		t.Fatalf("failed to create memory.js: %v", err)
	}

	cfg := Config{
		MemoryJSPath: memoryJSPath,
		WorkDir:      dir,
		ThoughtDir:   dir,
		LibDir:       filepath.Join(dir, "lib"),
		TmpDir:       filepath.Join(dir, "tmp"),
		MemoriesDir:  filepath.Join(dir, "memories"),
	}

	result := TryMemoryJS(context.Background(), cfg)

	if result.Success {
		t.Error("expected Success=false when memory.js calls agent.resume()")
	}
	if result.ResumeContext != "computed 3, now what?" {
		t.Errorf("ResumeContext = %q, want %q", result.ResumeContext, "computed 3, now what?")
	}
}

func TestMemoryJSProcessArgs(t *testing.T) {
	dir := t.TempDir()
	memoryJSPath := filepath.Join(dir, "memory.js")

	// Create memory.js that uses process.args
	code := `process.args.join(",")`
	err := os.WriteFile(memoryJSPath, []byte(code), 0644)
	if err != nil {
		t.Fatalf("failed to create memory.js: %v", err)
	}

	cfg := Config{
		MemoryJSPath: memoryJSPath,
		WorkDir:      dir,
		ThoughtDir:   dir,
		LibDir:       filepath.Join(dir, "lib"),
		TmpDir:       filepath.Join(dir, "tmp"),
		MemoriesDir:  filepath.Join(dir, "memories"),
		Args:         []string{"arg1", "arg2", "arg3"},
	}

	result := TryMemoryJS(context.Background(), cfg)

	if !result.Success {
		t.Errorf("expected Success=true, got ResumeContext=%q", result.ResumeContext)
	}
	if result.Output != "arg1,arg2,arg3" {
		t.Errorf("Output = %q, want %q", result.Output, "arg1,arg2,arg3")
	}
}

func TestMemoryJSFileAccess(t *testing.T) {
	dir := t.TempDir()
	libDir := filepath.Join(dir, "lib")
	os.MkdirAll(libDir, 0700)

	memoryJSPath := filepath.Join(dir, "memory.js")

	// Create a file in lib
	testFile := filepath.Join(libDir, "data.txt")
	err := os.WriteFile(testFile, []byte("test data"), 0644)
	if err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// Create memory.js that reads the file
	code := `fs.readFile("` + testFile + `")`
	err = os.WriteFile(memoryJSPath, []byte(code), 0644)
	if err != nil {
		t.Fatalf("failed to create memory.js: %v", err)
	}

	cfg := Config{
		MemoryJSPath: memoryJSPath,
		WorkDir:      dir,
		ThoughtDir:   dir,
		LibDir:       libDir,
		TmpDir:       filepath.Join(dir, "tmp"),
		MemoriesDir:  filepath.Join(dir, "memories"),
	}

	result := TryMemoryJS(context.Background(), cfg)

	if !result.Success {
		t.Errorf("expected Success=true, got ResumeContext=%q", result.ResumeContext)
	}
	if result.Output != "test data" {
		t.Errorf("Output = %q, want %q", result.Output, "test data")
	}
}
