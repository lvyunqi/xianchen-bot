package main

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"xianlv/internal/config"
	"xianlv/internal/handler"
	"xianlv/internal/service"
	"xianlv/internal/storage"
)

func main() {
	configPath := "config.yaml"
	if len(os.Args) > 1 {
		configPath = os.Args[1]
	}
	cfg, err := config.Load(configPath)
	if err != nil {
		fatal(err)
	}
	store, err := storage.Open(cfg)
	if err != nil {
		fatal(err)
	}
	defer store.Close()
	if err := service.SeedPlayerCommandMenus(store); err != nil {
		fatal(err)
	}
	assets := os.DirFS(filepath.Join("web", "admin"))
	mux := handler.NewAdminMux(store, assets, filepath.Join("data", "uploads"))
	server := &http.Server{Addr: cfg.Server.Address, Handler: mux, ReadTimeout: 30 * time.Second, WriteTimeout: 60 * time.Second}
	fmt.Printf("仙尘数据后台：http://%s/admin\n", cfg.Server.Address)
	fmt.Println("数据库初始化完成，默认游戏数据已导入")
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		fatal(err)
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
