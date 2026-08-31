//go:build linux

package main

import (
	"os"
	"syscall"
	"time"
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
			if err := syscall.Kill(int(hostPID), 0); err == syscall.ESRCH {
				os.Exit(0)
			}
		}
	}()
}
