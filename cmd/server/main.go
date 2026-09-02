package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"xianlv/internal/config"
	"xianlv/internal/handler"
	"xianlv/internal/scheduler"
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

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go runScheduler(ctx, store, cfg)

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		fatal(err)
	}
}

// runScheduler 激活 config.Scheduler 里预留的三个 cron，并追加保留期清理任务。
func runScheduler(ctx context.Context, store *storage.Store, cfg config.Config) {
	sched := scheduler.New()
	mustAdd := func(name, cron string, fn func(ctx context.Context) error) {
		if err := sched.Add(name, cron, fn); err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
	}
	taskRepo := storage.NewTaskRepository(store.DB)
	rankRepo := storage.NewRankRepository(store.DB)
	mustAdd("daily_reset", cfg.Scheduler.DailyReset, func(ctx context.Context) error {
		return taskRepo.ResetDaily()
	})
	mustAdd("ranking_refresh", cfg.Scheduler.RankingRefresh, func(ctx context.Context) error {
		return rankRepo.Refresh()
	})
	mustAdd("backup", cfg.Scheduler.Backup, func(ctx context.Context) error {
		_, err := store.BackupDatabase(cfg.Backup.Directory, cfg.Backup.KeepDays)
		return err
	})
	mustAdd("retention", "0 15 * * * *", func(ctx context.Context) error {
		_, err := store.RunRetention(ctx)
		return err
	})
	sched.Run(ctx)
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
