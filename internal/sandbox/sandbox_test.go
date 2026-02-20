package sandbox

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBasicExecution(t *testing.T) {
	sb, err := New(Config{})
	if err != nil {
		t.Fatalf("failed to create sandbox: %v", err)
	}

	result, err := sb.Run(context.Background(), `"hello world"`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "hello world" {
		t.Errorf("result = %q, want %q", result, "hello world")
	}
}

func TestHelloWorldConsoleLog(t *testing.T) {
	var stderr strings.Builder
	sb, err := New(Config{
		Stderr: &stderr,
	})
	if err != nil {
		t.Fatalf("failed to create sandbox: %v", err)
	}

	result, err := sb.Run(context.Background(), `console.log("hello world"); "done"`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "done" {
		t.Errorf("result = %q, want %q", result, "done")
	}
	if !strings.Contains(stderr.String(), "hello world") {
		t.Errorf("stderr = %q, want to contain %q", stderr.String(), "hello world")
	}
}

func TestAgentResumeNoContext(t *testing.T) {
	sb, err := New(Config{})
	if err != nil {
		t.Fatalf("failed to create sandbox: %v", err)
	}

	_, err = sb.Run(context.Background(), `agent.resume()`)
	if err == nil {
		t.Fatal("expected error from agent.resume()")
	}

	var resumeErr *ResumeError
	if !errors.As(err, &resumeErr) {
		t.Fatalf("expected ResumeError, got %T: %v", err, err)
	}
	if resumeErr.Context != "" {
		t.Errorf("context = %q, want empty", resumeErr.Context)
	}
}

func TestAgentResumeWithContext(t *testing.T) {
	sb, err := New(Config{})
	if err != nil {
		t.Fatalf("failed to create sandbox: %v", err)
	}

	_, err = sb.Run(context.Background(), `agent.resume("need help with API")`)
	if err == nil {
		t.Fatal("expected error from agent.resume()")
	}

	var resumeErr *ResumeError
	if !errors.As(err, &resumeErr) {
		t.Fatalf("expected ResumeError, got %T: %v", err, err)
	}
	if resumeErr.Context != "need help with API" {
		t.Errorf("context = %q, want %q", resumeErr.Context, "need help with API")
	}
}

func TestJavaScriptException(t *testing.T) {
	sb, err := New(Config{})
	if err != nil {
		t.Fatalf("failed to create sandbox: %v", err)
	}

	_, err = sb.Run(context.Background(), `throw new Error("something went wrong")`)
	if err == nil {
		t.Fatal("expected error from throw")
	}

	// Should NOT be a ResumeError
	var resumeErr *ResumeError
	if errors.As(err, &resumeErr) {
		t.Fatal("should not be a ResumeError")
	}

	if !strings.Contains(err.Error(), "something went wrong") {
		t.Errorf("error = %q, want to contain %q", err.Error(), "something went wrong")
	}
}

func TestReferenceError(t *testing.T) {
	sb, err := New(Config{})
	if err != nil {
		t.Fatalf("failed to create sandbox: %v", err)
	}

	_, err = sb.Run(context.Background(), `undefinedVariable`)
	if err == nil {
		t.Fatal("expected error from undefined variable")
	}

	// Should NOT be a ResumeError
	var resumeErr *ResumeError
	if errors.As(err, &resumeErr) {
		t.Fatal("should not be a ResumeError")
	}
}

func TestProcessExit(t *testing.T) {
	sb, err := New(Config{})
	if err != nil {
		t.Fatalf("failed to create sandbox: %v", err)
	}

	// Exit with code 0 should not be an error
	_, err = sb.Run(context.Background(), `process.exit(0)`)
	if err != nil {
		t.Errorf("process.exit(0) should not error, got: %v", err)
	}

	// Exit with non-zero code should be an error
	_, err = sb.Run(context.Background(), `process.exit(1)`)
	if err == nil {
		t.Error("process.exit(1) should return an error")
	}
}

func TestAgentResumeAfterWork(t *testing.T) {
	var stderr strings.Builder
	sb, err := New(Config{
		Stderr: &stderr,
	})
	if err != nil {
		t.Fatalf("failed to create sandbox: %v", err)
	}

	// Do some work, then call agent.resume
	code := `
		var x = 1 + 2;
		console.log("computed:", x);
		if (x === 3) {
			agent.resume("x is 3, need further instructions");
		}
		"should not reach here"
	`
	_, err = sb.Run(context.Background(), code)
	if err == nil {
		t.Fatal("expected error from agent.resume()")
	}

	var resumeErr *ResumeError
	if !errors.As(err, &resumeErr) {
		t.Fatalf("expected ResumeError, got %T: %v", err, err)
	}
	if resumeErr.Context != "x is 3, need further instructions" {
		t.Errorf("context = %q, want %q", resumeErr.Context, "x is 3, need further instructions")
	}

	// Verify the console.log happened before the resume
	if !strings.Contains(stderr.String(), "computed: 3") {
		t.Errorf("stderr = %q, want to contain %q", stderr.String(), "computed: 3")
	}
}

func TestResumeErrorMessage(t *testing.T) {
	err := &ResumeError{Context: "test context"}
	if err.Error() != "agent.resume: test context" {
		t.Errorf("error = %q, want %q", err.Error(), "agent.resume: test context")
	}

	err2 := &ResumeError{Context: ""}
	if err2.Error() != "agent.resume" {
		t.Errorf("error = %q, want %q", err2.Error(), "agent.resume")
	}
}

// Security tests

func TestWritablePathsExactFileMatch(t *testing.T) {
	dir := t.TempDir()
	// Resolve symlinks to match what sandbox does internally
	dir, _ = filepath.EvalSymlinks(dir)

	memoryJS := filepath.Join(dir, "memory.js")
	policyJSON := filepath.Join(dir, "policy.json")
	libDir := filepath.Join(dir, "lib")
	os.MkdirAll(libDir, 0700)

	// Create sandbox where memory.js is writable but thoughtDir is not
	sb, err := New(Config{
		AllowedPaths:  []string{dir},
		WritablePaths: []string{libDir, memoryJS}, // Note: dir is NOT in WritablePaths
		WorkDir:       dir,
	})
	if err != nil {
		t.Fatalf("failed to create sandbox: %v", err)
	}

	// Should be able to write to memory.js
	_, err = sb.Run(context.Background(), `fs.writeFile("`+memoryJS+`", "test")`)
	if err != nil {
		t.Errorf("should be able to write to memory.js: %v", err)
	}

	// Should be able to write to lib/
	libFile := filepath.Join(libDir, "test.js")
	_, err = sb.Run(context.Background(), `fs.writeFile("`+libFile+`", "test")`)
	if err != nil {
		t.Errorf("should be able to write to lib/: %v", err)
	}

	// Should NOT be able to write to policy.json (outside WritablePaths, no ApprovePath callback)
	_, err = sb.Run(context.Background(), `fs.writeFile("`+policyJSON+`", "malicious")`)
	if err == nil {
		t.Error("SECURITY BUG: should NOT be able to write to policy.json")
	}
}

func TestCannotWriteOutsideWritablePaths(t *testing.T) {
	dir := t.TempDir()
	libDir := filepath.Join(dir, "lib")
	os.MkdirAll(libDir, 0700)

	sb, err := New(Config{
		AllowedPaths:  []string{dir},
		WritablePaths: []string{libDir},
		WorkDir:       dir,
	})
	if err != nil {
		t.Fatalf("failed to create sandbox: %v", err)
	}

	// Try to write to a file in dir (readable but not writable)
	testFile := filepath.Join(dir, "test.txt")
	_, err = sb.Run(context.Background(), `fs.writeFile("`+testFile+`", "test")`)
	if err == nil {
		t.Error("should NOT be able to write outside WritablePaths")
	}
}

func TestCanReadOutsideWritablePaths(t *testing.T) {
	dir := t.TempDir()
	libDir := filepath.Join(dir, "lib")
	os.MkdirAll(libDir, 0700)

	// Create a readable file
	testFile := filepath.Join(dir, "readable.txt")
	os.WriteFile(testFile, []byte("hello"), 0644)

	sb, err := New(Config{
		AllowedPaths:  []string{dir},
		WritablePaths: []string{libDir},
		WorkDir:       dir,
	})
	if err != nil {
		t.Fatalf("failed to create sandbox: %v", err)
	}

	// Should be able to read from AllowedPaths
	result, err := sb.Run(context.Background(), `fs.readFile("`+testFile+`")`)
	if err != nil {
		t.Errorf("should be able to read from AllowedPaths: %v", err)
	}
	if result != "hello" {
		t.Errorf("result = %q, want %q", result, "hello")
	}
}

func TestApprovePathCallback(t *testing.T) {
	dir := t.TempDir()
	// Resolve symlinks to match what sandbox does internally
	dir, _ = filepath.EvalSymlinks(dir)
	libDir := filepath.Join(dir, "lib")
	outsideDir := t.TempDir()
	outsideDir, _ = filepath.EvalSymlinks(outsideDir)
	os.MkdirAll(libDir, 0700)

	approvePathCalled := false
	approvedPath := ""

	sb, err := New(Config{
		AllowedPaths:  []string{dir},
		WritablePaths: []string{libDir},
		WorkDir:       dir,
		ApprovePath: func(op, path string) (bool, error) {
			approvePathCalled = true
			approvedPath = path
			return false, nil // Deny
		},
	})
	if err != nil {
		t.Fatalf("failed to create sandbox: %v", err)
	}

	// Try to read from outside the sandbox
	outsideFile := filepath.Join(outsideDir, "outside.txt")
	os.WriteFile(outsideFile, []byte("secret"), 0644)

	_, err = sb.Run(context.Background(), `fs.readFile("`+outsideFile+`")`)
	if err == nil {
		t.Error("should NOT be able to read from outside without approval")
	}

	if !approvePathCalled {
		t.Error("ApprovePath callback should have been called")
	}

	// Verify the resolved path was passed (symlinks resolved)
	if approvedPath != outsideFile {
		t.Errorf("approved path = %q, want %q", approvedPath, outsideFile)
	}
}

// Filesystem bridge tests

func TestFsReadDir(t *testing.T) {
	dir := t.TempDir()
	dir, _ = filepath.EvalSymlinks(dir)

	// Create some files
	os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a"), 0644)
	os.WriteFile(filepath.Join(dir, "b.txt"), []byte("b"), 0644)
	os.MkdirAll(filepath.Join(dir, "subdir"), 0700)

	sb, err := New(Config{
		AllowedPaths: []string{dir},
		WorkDir:      dir,
	})
	if err != nil {
		t.Fatalf("failed to create sandbox: %v", err)
	}

	result, err := sb.Run(context.Background(), `JSON.stringify(fs.readDir("`+dir+`"))`)
	if err != nil {
		t.Fatalf("fs.readDir error: %v", err)
	}

	if !strings.Contains(result, "a.txt") {
		t.Errorf("result should contain a.txt: %s", result)
	}
	if !strings.Contains(result, "b.txt") {
		t.Errorf("result should contain b.txt: %s", result)
	}
	if !strings.Contains(result, "subdir") {
		t.Errorf("result should contain subdir: %s", result)
	}
}

func TestFsStat(t *testing.T) {
	dir := t.TempDir()
	dir, _ = filepath.EvalSymlinks(dir)

	testFile := filepath.Join(dir, "test.txt")
	os.WriteFile(testFile, []byte("hello"), 0644)

	sb, err := New(Config{
		AllowedPaths: []string{dir},
		WorkDir:      dir,
	})
	if err != nil {
		t.Fatalf("failed to create sandbox: %v", err)
	}

	result, err := sb.Run(context.Background(), `
		var stat = fs.stat("`+testFile+`");
		JSON.stringify({size: stat.size, isDir: stat.isDir})
	`)
	if err != nil {
		t.Fatalf("fs.stat error: %v", err)
	}

	if !strings.Contains(result, `"size":5`) {
		t.Errorf("size should be 5: %s", result)
	}
	if !strings.Contains(result, `"isDir":false`) {
		t.Errorf("isDir should be false: %s", result)
	}
}

func TestFsExists(t *testing.T) {
	dir := t.TempDir()
	dir, _ = filepath.EvalSymlinks(dir)

	testFile := filepath.Join(dir, "exists.txt")
	os.WriteFile(testFile, []byte("hello"), 0644)

	sb, err := New(Config{
		AllowedPaths: []string{dir},
		WorkDir:      dir,
	})
	if err != nil {
		t.Fatalf("failed to create sandbox: %v", err)
	}

	// File that exists
	result, err := sb.Run(context.Background(), `fs.exists("`+testFile+`")`)
	if err != nil {
		t.Fatalf("fs.exists error: %v", err)
	}
	if result != "true" {
		t.Errorf("fs.exists should return true for existing file: %s", result)
	}

	// File that doesn't exist
	result, err = sb.Run(context.Background(), `fs.exists("`+filepath.Join(dir, "nonexistent.txt")+`")`)
	if err != nil {
		t.Fatalf("fs.exists error: %v", err)
	}
	if result != "false" {
		t.Errorf("fs.exists should return false for non-existent file: %s", result)
	}
}

func TestFsGlob(t *testing.T) {
	dir := t.TempDir()
	dir, _ = filepath.EvalSymlinks(dir)

	// Create some files
	os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a"), 0644)
	os.WriteFile(filepath.Join(dir, "b.txt"), []byte("b"), 0644)
	os.WriteFile(filepath.Join(dir, "c.md"), []byte("c"), 0644)

	sb, err := New(Config{
		AllowedPaths: []string{dir},
		WorkDir:      dir,
	})
	if err != nil {
		t.Fatalf("failed to create sandbox: %v", err)
	}

	result, err := sb.Run(context.Background(), `JSON.stringify(fs.glob("`+dir+`/*.txt"))`)
	if err != nil {
		t.Fatalf("fs.glob error: %v", err)
	}

	if !strings.Contains(result, "a.txt") {
		t.Errorf("result should contain a.txt: %s", result)
	}
	if !strings.Contains(result, "b.txt") {
		t.Errorf("result should contain b.txt: %s", result)
	}
	if strings.Contains(result, "c.md") {
		t.Errorf("result should NOT contain c.md: %s", result)
	}
}

func TestFsAppendFile(t *testing.T) {
	dir := t.TempDir()
	dir, _ = filepath.EvalSymlinks(dir)

	testFile := filepath.Join(dir, "append.txt")
	os.WriteFile(testFile, []byte("hello"), 0644)

	sb, err := New(Config{
		AllowedPaths:  []string{dir},
		WritablePaths: []string{dir},
		WorkDir:       dir,
	})
	if err != nil {
		t.Fatalf("failed to create sandbox: %v", err)
	}

	_, err = sb.Run(context.Background(), `fs.appendFile("`+testFile+`", " world")`)
	if err != nil {
		t.Fatalf("fs.appendFile error: %v", err)
	}

	content, _ := os.ReadFile(testFile)
	if string(content) != "hello world" {
		t.Errorf("content = %q, want %q", string(content), "hello world")
	}
}

func TestFsDelete(t *testing.T) {
	dir := t.TempDir()
	dir, _ = filepath.EvalSymlinks(dir)

	testFile := filepath.Join(dir, "delete.txt")
	os.WriteFile(testFile, []byte("hello"), 0644)

	sb, err := New(Config{
		AllowedPaths:  []string{dir},
		WritablePaths: []string{dir},
		WorkDir:       dir,
	})
	if err != nil {
		t.Fatalf("failed to create sandbox: %v", err)
	}

	_, err = sb.Run(context.Background(), `fs.delete("`+testFile+`")`)
	if err != nil {
		t.Fatalf("fs.delete error: %v", err)
	}

	if _, err := os.Stat(testFile); !os.IsNotExist(err) {
		t.Error("file should have been deleted")
	}
}

func TestFsMkdir(t *testing.T) {
	dir := t.TempDir()
	dir, _ = filepath.EvalSymlinks(dir)

	newDir := filepath.Join(dir, "newdir")

	sb, err := New(Config{
		AllowedPaths:  []string{dir},
		WritablePaths: []string{dir},
		WorkDir:       dir,
	})
	if err != nil {
		t.Fatalf("failed to create sandbox: %v", err)
	}

	_, err = sb.Run(context.Background(), `fs.mkdir("`+newDir+`")`)
	if err != nil {
		t.Fatalf("fs.mkdir error: %v", err)
	}

	info, err := os.Stat(newDir)
	if err != nil {
		t.Fatalf("directory should exist: %v", err)
	}
	if !info.IsDir() {
		t.Error("should be a directory")
	}
}

func TestFsCopy(t *testing.T) {
	dir := t.TempDir()
	dir, _ = filepath.EvalSymlinks(dir)

	srcFile := filepath.Join(dir, "src.txt")
	dstFile := filepath.Join(dir, "dst.txt")
	os.WriteFile(srcFile, []byte("copy me"), 0644)

	sb, err := New(Config{
		AllowedPaths:  []string{dir},
		WritablePaths: []string{dir},
		WorkDir:       dir,
	})
	if err != nil {
		t.Fatalf("failed to create sandbox: %v", err)
	}

	_, err = sb.Run(context.Background(), `fs.copy("`+srcFile+`", "`+dstFile+`")`)
	if err != nil {
		t.Fatalf("fs.copy error: %v", err)
	}

	content, _ := os.ReadFile(dstFile)
	if string(content) != "copy me" {
		t.Errorf("content = %q, want %q", string(content), "copy me")
	}
}

func TestFsMove(t *testing.T) {
	dir := t.TempDir()
	dir, _ = filepath.EvalSymlinks(dir)

	srcFile := filepath.Join(dir, "move-src.txt")
	dstFile := filepath.Join(dir, "move-dst.txt")
	os.WriteFile(srcFile, []byte("move me"), 0644)

	sb, err := New(Config{
		AllowedPaths:  []string{dir},
		WritablePaths: []string{dir},
		WorkDir:       dir,
	})
	if err != nil {
		t.Fatalf("failed to create sandbox: %v", err)
	}

	_, err = sb.Run(context.Background(), `fs.move("`+srcFile+`", "`+dstFile+`")`)
	if err != nil {
		t.Fatalf("fs.move error: %v", err)
	}

	// Source should be gone
	if _, err := os.Stat(srcFile); !os.IsNotExist(err) {
		t.Error("source file should be gone after move")
	}

	// Destination should have content
	content, _ := os.ReadFile(dstFile)
	if string(content) != "move me" {
		t.Errorf("content = %q, want %q", string(content), "move me")
	}
}

// Environment bridge tests

func TestEnvApprovalRequired(t *testing.T) {
	approveEnvCalled := false
	requestedEnvVar := ""

	sb, err := New(Config{
		ApproveEnv: func(name string) (bool, error) {
			approveEnvCalled = true
			requestedEnvVar = name
			return false, nil // Deny
		},
	})
	if err != nil {
		t.Fatalf("failed to create sandbox: %v", err)
	}

	_, err = sb.Run(context.Background(), `env.get("HOME")`)
	if err == nil {
		t.Error("should error when env access is denied")
	}

	if !approveEnvCalled {
		t.Error("ApproveEnv callback should have been called")
	}
	if requestedEnvVar != "HOME" {
		t.Errorf("requestedEnvVar = %q, want %q", requestedEnvVar, "HOME")
	}
}

func TestEnvApproved(t *testing.T) {
	// Set a test env var
	os.Setenv("TEST_SANDBOX_VAR", "test-value")
	defer os.Unsetenv("TEST_SANDBOX_VAR")

	sb, err := New(Config{
		ApproveEnv: func(name string) (bool, error) {
			return name == "TEST_SANDBOX_VAR", nil // Only approve our test var
		},
	})
	if err != nil {
		t.Fatalf("failed to create sandbox: %v", err)
	}

	result, err := sb.Run(context.Background(), `env.get("TEST_SANDBOX_VAR")`)
	if err != nil {
		t.Fatalf("env.get error: %v", err)
	}

	if result != "test-value" {
		t.Errorf("result = %q, want %q", result, "test-value")
	}
}

// Process bridge tests

func TestProcessCwd(t *testing.T) {
	dir := t.TempDir()
	dir, _ = filepath.EvalSymlinks(dir)

	sb, err := New(Config{
		WorkDir: dir,
	})
	if err != nil {
		t.Fatalf("failed to create sandbox: %v", err)
	}

	result, err := sb.Run(context.Background(), `process.cwd()`)
	if err != nil {
		t.Fatalf("process.cwd error: %v", err)
	}

	if result != dir {
		t.Errorf("cwd = %q, want %q", result, dir)
	}
}

func TestProcessArgs(t *testing.T) {
	sb, err := New(Config{
		Args: []string{"arg1", "arg2", "arg3"},
	})
	if err != nil {
		t.Fatalf("failed to create sandbox: %v", err)
	}

	result, err := sb.Run(context.Background(), `JSON.stringify(process.args)`)
	if err != nil {
		t.Fatalf("process.args error: %v", err)
	}

	if result != `["arg1","arg2","arg3"]` {
		t.Errorf("args = %s, want %s", result, `["arg1","arg2","arg3"]`)
	}
}

func TestProcessExitZero(t *testing.T) {
	sb, err := New(Config{})
	if err != nil {
		t.Fatalf("failed to create sandbox: %v", err)
	}

	_, err = sb.Run(context.Background(), `process.exit(0)`)
	if err != nil {
		t.Errorf("process.exit(0) should not error: %v", err)
	}
}

func TestProcessExitNonZero(t *testing.T) {
	sb, err := New(Config{})
	if err != nil {
		t.Fatalf("failed to create sandbox: %v", err)
	}

	_, err = sb.Run(context.Background(), `process.exit(42)`)
	if err == nil {
		t.Error("process.exit(42) should return an error")
	}
	if !strings.Contains(err.Error(), "42") {
		t.Errorf("error should contain exit code: %v", err)
	}
}

// System bridge tests

func TestSysPlatform(t *testing.T) {
	sb, err := New(Config{})
	if err != nil {
		t.Fatalf("failed to create sandbox: %v", err)
	}

	result, err := sb.Run(context.Background(), `sys.platform()`)
	if err != nil {
		t.Fatalf("sys.platform error: %v", err)
	}

	// Should be one of the known platforms
	validPlatforms := []string{"darwin", "linux", "windows"}
	found := false
	for _, p := range validPlatforms {
		if result == p {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("unexpected platform: %s", result)
	}
}

func TestSysArch(t *testing.T) {
	sb, err := New(Config{})
	if err != nil {
		t.Fatalf("failed to create sandbox: %v", err)
	}

	result, err := sb.Run(context.Background(), `sys.arch()`)
	if err != nil {
		t.Fatalf("sys.arch error: %v", err)
	}

	// Should be one of the known architectures
	validArchs := []string{"amd64", "arm64", "386", "arm"}
	found := false
	for _, a := range validArchs {
		if result == a {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("unexpected arch: %s", result)
	}
}

func TestSysCpus(t *testing.T) {
	sb, err := New(Config{})
	if err != nil {
		t.Fatalf("failed to create sandbox: %v", err)
	}

	result, err := sb.Run(context.Background(), `sys.cpus()`)
	if err != nil {
		t.Fatalf("sys.cpus error: %v", err)
	}

	// Should be a positive number
	var cpus int
	if _, err := strings.NewReader(result).Read([]byte{}); err != nil {
		// Try to parse as number
	}
	if result == "0" || result == "" {
		t.Errorf("cpus should be positive: %s", result)
	}
	_ = cpus
}

func TestSysTotalmem(t *testing.T) {
	sb, err := New(Config{})
	if err != nil {
		t.Fatalf("failed to create sandbox: %v", err)
	}

	result, err := sb.Run(context.Background(), `sys.totalmem()`)
	if err != nil {
		t.Fatalf("sys.totalmem error: %v", err)
	}

	if result == "0" || result == "" {
		t.Errorf("totalmem should be positive: %s", result)
	}
}

// Console bridge tests

func TestConsoleLog(t *testing.T) {
	var stderr strings.Builder
	sb, err := New(Config{
		Stderr: &stderr,
	})
	if err != nil {
		t.Fatalf("failed to create sandbox: %v", err)
	}

	_, err = sb.Run(context.Background(), `console.log("test message"); "done"`)
	if err != nil {
		t.Fatalf("console.log error: %v", err)
	}

	if !strings.Contains(stderr.String(), "test message") {
		t.Errorf("stderr should contain 'test message': %q", stderr.String())
	}
}

func TestConsoleError(t *testing.T) {
	var stderr strings.Builder
	sb, err := New(Config{
		Stderr: &stderr,
	})
	if err != nil {
		t.Fatalf("failed to create sandbox: %v", err)
	}

	_, err = sb.Run(context.Background(), `console.error("error message"); "done"`)
	if err != nil {
		t.Fatalf("console.error error: %v", err)
	}

	if !strings.Contains(stderr.String(), "error message") {
		t.Errorf("stderr should contain 'error message': %q", stderr.String())
	}
}

func TestConsoleLogObject(t *testing.T) {
	var stderr strings.Builder
	sb, err := New(Config{
		Stderr: &stderr,
	})
	if err != nil {
		t.Fatalf("failed to create sandbox: %v", err)
	}

	// Objects should be JSON stringified
	_, err = sb.Run(context.Background(), `console.log({foo: "bar", num: 42}); "done"`)
	if err != nil {
		t.Fatalf("console.log error: %v", err)
	}

	output := stderr.String()
	if !strings.Contains(output, "foo") || !strings.Contains(output, "bar") {
		t.Errorf("stderr should contain JSON object: %q", output)
	}
}

// Security: Path traversal tests

func TestPathTraversalBlocked(t *testing.T) {
	dir := t.TempDir()
	dir, _ = filepath.EvalSymlinks(dir)

	// Create a subdirectory that's allowed
	allowed := filepath.Join(dir, "allowed")
	os.MkdirAll(allowed, 0700)

	// Create a file outside the allowed directory
	secret := filepath.Join(dir, "secret.txt")
	os.WriteFile(secret, []byte("secret data"), 0644)

	sb, err := New(Config{
		AllowedPaths: []string{allowed},
		WorkDir:      allowed,
	})
	if err != nil {
		t.Fatalf("failed to create sandbox: %v", err)
	}

	// Try to read using path traversal
	_, err = sb.Run(context.Background(), `fs.readFile("`+allowed+`/../secret.txt")`)
	if err == nil {
		t.Error("SECURITY BUG: path traversal should be blocked")
	}
}

func TestSymlinkEscape(t *testing.T) {
	dir := t.TempDir()
	dir, _ = filepath.EvalSymlinks(dir)

	// Create allowed and forbidden directories
	allowed := filepath.Join(dir, "allowed")
	forbidden := filepath.Join(dir, "forbidden")
	os.MkdirAll(allowed, 0700)
	os.MkdirAll(forbidden, 0700)

	// Create a secret file in forbidden
	secret := filepath.Join(forbidden, "secret.txt")
	os.WriteFile(secret, []byte("secret"), 0644)

	// Create a symlink in allowed pointing to forbidden
	symlink := filepath.Join(allowed, "escape")
	os.Symlink(forbidden, symlink)

	sb, err := New(Config{
		AllowedPaths: []string{allowed},
		WorkDir:      allowed,
	})
	if err != nil {
		t.Fatalf("failed to create sandbox: %v", err)
	}

	// Try to read through the symlink
	_, err = sb.Run(context.Background(), `fs.readFile("`+symlink+`/secret.txt")`)
	if err == nil {
		t.Error("SECURITY BUG: symlink escape should be blocked")
	}
}

// Network bridge tests (approval only, no actual network)

func TestNetApprovalRequired(t *testing.T) {
	approveNetCalled := false
	requestedHost := ""

	sb, err := New(Config{
		ApproveNet: func(host string) (bool, error) {
			approveNetCalled = true
			requestedHost = host
			return false, nil // Deny
		},
	})
	if err != nil {
		t.Fatalf("failed to create sandbox: %v", err)
	}

	_, err = sb.Run(context.Background(), `net.fetch("https://example.com/api")`)
	if err == nil {
		t.Error("should error when network access is denied")
	}

	if !approveNetCalled {
		t.Error("ApproveNet callback should have been called")
	}
	if requestedHost != "example.com" {
		t.Errorf("requestedHost = %q, want %q", requestedHost, "example.com")
	}
}

// CommonJS require() tests

func TestRequireLocalModule(t *testing.T) {
	dir := t.TempDir()
	dir, _ = filepath.EvalSymlinks(dir)

	// Create a module
	moduleDir := filepath.Join(dir, "lib")
	os.MkdirAll(moduleDir, 0700)
	modulePath := filepath.Join(moduleDir, "helper.js")
	os.WriteFile(modulePath, []byte(`module.exports = { add: function(a, b) { return a + b; } };`), 0644)

	sb, err := New(Config{
		AllowedPaths: []string{dir},
		WorkDir:      dir,
	})
	if err != nil {
		t.Fatalf("failed to create sandbox: %v", err)
	}

	result, err := sb.Run(context.Background(), `
		var helper = require("`+modulePath+`");
		helper.add(2, 3)
	`)
	if err != nil {
		t.Fatalf("require error: %v", err)
	}

	if result != "5" {
		t.Errorf("result = %q, want %q", result, "5")
	}
}

// OnWrite callback test

func TestOnWriteCallback(t *testing.T) {
	dir := t.TempDir()
	dir, _ = filepath.EvalSymlinks(dir)

	writtenPath := ""
	writtenContent := ""

	sb, err := New(Config{
		AllowedPaths:  []string{dir},
		WritablePaths: []string{dir},
		WorkDir:       dir,
		OnWrite: func(path, content string) {
			writtenPath = path
			writtenContent = content
		},
	})
	if err != nil {
		t.Fatalf("failed to create sandbox: %v", err)
	}

	testFile := filepath.Join(dir, "callback-test.txt")
	_, err = sb.Run(context.Background(), `fs.writeFile("`+testFile+`", "callback content")`)
	if err != nil {
		t.Fatalf("fs.writeFile error: %v", err)
	}

	if writtenPath != testFile {
		t.Errorf("writtenPath = %q, want %q", writtenPath, testFile)
	}
	if writtenContent != "callback content" {
		t.Errorf("writtenContent = %q, want %q", writtenContent, "callback content")
	}
}

// Resource limit tests

func TestDefaultTimeout(t *testing.T) {
	sb, err := New(Config{})
	if err != nil {
		t.Fatalf("failed to create sandbox: %v", err)
	}

	// Should have default timeout applied
	if sb.cfg.Timeout != DefaultTimeout {
		t.Errorf("timeout = %v, want %v", sb.cfg.Timeout, DefaultTimeout)
	}
}

func TestWriteSizeLimitCheck(t *testing.T) {
	// This test verifies that the size check exists by checking the constant
	// Full integration test with 10MB+ data is impractical due to timeout
	if MaxWriteSize != 10<<20 {
		t.Errorf("MaxWriteSize = %d, want %d (10MB)", MaxWriteSize, 10<<20)
	}
	if MaxReadSize != 50<<20 {
		t.Errorf("MaxReadSize = %d, want %d (50MB)", MaxReadSize, 50<<20)
	}
	if MaxCopySize != 50<<20 {
		t.Errorf("MaxCopySize = %d, want %d (50MB)", MaxCopySize, 50<<20)
	}
}

func TestSSRFProtectionLocalhost(t *testing.T) {
	sb, err := New(Config{
		ApproveNet: func(host string) (bool, error) {
			return true, nil // Would approve, but SSRF check should block first
		},
	})
	if err != nil {
		t.Fatalf("failed to create sandbox: %v", err)
	}

	// Try to access localhost
	_, err = sb.Run(context.Background(), `net.fetch("http://127.0.0.1:8080/secret")`)
	if err == nil {
		t.Error("SECURITY BUG: should block access to localhost")
	}
	if !strings.Contains(err.Error(), "private IP") {
		t.Errorf("error should mention private IP: %v", err)
	}
}

func TestSSRFProtectionPrivateIP(t *testing.T) {
	sb, err := New(Config{
		ApproveNet: func(host string) (bool, error) {
			return true, nil
		},
	})
	if err != nil {
		t.Fatalf("failed to create sandbox: %v", err)
	}

	privateIPs := []string{
		"http://10.0.0.1/",
		"http://172.16.0.1/",
		"http://192.168.1.1/",
	}

	for _, url := range privateIPs {
		_, err = sb.Run(context.Background(), `net.fetch("`+url+`")`)
		if err == nil {
			t.Errorf("SECURITY BUG: should block access to %s", url)
		}
	}
}
