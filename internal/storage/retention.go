package storage

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// RetentionPolicy 描述一张流水表的保留期：超过 KeepDays 的行按批次删除。
// 表名只允许出现在下方白名单里，杜绝拼接注入。
type RetentionPolicy struct {
	Table      string
	KeepDays   int
	TimeColumn string // 为空时用 created_at
}

type ExpiryPolicy struct {
	Table      string
	TimeColumn string // expires_at 类列；行到期即删
	Nullable   bool   // 列可空（*time.Time）时跳过 NULL 行，nil 表示永不过期
}

var retentionPolicies = []RetentionPolicy{
	{Table: "social_messages", KeepDays: 60},
	{Table: "broadcasts", KeepDays: 30},
	{Table: "game_logs", KeepDays: 30},
	{Table: "slow_query_logs", KeepDays: 14},
	{Table: "dungeon_runs", KeepDays: 7},
	{Table: "admin_menu_logs", KeepDays: 90},
	{Table: "arena_records", KeepDays: 90},
	{Table: "bank_transactions", KeepDays: 180},
	{Table: "trade_records", KeepDays: 365},
	{Table: "content_reviews", KeepDays: 180},
	{Table: "operation_logs", KeepDays: 365},
}

var expiryPolicies = []ExpiryPolicy{
	{Table: "trade_listings", TimeColumn: "expires_at"},
	{Table: "barter_requests", TimeColumn: "expires_at"},
	{Table: "account_migration_codes", TimeColumn: "expires_at"},
	{Table: "player_values", TimeColumn: "expires_at", Nullable: true},
}

const retentionBatchSize = 5000

// RunRetention 按白名单策略清理流水表并回收 SQLite 空间，返回每张表删除的行数。
// 批量删除（子查询 LIMIT）避免长事务锁库；全部策略跑完一次 incremental_vacuum。
func (s *Store) RunRetention(ctx context.Context) (map[string]int64, error) {
	stats := map[string]int64{}
	now := time.Now()
	tables := s.existingTables()
	for _, p := range retentionPolicies {
		if !tables[p.Table] {
			continue // 老库或测试库未建该表，跳过
		}
		column := p.TimeColumn
		if column == "" {
			column = "created_at"
		}
		cutoff := now.AddDate(0, 0, -p.KeepDays)
		deleted, err := s.deleteInBatches(ctx, p.Table, column, cutoff)
		if err != nil {
			return stats, fmt.Errorf("retention %s: %w", p.Table, err)
		}
		if deleted > 0 {
			stats[p.Table] = deleted
		}
	}
	for _, p := range expiryPolicies {
		if !tables[p.Table] {
			continue
		}
		cond := p.TimeColumn + " < ?"
		args := []any{now}
		if p.Nullable {
			cond = p.TimeColumn + " IS NOT NULL AND " + p.TimeColumn + " < ?"
		}
		deleted, err := s.deleteInBatchesCond(ctx, p.Table, cond, args)
		if err != nil {
			return stats, fmt.Errorf("expiry %s: %w", p.Table, err)
		}
		if deleted > 0 {
			stats[p.Table] = deleted
		}
	}
	if strings.ToLower(s.cfg.Database.Driver) != "postgres" && len(stats) > 0 {
		if err := s.DB.Exec("PRAGMA incremental_vacuum").Error; err != nil {
			return stats, fmt.Errorf("incremental vacuum: %w", err)
		}
	}
	return stats, nil
}

func (s *Store) deleteInBatches(ctx context.Context, table, column string, cutoff time.Time) (int64, error) {
	return s.deleteInBatchesCond(ctx, table, column+" < ?", []any{cutoff})
}

// existingTables 汇总当前库里的表名（SQLite 用 sqlite_master，Postgres 用 information_schema）。
func (s *Store) existingTables() map[string]bool {
	set := map[string]bool{}
	var names []string
	var err error
	if strings.ToLower(s.cfg.Database.Driver) == "postgres" {
		err = s.DB.Raw("SELECT table_name FROM information_schema.tables WHERE table_schema = current_schema()").Scan(&names).Error
	} else {
		err = s.DB.Raw("SELECT name FROM sqlite_master WHERE type = 'table'").Scan(&names).Error
	}
	if err != nil {
		return set
	}
	for _, n := range names {
		set[n] = true
	}
	return set
}

