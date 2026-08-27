//go:build windows

package main

import (
	"syscall"
	"unsafe"
)

// imageFileMachineARM64 is IMAGE_FILE_MACHINE_ARM64 from the Windows headers.
const imageFileMachineARM64 = 0xAA64

// nativeArch reports the architecture of the machine.
//
// This program is built for amd64 and may be running under emulation on an
// arm64 machine, so runtime.GOARCH describes the build rather than the machine.
// IsWow64Process2 reports both, and its second answer is the machine.
//
// Every failure path returns amd64. An amd64 build runs on both machines, so a
// wrong answer costs emulation rather than a server that will not start.
func nativeArch() string {
	const fallback = "amd64"

	proc := syscall.NewLazyDLL("kernel32.dll").NewProc("IsWow64Process2")
	if err := proc.Find(); err != nil {
		// Windows 10 version 1511 and older. Those releases predate arm64.
		return fallback
	}

	handle, err := syscall.GetCurrentProcess()
	if err != nil {
		return fallback
	}

	var processMachine, nativeMachine uint16
	ret, _, _ := proc.Call(
		uintptr(handle),
		uintptr(unsafe.Pointer(&processMachine)),
		uintptr(unsafe.Pointer(&nativeMachine)),
	)
	if ret == 0 {
		return fallback
	}

	if nativeMachine == imageFileMachineARM64 {
		return "arm64"
	}
	return fallback
}
