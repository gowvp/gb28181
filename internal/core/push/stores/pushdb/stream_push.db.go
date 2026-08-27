package pushdb

import (
	"context"

	"github.com/gowvp/owl/internal/core/push"
	"github.com/ixugo/goddd/pkg/orm"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var _ push.StreamPushStorer = StreamPushDB{}

// StreamPushDB 推流实体持久化实现
type StreamPushDB struct {
	db *gorm.DB
}

// WithTx 返回使用指定事务的 Store 副本
func (d StreamPushDB) WithTx(tx orm.Tx) (push.StreamPushStorer, error) {
	return StreamPushDB{db: orm.GormDB(tx)}, nil
}

// GetByID 按主键查询
func (d StreamPushDB) GetByID(ctx context.Context, id string) (*push.StreamPush, error) {
	if id == "" {
		panic("push: GetByID called with empty ID")
	}
	model := push.StreamPush{ID: id}
	if err := d.db.WithContext(ctx).Take(&model).Error; err != nil {
		return nil, err
	}
	return &model, nil
}

// GetByAppStream 按 app + stream 组合唯一键查询
func (d StreamPushDB) GetByAppStream(ctx context.Context, app, stream string) (*push.StreamPush, error) {
	var model push.StreamPush
	if err := d.db.WithContext(ctx).Where("app = ? AND stream = ?", app, stream).Take(&model).Error; err != nil {
		return nil, err
	}
	return &model, nil
}

// Create 创建记录
func (d StreamPushDB) Create(ctx context.Context, model *push.StreamPush) error {
	return d.db.WithContext(ctx).Create(model).Error
}

// Update 原子更新：SELECT FOR UPDATE + changeFn + Save
func (d StreamPushDB) Update(ctx context.Context, model *push.StreamPush, changeFn func(*push.StreamPush) error) error {
	if model.ID == "" {
		panic("push: Update called with empty ID")
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
func (d StreamPushDB) Delete(ctx context.Context, model *push.StreamPush) error {
	if model.ID == "" {
		panic("push: Delete called with empty ID")
	}
	return d.db.WithContext(ctx).Clauses(clause.Returning{}).Delete(model).Error
}

// List 分页查询，支持按状态和关键字筛选
func (d StreamPushDB) List(ctx context.Context, out *[]*push.StreamPush, in *push.ListStreamPushInput) (int64, error) {
	db := d.db.Model(new(push.StreamPush)).WithContext(ctx).Order("created_at DESC")
	if in.Status != "" {
		db = db.Where("status = ?", in.Status)
	}
	if in.Key != "" {
		db = db.Where("id = ? OR app LIKE ? OR stream LIKE ?", in.Key, "%"+in.Key+"%", "%"+in.Key+"%")
	}

	var total int64
	if err := db.Count(&total).Error; err != nil || total <= 0 {
		return total, err
	}
	return total, db.Limit(in.Limit()).Offset(in.Offset()).Find(out).Error
}
