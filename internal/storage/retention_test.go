package storage

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"xianlv/internal/config"
	"xianlv/internal/model"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func retentionTestStore(t *testing.T) *Store {
	t.Helper()
	path := filepath.Join(t.TempDir(), "retention.db")
	db, err := gorm.Open(sqlite.Open(path), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.SocialMessage{}, &model.GameLog{}, &model.Broadcast{}, &model.TradeListing{}, &model.PlayerValue{}, &model.RankEntry{}, &model.Notice{}, &model.Mail{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	t.Cleanup(func() {
		// Windows 下 WAL 文件被连接池占用会导致 TempDir 清理失败，测试结束显式关池。
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})
	return &Store{DB: db, cfg: config.Config{}}
}

func TestRunRetentionDeletesStaleRows(t *testing.T) {
	store := retentionTestStore(t)
	old := time.Now().AddDate(0, 0, -90)
	fresh := time.Now()
	rows := []model.SocialMessage{
		{Content: "旧消息", CreatedAt: old},
		{Content: "新消息", CreatedAt: fresh},
	}
	if err := store.DB.Create(&rows).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}
	logs := []model.GameLog{{Message: "旧日志", CreatedAt: old}, {Message: "新日志", CreatedAt: fresh}}
	if err := store.DB.Create(&logs).Error; err != nil {
		t.Fatalf("seed log: %v", err)
	}

	stats, err := store.RunRetention(context.Background())
	if err != nil {
		t.Fatalf("RunRetention: %v", err)
	}
	if stats["social_messages"] != 1 {
		t.Errorf("social_messages 应删 1 行，实际 %d", stats["social_messages"])
	}
	if stats["game_logs"] != 1 {
		t.Errorf("game_logs 应删 1 行，实际 %d", stats["game_logs"])
	}
	var remain int64
	store.DB.Model(&model.SocialMessage{}).Count(&remain)
	if remain != 1 {
		t.Errorf("social_messages 应剩 1 行，实际 %d", remain)
	}
}

func TestRunRetentionExpiryPolicies(t *testing.T) {
	store := retentionTestStore(t)
	past := time.Now().Add(-time.Hour)
	future := time.Now().Add(24 * time.Hour)
	listings := []model.TradeListing{{ExpiresAt: past}, {ExpiresAt: future}}
	if err := store.DB.Create(&listings).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}
	stats, err := store.RunRetention(context.Background())
	if err != nil {
		t.Fatalf("RunRetention: %v", err)
	}
	if stats["trade_listings"] != 1 {
		t.Errorf("trade_listings 应删 1 行，实际 %d", stats["trade_listings"])
	}
	values := []model.PlayerValue{{PlayerID: 1, Key: "k1", Value: "v", ExpiresAt: &past}, {PlayerID: 1, Key: "k2", Value: "v"}}
	if err := store.DB.Create(&values).Error; err != nil {
		t.Fatalf("seed player values: %v", err)
	}
	if _, err := store.RunRetention(context.Background()); err != nil {
		t.Fatalf("RunRetention round2: %v", err)
	}
	var valueCount int64
	store.DB.Model(&model.PlayerValue{}).Count(&valueCount)
	if valueCount != 1 {
		t.Errorf("player_values 过期行应删 1 留 1，实际剩 %d", valueCount)
	}
}

func TestRunRetentionLargeTableBatching(t *testing.T) {
	store := retentionTestStore(t)
	base := time.Now().AddDate(0, 0, -400)
	var logs []model.GameLog
	for i := 0; i < 37; i++ {
		logs = append(logs, model.GameLog{Message: "旧", CreatedAt: base.Add(time.Duration(i)*time.Minute)})
	}
	if err := store.DB.CreateInBatches(logs, 50).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}
	if _, err := store.RunRetention(context.Background()); err != nil {
		t.Fatalf("RunRetention: %v", err)
	}
	var remain int64
	store.DB.Model(&model.GameLog{}).Count(&remain)
	if remain != 0 {
		t.Errorf("旧日志应全部删除，实际剩 %d", remain)
	}
}

func TestBackupDatabaseCreatesAndPrunes(t *testing.T) {
	store := retentionTestStore(t)
	dir := t.TempDir()
	target, err := store.BackupDatabase(dir, 30)
	if err != nil {
		t.Fatalf("BackupDatabase: %v", err)
	}
	if target == "" {
		t.Fatal("SQLite 模式应返回备份路径")
	}
	if _, err := os.Stat(target); err != nil {
		t.Fatalf("备份文件应存在: %v", err)
	}
}

func TestDatabaseStatsShape(t *testing.T) {
	store := retentionTestStore(t)
	out, err := store.DatabaseStats()
	if err != nil {
		t.Fatalf("DatabaseStats: %v", err)
	}
	if out["mode"] == "" {
		t.Fatal("mode 不应为空")
	}
}
