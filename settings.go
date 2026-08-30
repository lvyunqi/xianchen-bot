package main

import (
	"os"
	"os/exec"
	"runtime"
	"sync"
)

var settingsState struct {
	sync.RWMutex
	url string
}

func setSettingsURL(url string) {
	settingsState.Lock()
	settingsState.url = url
	settingsState.Unlock()
}

func showSettingsWindow() {
	settingsState.RLock()
	url := settingsState.url
	settingsState.RUnlock()
	if url == "" || runtime.GOOS != "windows" {
		return
	}
	_ = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
}

func closeSettingsWindow() {}

func startHostWatcher(hostPID uint32) {
	go func() {
		process, err := os.FindProcess(int(hostPID))
		if err != nil {
			return
		}
		_, _ = process.Wait()
		os.Exit(0)
	}()
}
