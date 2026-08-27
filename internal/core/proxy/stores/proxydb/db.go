package proxydb

import (
	"github.com/gowvp/owl/internal/core/proxy"
	"github.com/ixugo/goddd/pkg/orm"
	"gorm.io/gorm"
)

var _ proxy.Storer = DB{}

// DB 拉流代理持久层聚合
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

// StreamProxy 获取拉流代理实体 Store
func (d DB) StreamProxy() proxy.StreamProxyStorer {
	return StreamProxyDB{db: d.db}
}

// AutoMigrate 同步数据库表结构
func (d DB) AutoMigrate(ok bool) DB {
	if !ok {
		return d
	}
	if err := d.db.AutoMigrate(new(proxy.StreamProxy)); err != nil {
		panic(err)
	}
	return d
}
