package service

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"xianlv/internal/config"
	"xianlv/internal/model"
)

type System struct{ *Base }

func NewSystem(base *Base) *System { return &System{Base: base} }
func (s *System) Status(started time.Time) (map[string]any, error) {
	sqlDB, err := s.Store.DB.DB()
	if err != nil {
		return nil, err
	}
	stats := sqlDB.Stats()
	var online int64
	s.Store.DB.Model(&model.Player{}).Where("online = ?", true).Count(&online)
	return map[string]any{"status": "running", "uptime_seconds": int64(time.Since(started).Seconds()), "online_players": online, "db_open_connections": stats.OpenConnections, "db_in_use": stats.InUse, "time": time.Now()}, nil
}
func (s *System) Backup() (string, error) {
	if strings.ToLower(s.Config.Database.Driver) != "sqlite" {
		return "", errors.New("PostgreSQL请使用pg_dump脚本备份")
	}
	if err := os.MkdirAll(s.Config.Backup.Directory, 0o755); err != nil {
		return "", err
	}
	if err := s.Store.DB.Exec("PRAGMA wal_checkpoint(FULL)").Error; err != nil {
		return "", err
	}
	name := "xianlv-" + time.Now().Format("20060102-150405") + ".db"
	target := filepath.Join(s.Config.Backup.Directory, name)
	if err := copyFile(s.Config.Database.DSN, target); err != nil {
		return "", err
	}
	return target, nil
}
func (s *System) StageRestore(backup string) (string, error) {
	if strings.ToLower(s.Config.Database.Driver) != "sqlite" {
		return "", errors.New("PostgreSQL请使用pg_restore脚本恢复")
	}
	backupAbs, err := filepath.Abs(backup)
	if err != nil {
		return "", err
	}
	backupDir, err := filepath.Abs(s.Config.Backup.Directory)
	if err != nil {
		return "", err
	}
	if !strings.HasPrefix(strings.ToLower(backupAbs), strings.ToLower(backupDir)+string(os.PathSeparator)) {
		return "", errors.New("备份文件必须位于配置的备份目录")
	}
	if _, err := os.Stat(backupAbs); err != nil {
		return "", err
	}
	pending := s.Config.Database.DSN + ".restore-pending"
	if err := copyFile(backupAbs, pending); err != nil {
		return "", err
	}
	return pending, nil
}
func ApplyPendingRestore(cfg config.Config) error {
	if strings.ToLower(cfg.Database.Driver) != "sqlite" {
		return nil
	}
	pending := cfg.Database.DSN + ".restore-pending"
	if _, err := os.Stat(pending); errors.Is(err, os.ErrNotExist) {
		return nil
	} else if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(cfg.Database.DSN), 0o755); err != nil {
		return err
	}
	old := cfg.Database.DSN + ".before-restore"
	if _, err := os.Stat(cfg.Database.DSN); err == nil {
		if err := copyFile(cfg.Database.DSN, old); err != nil {
			return err
		}
	}
	if err := copyFile(pending, cfg.Database.DSN); err != nil {
		return fmt.Errorf("apply pending restore: %w", err)
	}
	return os.Remove(pending)
}
func copyFile(source, target string) error {
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	out, err := os.Create(target)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, in)
	closeErr := out.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}
