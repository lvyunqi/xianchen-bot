//go:build windows && 386

package storage

import (
	"path/filepath"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func sqliteDialector(path string) gorm.Dialector {
	absolute, err := filepath.Abs(path)
	if err == nil {
		path = absolute
	}
	dsn := "file:" + filepath.ToSlash(path) + "?_busy_timeout=5000&_journal_mode=WAL&_synchronous=NORMAL"
	return sqlite.Open(dsn)
}
