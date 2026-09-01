package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const runtimeLogRotateBytes int64 = 16 * 1024 * 1024

type runtimeLogEntry struct {
	dataDir string
	line    string
}

var (
	runtimeLogOnce  sync.Once
	runtimeLogQueue = make(chan runtimeLogEntry, 1024)
)

func writeRuntimeLog(event, detail string) {
	runtimeState.RLock()
	dataDir := runtimeState.dataDir
	runtimeState.RUnlock()
	if strings.TrimSpace(dataDir) == "" {
		if executable, err := os.Executable(); err == nil {
			dataDir = filepath.Dir(executable)
		}
	}
	if strings.TrimSpace(dataDir) == "" {
		return
	}
	line := fmt.Sprintf("%s [%s] %s\r\n", time.Now().Format("2006-01-02 15:04:05.000"), event, strings.ReplaceAll(detail, "\r\n", " "))
	runtimeLogOnce.Do(func() { go runRuntimeLogWriter() })
	select {
	case runtimeLogQueue <- runtimeLogEntry{dataDir: dataDir, line: line}:
	default:
		// Diagnostics must never block QQ message handling. A full queue means
		// the disk is already slower than the event stream, so drop this line.
	}
}

func runRuntimeLogWriter() {
	var file *os.File
	var currentPath string
	closeCurrent := func() {
		if file != nil {
			_ = file.Close()
			file = nil
		}
	}
	for entry := range runtimeLogQueue {
		path := filepath.Join(entry.dataDir, "runtime.log")
		if path != currentPath {
			closeCurrent()
			currentPath = path
		}
		if file == nil {
			_ = os.MkdirAll(entry.dataDir, 0o755)
			if info, err := os.Stat(path); err == nil && info.Size() >= runtimeLogRotateBytes {
				archive := filepath.Join(entry.dataDir, "runtime.previous.log")
				_ = os.Remove(archive)
				_ = os.Rename(path, archive)
			}
			opened, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
			if err != nil {
				continue
			}
			file = opened
		}
		if info, err := file.Stat(); err == nil && info.Size() >= runtimeLogRotateBytes {
			closeCurrent()
			archive := filepath.Join(entry.dataDir, "runtime.previous.log")
			_ = os.Remove(archive)
			_ = os.Rename(path, archive)
			opened, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
			if err != nil {
				continue
			}
			file = opened
		}
		_, _ = file.WriteString(entry.line)
	}
	closeCurrent()
}
