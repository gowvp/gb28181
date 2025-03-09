// Code generated by gowebx, DO AVOID EDIT.
package gb28181db

import (
	"context"

	"github.com/gowvp/gb28181/internal/core/gb28181"
	"github.com/ixugo/goweb/pkg/orm"
	"gorm.io/gorm"
)

var _ gb28181.ChannelStorer = Channel{}

// Channel Related business namespaces
type Channel DB

// BatchEdit implements gb28181.ChannelStorer.
func (d Channel) BatchEdit(ctx context.Context, column string, value any, args ...orm.QueryOption) error {
	if len(args) == 0 {
		panic("没有查询条件")
	}
	db := d.db.WithContext(ctx).Model(new(gb28181.Channel))
	for _, fn := range args {
		fn(db)
	}
	return db.UpdateColumn(column, value).Error
}

// NewChannel instance object
func NewChannel(db *gorm.DB) Channel {
	return Channel{db: db}
}

// Find implements gb28181.ChannelStorer.
func (d Channel) Find(ctx context.Context, bs *[]*gb28181.Channel, page orm.Pager, opts ...orm.QueryOption) (int64, error) {
	return orm.FindWithContext(ctx, d.db, bs, page, opts...)
}

// Get implements gb28181.ChannelStorer.
func (d Channel) Get(ctx context.Context, model *gb28181.Channel, opts ...orm.QueryOption) error {
	return orm.FirstWithContext(ctx, d.db, model, opts...)
}

// Add implements gb28181.ChannelStorer.
func (d Channel) Add(ctx context.Context, model *gb28181.Channel) error {
	return d.db.WithContext(ctx).Create(model).Error
}

// Edit implements gb28181.ChannelStorer.
func (d Channel) Edit(ctx context.Context, model *gb28181.Channel, changeFn func(*gb28181.Channel), opts ...orm.QueryOption) error {
	return orm.UpdateWithContext(ctx, d.db, model, changeFn, opts...)
}

// Del implements gb28181.ChannelStorer.
func (d Channel) Del(ctx context.Context, model *gb28181.Channel, opts ...orm.QueryOption) error {
	return orm.DeleteWithContext(ctx, d.db, model, opts...)
}

func (d Channel) Session(ctx context.Context, changeFns ...func(*gorm.DB) error) error {
	return d.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, fn := range changeFns {
			if err := fn(tx); err != nil {
				return err
			}
		}
		return nil
	})
}

func (d Channel) EditWithSession(tx *gorm.DB, model *gb28181.Channel, changeFn func(b *gb28181.Channel) error, opts ...orm.QueryOption) error {
	return orm.UpdateWithSession(tx, model, changeFn, opts...)
}
