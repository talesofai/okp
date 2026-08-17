package testutil

import (
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/talesofai/okp/internal/model"
	"github.com/talesofai/okp/internal/store"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// OpenDatabase replaces the process-global store with an isolated in-memory
// SQLite database for a single test.
func OpenDatabase(t *testing.T) *gorm.DB {
	t.Helper()

	previousDB := store.DB
	previousSQLite := store.IsSQLite

	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	if err := db.AutoMigrate(
		&model.Concept{},
		&model.Link{},
		&model.Revision{},
		&model.User{},
		&model.DomainMeta{},
		&model.DomainReadStat{},
		&model.DomainMember{},
		&model.DomainInvite{},
	); err != nil {
		t.Fatalf("migrate test database: %v", err)
	}
	store.DB = db
	store.IsSQLite = true

	t.Cleanup(func() {
		sqlDB, sqlErr := db.DB()
		if sqlErr == nil {
			_ = sqlDB.Close()
		}
		store.DB = previousDB
		store.IsSQLite = previousSQLite
	})
	return db
}
