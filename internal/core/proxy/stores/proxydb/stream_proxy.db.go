package proxydb

import (
	"context"

	"github.com/gowvp/owl/internal/core/proxy"
	"github.com/ixugo/goddd/pkg/orm"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var _ proxy.StreamProxyStorer = StreamProxyDB{}

// StreamProxyDB 拉流代理实体持久化实现
type StreamProxyDB struct {
	db *gorm.DB
}

// WithTx 返回使用指定事务的 Store 副本
func (d StreamProxyDB) WithTx(tx orm.Tx) (proxy.StreamProxyStorer, error) {
	return StreamProxyDB{db: orm.GormDB(tx)}, nil
}

// GetByID 按主键查询
func (d StreamProxyDB) GetByID(ctx context.Context, id string) (*proxy.StreamProxy, error) {
	if id == "" {
		panic("proxy: GetByID called with empty ID")
	}
	model := proxy.StreamProxy{ID: id}
	if err := d.db.WithContext(ctx).Take(&model).Error; err != nil {
		return nil, err
	}
	return &model, nil
}

// GetByAppStream 按 app + stream 组合唯一键查询
func (d StreamProxyDB) GetByAppStream(ctx context.Context, app, stream string) (*proxy.StreamProxy, error) {
	var model proxy.StreamProxy
	if err := d.db.WithContext(ctx).Where("app = ? AND stream = ?", app, stream).Take(&model).Error; err != nil {
		return nil, err
	}
	return &model, nil
}

// Create 创建记录
func (d StreamProxyDB) Create(ctx context.Context, model *proxy.StreamProxy) error {
	return d.db.WithContext(ctx).Create(model).Error
}

// Update 原子更新：SELECT FOR UPDATE + changeFn + Save
func (d StreamProxyDB) Update(ctx context.Context, model *proxy.StreamProxy, changeFn func(*proxy.StreamProxy) error) error {
	if model.ID == "" {
		panic("proxy: Update called with empty ID")
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
func (d StreamProxyDB) Delete(ctx context.Context, model *proxy.StreamProxy) error {
	if model.ID == "" {
		panic("proxy: Delete called with empty ID")
	}
	return d.db.WithContext(ctx).Clauses(clause.Returning{}).Delete(model).Error
}

// List 分页查询
func (d StreamProxyDB) List(ctx context.Context, out *[]*proxy.StreamProxy, in *proxy.ListStreamProxyInput) (int64, error) {
	db := d.db.Model(new(proxy.StreamProxy)).WithContext(ctx).Order("created_at DESC")

	var total int64
	if err := db.Count(&total).Error; err != nil || total <= 0 {
		return total, err
	}
	return total, db.Limit(in.Limit()).Offset(in.Offset()).Find(out).Error
}
