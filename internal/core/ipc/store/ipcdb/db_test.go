package ipcdb

import (
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/gowvp/owl/internal/core/ipc"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// testDB 返回 SQLite 内存数据库，自动建表，跳过 SQLite 不支持的 FOR UPDATE 子句。
func testDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		SkipDefaultTransaction: true,
	})
	if err != nil {
		t.Fatal(err)
	}

	// SQLite 不支持行级锁，移除 FOR UPDATE
	db.Callback().Query().Before("gorm:query").Register("strip_for_update", func(d *gorm.DB) {
		delete(d.Statement.Clauses, clause.Locking{}.Name())
	})

	if err := db.AutoMigrate(new(ipc.Device), new(ipc.Channel)); err != nil {
		t.Fatal(err)
	}
	return db
}

// testStore 返回完整的 DB Store 聚合（包含 Begin/Device/Channel）
func testStore(t *testing.T) DB {
	t.Helper()
	return NewDB(testDB(t))
}
