//go:build !windows

package main

import "runtime"

// nativeArch reports the architecture of the machine.
//
// Off Windows this program is built for the machine it runs on, so the build
// and the machine agree. The Linux extension uses a shell script instead, so
// this path exists to keep the package buildable and testable everywhere.
func nativeArch() string {
	return runtime.GOARCH
}
