package main

import (
	"embed"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"path/filepath"
	"sync"
	"time"

	"xianlv/internal/handler"
	"xianlv/internal/storage"
)

//go:embed web/admin
var embeddedAdmin embed.FS

var adminServerState struct {
	sync.Mutex
	server *http.Server
	url    string
}

func startAdminServer(store *storage.Store, dataDir string) (string, error) {
	adminServerState.Lock()
	defer adminServerState.Unlock()
	if adminServerState.server != nil {
		return adminServerState.url, nil
	}
	assets, err := fs.Sub(embeddedAdmin, "web/admin")
	if err != nil {
		return "", err
	}
	var listener net.Listener
	var address string
	for port := 8088; port <= 8098; port++ {
		address = fmt.Sprintf("127.0.0.1:%d", port)
		listener, err = net.Listen("tcp", address)
		if err == nil {
			break
		}
	}
	if listener == nil {
		return "", fmt.Errorf("管理后台端口不可用: %w", err)
	}
	server := &http.Server{
		Handler:           handler.NewAdminMux(store, assets, filepath.Join(dataDir, "uploads")),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       90 * time.Second,
	}
	adminServerState.server = server
	adminServerState.url = "http://" + address + "/admin"
	go func() {
		serveErr := server.Serve(listener)
		adminServerState.Lock()
		if adminServerState.server == server {
			adminServerState.server = nil
			adminServerState.url = ""
		}
		adminServerState.Unlock()
		if serveErr != nil && serveErr != http.ErrServerClosed {
			writeRuntimeLog("数据管理服务退出", serveErr.Error())
		}
	}()
	return adminServerState.url, nil
}

func stopAdminServer() {
	adminServerState.Lock()
	server := adminServerState.server
	adminServerState.server = nil
	adminServerState.url = ""
	adminServerState.Unlock()
	if server != nil {
		_ = server.Close()
	}
}

func currentAdminURL() string {
	adminServerState.Lock()
	defer adminServerState.Unlock()
	return adminServerState.url
}
