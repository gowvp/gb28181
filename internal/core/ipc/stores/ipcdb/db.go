package ipcdb

import (
	"github.com/gowvp/owl/internal/core/ipc"
	"github.com/ixugo/goddd/pkg/orm"
	"gorm.io/gorm"
)

var _ ipc.Storer = DB{}

// DB 持久层聚合入口
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

// Device 获取设备实体 Store
func (d DB) Device() ipc.DeviceStorer {
	return DeviceDB{db: d.db}
}

// Channel 获取通道实体 Store
func (d DB) Channel() ipc.ChannelStorer {
	return ChannelDB{db: d.db}
}

// AutoMigrate 同步数据库表结构
func (d DB) AutoMigrate(ok bool) DB {
	if !ok {
		return d
	}
	if err := d.db.AutoMigrate(
		new(ipc.Device),
		new(ipc.Channel),
	); err != nil {
		panic(err)
	}
	return d
}
