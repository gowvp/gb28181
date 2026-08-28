package metadatadb

import (
	"context"

	"github.com/gowvp/owl/internal/core/metadata"
	"github.com/ixugo/goddd/pkg/orm"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var _ metadata.MetadataStorer = MetadataDB{}

// MetadataDB 元数据实体持久化实现
type MetadataDB struct {
	db *gorm.DB
}

// WithTx 返回使用指定事务的 Store 副本
func (d MetadataDB) WithTx(tx orm.Tx) (metadata.MetadataStorer, error) {
	return MetadataDB{db: orm.GormDB(tx)}, nil
}

// GetByID 按主键查询单条记录
func (d MetadataDB) GetByID(ctx context.Context, id string) (*metadata.Metadata, error) {
	if id == "" {
		panic("metadata: GetByID called with empty ID")
	}
	model := metadata.Metadata{ID: id}
	if err := d.db.WithContext(ctx).Take(&model).Error; err != nil {
		return nil, err
	}
	return &model, nil
}

// Create 创建记录
func (d MetadataDB) Create(ctx context.Context, model *metadata.Metadata) error {
	return d.db.WithContext(ctx).Create(model).Error
}

// Update 原子更新：SELECT FOR UPDATE + changeFn + Save
func (d MetadataDB) Update(ctx context.Context, model *metadata.Metadata, changeFn func(*metadata.Metadata) error) error {
	if model.ID == "" {
		panic("metadata: Update called with empty ID")
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
func (d MetadataDB) Delete(ctx context.Context, model *metadata.Metadata) error {
	if model.ID == "" {
		panic("metadata: Delete called with empty ID")
	}
	return d.db.WithContext(ctx).Clauses(clause.Returning{}).Delete(model).Error
}

// List 分页查询
func (d MetadataDB) List(ctx context.Context, in *metadata.ListMetadataInput) ([]*metadata.Metadata, int64, error) {
	db := d.db.Model(new(metadata.Metadata)).WithContext(ctx)
	var total int64
	if err := db.Count(&total).Error; err != nil || total <= 0 {
		return nil, total, err
	}
	var out []*metadata.Metadata
	return out, total, db.Limit(in.Limit()).Offset(in.Offset()).Find(&out).Error
}

// Count 统计总数
func (d MetadataDB) Count(ctx context.Context, in *metadata.ListMetadataInput) (int64, error) {
	var count int64
	return count, d.db.Model(new(metadata.Metadata)).WithContext(ctx).Count(&count).Error
}
