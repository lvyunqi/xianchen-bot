//go:build windows

package main

import (
	"os"
	"syscall"
	"time"
	"unsafe"
)

const processQueryLimitedInformation = 0x1000
const processStillActive = 259

var (
	kernel32                  = syscall.NewLazyDLL("kernel32.dll")
	openProcessProc           = kernel32.NewProc("OpenProcess")
	getExitCodeProcessProc    = kernel32.NewProc("GetExitCodeProcess")
	closeHandleProc           = kernel32.NewProc("CloseHandle")
)

// startHostWatcher 轮询宿主 PID；worker 不是宿主的子进程，不能调用 Process.Wait。
func startHostWatcher(hostPID uint32) {
	if hostPID == 0 {
		return
	}
	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for range ticker.C {
			if !windowsProcessAlive(hostPID) {
				os.Exit(0)
			}
		}
	}()
}

func windowsProcessAlive(pid uint32) bool {
	handle, _, callErr := openProcessProc.Call(processQueryLimitedInformation, 0, uintptr(pid))
	if handle == 0 {
		// Only access denial proves that a non-queryable process may still exist;
		// invalid or already-exited PIDs must let the worker terminate.
		return callErr == syscall.ERROR_ACCESS_DENIED
	}
	defer closeHandleProc.Call(handle)
	var exitCode uint32
	result, _, _ := getExitCodeProcessProc.Call(handle, uintptr(unsafe.Pointer(&exitCode)))
	if result == 0 {
		return true
	}
	return exitCode == processStillActive
}