// deleteInBatchesCond 按 WHERE 条件批量删除，每批上限 retentionBatchSize，跨 SQLite/Postgres 通用。
func (s *Store) deleteInBatchesCond(ctx context.Context, table, cond string, args []any) (int64, error) {
	var total int64
	for {
		if err := ctx.Err(); err != nil {
			return total, err
		}
		params := append([]any{}, args...)
		params = append(params, retentionBatchSize)
		res := s.DB.Exec(
			"DELETE FROM "+table+" WHERE id IN (SELECT id FROM "+table+" WHERE "+cond+" LIMIT ?)",
			params...,
		)
		if res.Error != nil {
			return total, res.Error
		}
		total += res.RowsAffected
		if res.RowsAffected < int64(retentionBatchSize) {
			return total, nil
		}
	}
}

// BackupDatabase 用 VACUUM INTO 生成热备份（SQLite 专用；Postgres 模式交由 pg_dump，返回空路径）。
// 备份目录按 KeepDays 清理过期文件。
func (s *Store) BackupDatabase(dir string, keepDays int) (string, error) {
	if strings.ToLower(s.cfg.Database.Driver) == "postgres" {
		return "", nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create backup dir: %w", err)
	}
	target := filepath.Join(dir, "xianchen-backup-"+time.Now().Format("20060102-150405")+".db")
	if err := s.DB.Exec("VACUUM INTO '" + strings.ReplaceAll(filepath.ToSlash(target), "'", "''") + "'").Error; err != nil {
		return "", fmt.Errorf("vacuum into: %w", err)
	}
	if keepDays > 0 {
		cutoff := time.Now().AddDate(0, 0, -keepDays)
		entries, err := os.ReadDir(dir)
		if err == nil {
			for _, e := range entries {
				if e.IsDir() || !strings.HasPrefix(e.Name(), "xianchen-backup-") {
					continue
				}
				info, err := e.Info()
				if err == nil && info.ModTime().Before(cutoff) {
					_ = os.Remove(filepath.Join(dir, e.Name()))
				}
			}
		}
	}
	return target, nil
}

// VacuumDatabase 手动整库压缩（SQLite 专用）。auto_vacuum 对建库后才设置的库需要一次 VACUUM 才生效。
func (s *Store) VacuumDatabase() error {
	if strings.ToLower(s.cfg.Database.Driver) == "postgres" {
		return fmt.Errorf("PostgreSQL 模式请使用数据库自身的 VACUUM 工具")
	}
	return s.DB.Exec("VACUUM").Error
}

// TableStat 单表体积（dbstat 可用时给出，单位 KB）。
type TableStat struct {
	Name string `json:"name"`
	KB   int64  `json:"kb"`
	Rows int64  `json:"rows"`
}

// DatabaseStats 管理端"数据体积"面板数据源。dbstat 虚表不可用时 Tables 为 nil，仅报告文件大小。
func (s *Store) DatabaseStats() (map[string]any, error) {
	out := map[string]any{"mode": s.DatabaseMode()}
	if strings.ToLower(s.cfg.Database.Driver) == "postgres" {
		out["file_size_bytes"] = 0
		out["dbstat_available"] = false
		out["tables"] = nil
		return out, nil
	}
	if dsn := s.cfg.Database.DSN; dsn != "" {
		if info, err := os.Stat(dsn); err == nil {
			out["file_size_bytes"] = info.Size()
		}
	}
	var probe int
	dbstatOK := s.DB.Raw("SELECT 1 FROM dbstat LIMIT 1").Scan(&probe).Error == nil
	out["dbstat_available"] = dbstatOK
	tables := []TableStat{}
	if dbstatOK {
		type row struct {
			Name string
			KB   int64
		}
		var rows []row
		if err := s.DB.Raw(
			"SELECT name, SUM(pgsize)/1024 AS kb FROM dbstat WHERE name NOT LIKE 'sqlite_%' GROUP BY name ORDER BY kb DESC LIMIT 20",
		).Scan(&rows).Error; err != nil {
			dbstatOK = false
		} else {
			for _, r := range rows {
				var count int64
				_ = s.DB.Raw("SELECT COUNT(*) FROM " + quoteIdent(r.Name)).Scan(&count).Error
				tables = append(tables, TableStat{Name: r.Name, KB: r.KB, Rows: count})
			}
		}
	}
	out["dbstat_available"] = dbstatOK
	out["tables"] = tables
	return out, nil
}

func quoteIdent(name string) string {
	return "\"" + strings.ReplaceAll(name, "\"", "") + "\""
}

// SortTableStats 供测试与面板复用的确定性排序。
func SortTableStats(stats []TableStat) {
	sort.Slice(stats, func(i, j int) bool { return stats[i].KB > stats[j].KB })
}
