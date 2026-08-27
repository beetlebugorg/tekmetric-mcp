package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestBinaryName(t *testing.T) {
	got := binaryName("arm64")

	want := fmt.Sprintf("tekmetric-mcp-%s-arm64", runtime.GOOS)
	if runtime.GOOS == "windows" {
		want += ".exe"
	}

	if got != want {
		t.Errorf("binaryName(arm64) = %q, want %q", got, want)
	}
}

func TestNativeArchIsASupportedBuild(t *testing.T) {
	// Every answer has to name a build the release carries. The Windows path
	// falls back to amd64, so an unknown machine never yields an unknown name.
	switch got := nativeArch(); got {
	case "amd64", "arm64":
	default:
		t.Errorf("nativeArch() = %q, want amd64 or arm64", got)
	}
}

// buildLauncher compiles the launcher into dir and returns its path.
func buildLauncher(t *testing.T, dir string) string {
	t.Helper()

	path := filepath.Join(dir, "launcher")
	if runtime.GOOS == "windows" {
		path += ".exe"
	}

	out, err := exec.Command("go", "build", "-o", path, ".").CombinedOutput()
	if err != nil {
		t.Fatalf("go build error = %v\n%s", err, out)
	}
	return path
}

// writeStubTarget puts a stub where the launcher looks for the real build.
func writeStubTarget(t *testing.T, dir, script string) {
	t.Helper()

	if runtime.GOOS == "windows" {
		t.Skip("the stub needs a shell, which this test does not assume on Windows")
	}

	path := filepath.Join(dir, binaryName(nativeArch()))
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+script+"\n"), 0o755); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
}

func TestLauncherRunsTheBuildForThisMachine(t *testing.T) {
	dir := t.TempDir()
	launcher := buildLauncher(t, dir)
	writeStubTarget(t, dir, `echo "target ran with: $*"`)

	out, err := exec.Command(launcher, "serve", "--debug").CombinedOutput()
	if err != nil {
		t.Fatalf("launcher error = %v\n%s", err, out)
	}

	if got := strings.TrimSpace(string(out)); got != "target ran with: serve --debug" {
		t.Errorf("output = %q, want the arguments forwarded", got)
	}
}

func TestLauncherReportsTheChildStatus(t *testing.T) {
	dir := t.TempDir()
	launcher := buildLauncher(t, dir)
	writeStubTarget(t, dir, "exit 42")

	err := exec.Command(launcher).Run()

	var exit *exec.ExitError
	if !errorAs(err, &exit) {
		t.Fatalf("error = %v, want an exit status", err)
	}
	if exit.ExitCode() != 42 {
		t.Errorf("exit code = %d, want 42", exit.ExitCode())
	}
}

func TestLauncherReportsAMissingBuild(t *testing.T) {
	dir := t.TempDir()
	launcher := buildLauncher(t, dir)
	// No stub, so the build the launcher wants is absent.

	out, err := exec.Command(launcher).CombinedOutput()
	if err == nil {
		t.Fatal("launcher returned nil, want an error")
	}

	if !strings.Contains(string(out), "is missing") {
		t.Errorf("output = %q, want it to say the build is missing", out)
	}
	if !strings.Contains(string(out), binaryName(nativeArch())) {
		t.Errorf("output = %q, want it to name the build", out)
	}
}

func TestLauncherPassesStdin(t *testing.T) {
	dir := t.TempDir()
	launcher := buildLauncher(t, dir)
	writeStubTarget(t, dir, "cat")

	cmd := exec.Command(launcher)
	cmd.Stdin = strings.NewReader("a line of MCP traffic\n")

	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("launcher error = %v\n%s", err, out)
	}
	if got := strings.TrimSpace(string(out)); got != "a line of MCP traffic" {
		t.Errorf("output = %q, want the input forwarded", got)
	}
}

// errorAs wraps errors.As so the test reads without an extra import block.
func errorAs(err error, target any) bool {
	if err == nil {
		return false
	}
	if exit, ok := err.(*exec.ExitError); ok {
		if p, ok := target.(**exec.ExitError); ok {
			*p = exit
			return true
		}
	}
	return false
}
