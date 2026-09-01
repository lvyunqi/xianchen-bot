package service

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"xianlv/internal/config"
	"xianlv/internal/model"
	"xianlv/internal/storage"
)

// TestLiveDatabaseV222Rehearsal is opt-in because it snapshots a real runtime
// database. VACUUM INTO keeps the source read-only and gives the migration a
// consistent disposable copy.
func TestLiveDatabaseV222Rehearsal(t *testing.T) {
	sourcePath := strings.TrimSpace(os.Getenv("XIANLV_REHEARSAL_DB"))
	if sourcePath == "" {
		t.Skip("XIANLV_REHEARSAL_DB is not set")
	}
	playerID := uint64(93)
	if raw := strings.TrimSpace(os.Getenv("XIANLV_REHEARSAL_PLAYER_ID")); raw != "" {
		parsed, err := strconv.ParseUint(raw, 10, 32)
		if err != nil || parsed == 0 {
			t.Fatalf("invalid XIANLV_REHEARSAL_PLAYER_ID %q", raw)
		}
		playerID = parsed
	}

	snapshotPath := filepath.Join(t.TempDir(), "xianlv-v222-rehearsal.db")
	sourceDSN := "file:" + filepath.ToSlash(sourcePath) + "?mode=ro&_pragma=busy_timeout(10000)"
	source, err := gorm.Open(sqlite.Open(sourceDSN), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("open source database: %v", err)
	}
	var sourcePlayer model.Player
	if err := source.First(&sourcePlayer, uint(playerID)).Error; err != nil {
		t.Fatalf("load source player %d: %v", playerID, err)
	}
	statement := fmt.Sprintf("VACUUM INTO '%s'", strings.ReplaceAll(filepath.ToSlash(snapshotPath), "'", "''"))
	if err := source.Exec(statement).Error; err != nil {
		t.Fatalf("snapshot source database: %v", err)
	}
	if sqlDB, closeErr := source.DB(); closeErr == nil {
		_ = sqlDB.Close()
	}

	cfg := config.Config{Database: config.DatabaseConfig{
		Driver: "sqlite", DSN: snapshotPath, MaxOpenConnections: 4, MaxIdleConnections: 2,
	}}
	store, err := storage.Open(cfg)
	if err != nil {
		t.Fatalf("open rehearsal snapshot: %v", err)
	}
	var before model.Player
	if err := store.DB.First(&before, uint(playerID)).Error; err != nil {
		t.Fatalf("load snapshot player before repair: %v", err)
	}
	game, err := NewGame(store)
	if err != nil {
		t.Fatalf("initialize v2.2.2 against snapshot: %v", err)
	}
	var repaired model.Player
	if err := store.DB.First(&repaired, uint(playerID)).Error; err != nil {
		t.Fatalf("load repaired player: %v", err)
	}
	floor := model.PlayerLevelStats(repaired.Level)
	if repaired.MaxHealth < floor.MaxHealth || repaired.PhysicalAttack < floor.PhysicalAttack ||
		repaired.MagicAttack < floor.MagicAttack || repaired.PhysicalDefense < floor.PhysicalDefense ||
		repaired.MagicDefense < floor.MagicDefense {
		t.Fatalf("level floor not restored: player=%+v floor=%+v", repaired, floor)
	}

	first, handled, err := game.claimV221ServerCompensation(&repaired)
	if err != nil || !handled || !strings.Contains(first.Title, "领取成功") {
		t.Fatalf("first compensation claim failed: handled=%v err=%v result=%+v", handled, err, first)
	}
	second, handled, err := game.claimV221ServerCompensation(&repaired)
	if err != nil || !handled || !strings.Contains(second.Title, "已经领取") {
		t.Fatalf("second compensation claim was not idempotent: handled=%v err=%v result=%+v", handled, err, second)
	}
	var receipts int64
	if err := store.DB.Model(&model.AccountRewardClaim{}).
		Where("claim_key = ? AND player_id = ?", v221CompensationClaimKey, repaired.ID).
		Count(&receipts).Error; err != nil || receipts != 1 {
		t.Fatalf("unexpected compensation receipt count: count=%d err=%v", receipts, err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("close first rehearsal instance: %v", err)
	}

	reopened, err := storage.Open(cfg)
	if err != nil {
		t.Fatalf("reopen rehearsal snapshot: %v", err)
	}
	defer reopened.Close()
	if _, err := NewGame(reopened); err != nil {
		t.Fatalf("initialize v2.2.2 a second time: %v", err)
	}
	var secondStart model.Player
	if err := reopened.DB.First(&secondStart, uint(playerID)).Error; err != nil {
		t.Fatalf("load player after second start: %v", err)
	}
	if secondStart.MaxHealth != repaired.MaxHealth ||
		secondStart.PhysicalAttack != repaired.PhysicalAttack || secondStart.MagicAttack != repaired.MagicAttack ||
		secondStart.PhysicalDefense != repaired.PhysicalDefense || secondStart.MagicDefense != repaired.MagicDefense {
		t.Fatalf("second startup changed repaired attributes: first=%+v second=%+v", repaired, secondStart)
	}

	t.Logf("source player %d LV%d: attack=%d/%d defense=%d/%d health=%d; repaired: attack=%d/%d defense=%d/%d health=%d; compensation receipts=%d",
		playerID, sourcePlayer.Level,
		sourcePlayer.PhysicalAttack, sourcePlayer.MagicAttack,
		sourcePlayer.PhysicalDefense, sourcePlayer.MagicDefense, sourcePlayer.MaxHealth,
		repaired.PhysicalAttack, repaired.MagicAttack,
		repaired.PhysicalDefense, repaired.MagicDefense, repaired.MaxHealth,
		receipts,
	)
}
