package metadatadb

import (
	"github.com/gowvp/owl/internal/core/metadata"
	"github.com/ixugo/goddd/pkg/orm"
	"gorm.io/gorm"
)

var _ metadata.Storer = DB{}

// DB 元数据持久层聚合
type DB struct {
	db *gorm.DB
}

// NewDB 创建持久层聚合实例
func NewDB(db *gorm.DB) DB {
	return DB{db: db}
}

// Begin 开启事务
func (d DB) Begin() (orm.Tx, error) {
	return orm.Begin(d.db)
}

// Metadata 获取元数据实体 Store
func (d DB) Metadata() metadata.MetadataStorer {
	return MetadataDB{db: d.db}
}

// AutoMigrate 同步数据库表结构
func (d DB) AutoMigrate(ok bool) DB {
	if !ok {
		return d
	}
	if err := d.db.AutoMigrate(new(metadata.Metadata)); err != nil {
		panic(err)
	}
	return d
}
