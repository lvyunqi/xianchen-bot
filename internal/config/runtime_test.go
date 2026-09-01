package config

import (
	"path/filepath"
	"testing"
)

func TestRuntimeUsesLocalDatabaseByDefault(t *testing.T) {
	t.Setenv("XIANLV_CLOUD_DATABASE_DSN", "")
	dataDir := t.TempDir()
	cfg := Runtime(dataDir)
	if cfg.Database.Driver != "sqlite" || cfg.Database.DSN != filepath.Join(dataDir, "data", "xianlv.db") {
		t.Fatalf("unexpected local runtime database: %+v", cfg.Database)
	}
}

func TestRuntimeUsesCloudDatabaseOnlyWhenExplicitlyConfigured(t *testing.T) {
	const dsn = "postgres://xianchen:secret@db.example.test:5432/xianchen?sslmode=require"
	t.Setenv("XIANLV_CLOUD_DATABASE_DSN", "  "+dsn+"  ")
	cfg := Runtime(t.TempDir())
	if cfg.Database.Driver != "postgres" || cfg.Database.DSN != dsn {
		t.Fatalf("unexpected cloud runtime database: %+v", cfg.Database)
	}
	if cfg.Database.MaxOpenConnections < 2 || cfg.Database.MaxIdleConnections < 1 {
		t.Fatalf("cloud connection pool is not usable: %+v", cfg.Database)
	}
}
