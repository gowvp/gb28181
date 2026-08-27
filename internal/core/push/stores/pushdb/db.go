package pushdb

import (
	"github.com/gowvp/owl/internal/core/push"
	"github.com/ixugo/goddd/pkg/orm"
	"gorm.io/gorm"
)

var _ push.Storer = DB{}

// DB 推流持久层聚合
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

// StreamPush 获取推流实体 Store
func (d DB) StreamPush() push.StreamPushStorer {
	return StreamPushDB{db: d.db}
}

// AutoMigrate 同步数据库表结构
func (d DB) AutoMigrate(ok bool) DB {
	if !ok {
		return d
	}
	if err := d.db.AutoMigrate(new(push.StreamPush)); err != nil {
		panic(err)
	}
	return d
}
