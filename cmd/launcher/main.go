// Command launcher runs the tekmetric-mcp build that matches the machine.
//
// Claude Desktop reads one command per platform from manifest.json, and the
// manifest format has no key for the CPU architecture. On Linux a shell script
// covers this. Windows has no shell the host is guaranteed to use, so this
// program takes that role.
//
// It is built for windows/amd64. Windows on arm64 runs an amd64 program under
// emulation, so this launcher starts on either machine and then hands control
// to the native build.
package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// binaryName is the build for an architecture, next to this program.
func binaryName(arch string) string {
	suffix := ""
	if runtime.GOOS == "windows" {
		suffix = ".exe"
	}
	return fmt.Sprintf("tekmetric-mcp-%s-%s%s", runtime.GOOS, arch, suffix)
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "tekmetric-mcp: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	self, err := os.Executable()
	if err != nil {
		return fmt.Errorf("cannot locate this program: %w", err)
	}
	dir := filepath.Dir(self)

	// nativeArch reports the machine, not the build of this program. An
	// unknown answer falls back to amd64, which every supported machine runs.
	target := filepath.Join(dir, binaryName(nativeArch()))

	if _, err := os.Stat(target); err != nil {
		return fmt.Errorf("%s is missing", filepath.Base(target))
	}

	cmd := exec.Command(target, os.Args[1:]...)

	// The server speaks MCP over these streams, so pass the handles through
	// rather than copying between pipes.
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			// Report the child's status as this program's status.
			os.Exit(exit.ExitCode())
		}
		return fmt.Errorf("cannot start %s: %w", filepath.Base(target), err)
	}
	return nil
}
