package ipcdb

import (
	"context"

	"github.com/gowvp/owl/internal/core/ipc"
	"github.com/ixugo/goddd/pkg/orm"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var _ ipc.DeviceStorer = DeviceDB{}

// DeviceDB 设备实体持久化实现
type DeviceDB struct {
	db *gorm.DB
}

// NewDevice 创建设备 Store 实例
func NewDevice(db *gorm.DB) DeviceDB {
	return DeviceDB{db: db}
}

// WithTx 返回使用指定事务的 Store 副本
func (d DeviceDB) WithTx(tx orm.Tx) (ipc.DeviceStorer, error) {
	return DeviceDB{db: orm.GormDB(tx)}, nil
}

// GetByID 按主键查询单条记录
func (d DeviceDB) GetByID(ctx context.Context, id string) (*ipc.Device, error) {
	if id == "" {
		panic("device: GetByID called with empty ID")
	}
	var model ipc.Device
	if err := d.db.WithContext(ctx).Where("id = ?", id).Take(&model).Error; err != nil {
		return nil, err
	}
	return &model, nil
}

// GetByDeviceID 按国标编号查询
func (d DeviceDB) GetByDeviceID(ctx context.Context, deviceID string) (*ipc.Device, error) {
	var model ipc.Device
	if err := d.db.WithContext(ctx).Where("device_id = ?", deviceID).Take(&model).Error; err != nil {
		return nil, err
	}
	return &model, nil
}

// Create 创建记录
func (d DeviceDB) Create(ctx context.Context, model *ipc.Device) error {
	return d.db.WithContext(ctx).Create(model).Error
}

// Update 原子更新：SELECT FOR UPDATE + changeFn + Save
func (d DeviceDB) Update(ctx context.Context, model *ipc.Device, changeFn func(*ipc.Device) error) error {
	if model.ID == "" {
		panic("device: Update called with empty ID")
	}
	return d.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Take(model).Error; err != nil {
			return err
		}
		if err := changeFn(model); err != nil {
			return err
		}
		return tx.Save(model).Error
	})
}

// Delete 幂等删除
func (d DeviceDB) Delete(ctx context.Context, model *ipc.Device) error {
	if model.ID == "" {
		panic("device: Delete called with empty ID")
	}
	return d.db.WithContext(ctx).Clauses(clause.Returning{}).Delete(model).Error
}

// List 分页查询，过滤条件从 FindDeviceInput 构建
func (d DeviceDB) List(ctx context.Context, out *[]*ipc.Device, in *ipc.FindDeviceInput) (int64, error) {
	db := d.db.Model(new(ipc.Device)).WithContext(ctx)

	if in.Key != "" {
		db = db.Where("name LIKE ? OR device_id LIKE ? OR id = ?", "%"+in.Key+"%", "%"+in.Key+"%", in.Key)
	}
	if in.ExcludeType != "" {
		db = db.Where("type != ?", in.ExcludeType)
	}

	db = db.Order("created_at DESC")

	var total int64
	if err := db.Count(&total).Error; err != nil || total <= 0 {
		return total, err
	}
	return total, db.Limit(in.Limit()).Offset(in.Offset()).Find(out).Error
}
