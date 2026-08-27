package eventdb

import (
	"context"

	"github.com/gowvp/owl/internal/core/event"
	"github.com/ixugo/goddd/pkg/orm"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var _ event.EventStorer = EventDB{}

// EventDB 事件实体持久化实现
type EventDB struct {
	db *gorm.DB
}

// WithTx 返回使用指定事务的 Store 副本
func (d EventDB) WithTx(tx orm.Tx) (event.EventStorer, error) {
	return EventDB{db: orm.GormDB(tx)}, nil
}

// GetByID 按主键查询单条记录
func (d EventDB) GetByID(ctx context.Context, id int64) (*event.Event, error) {
	if id == 0 {
		panic("event: GetByID called with zero ID")
	}
	model := event.Event{ID: id}
	if err := d.db.WithContext(ctx).Take(&model).Error; err != nil {
		return nil, err
	}
	return &model, nil
}

// Create 创建记录
func (d EventDB) Create(ctx context.Context, model *event.Event) error {
	return d.db.WithContext(ctx).Create(model).Error
}

// Update 原子更新：SELECT FOR UPDATE + changeFn + Save
func (d EventDB) Update(ctx context.Context, model *event.Event, changeFn func(*event.Event) error) error {
	if model.ID == 0 {
		panic("event: Update called with zero ID")
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

// Delete 幂等删除，重复删除不报错
func (d EventDB) Delete(ctx context.Context, model *event.Event) error {
	if model.ID == 0 {
		panic("event: Delete called with zero ID")
	}
	return d.db.WithContext(ctx).Clauses(clause.Returning{}).Delete(model).Error
}

// List 分页查询，过滤条件从 ListEventInput 构建
func (d EventDB) List(ctx context.Context, out *[]*event.Event, in *event.ListEventInput) (int64, error) {
	db := d.db.Model(new(event.Event)).WithContext(ctx)
	if in.CID != "" {
		db = db.Where("cid = ?", in.CID)
	}
	if in.DID != "" {
		db = db.Where("did = ?", in.DID)
	}
	if in.Label != "" {
		db = db.Where("label = ?", in.Label)
	}
	if in.StartMs > 0 && in.EndMs > 0 {
		db = db.Where("started_at >= ? AND started_at <= ?", in.StartAt(), in.EndAt())
	}
	if !in.BeforeAt.IsZero() {
		db = db.Where("started_at < ?", in.BeforeAt)
	}
	db = db.Order("started_at DESC")

	var total int64
	if err := db.Count(&total).Error; err != nil || total <= 0 {
		return total, err
	}
	return total, db.Limit(in.Limit()).Offset(in.Offset()).Find(out).Error
}

// Count 统计总数
func (d EventDB) Count(ctx context.Context, in *event.ListEventInput) (int64, error) {
	db := d.db.Model(new(event.Event)).WithContext(ctx)
	if in.CID != "" {
		db = db.Where("cid = ?", in.CID)
	}
	if in.DID != "" {
		db = db.Where("did = ?", in.DID)
	}
	if in.Label != "" {
		db = db.Where("label = ?", in.Label)
	}
	var total int64
	return total, db.Count(&total).Error
}

// BatchDeleteByIDs 批量按 ID 删除，清理协程专用
func (d EventDB) BatchDeleteByIDs(ctx context.Context, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	return d.db.WithContext(ctx).Where("id IN ?", ids).Delete(&event.Event{}).Error
}
