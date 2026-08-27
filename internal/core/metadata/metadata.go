package metadata

import (
	"context"
	"log/slog"

	"github.com/ixugo/goddd/pkg/orm"
	"github.com/ixugo/goddd/pkg/reason"
	"github.com/jinzhu/copier"
)

// MetadataStorer 元数据持久化接口
type MetadataStorer interface {
	WithTx(orm.Tx) (MetadataStorer, error)
	Create(context.Context, *Metadata) error
	Update(context.Context, *Metadata, func(*Metadata) error) error
	Delete(context.Context, *Metadata) error
	List(context.Context, *[]*Metadata, *ListMetadataInput) (int64, error)
	Count(context.Context, *ListMetadataInput) (int64, error)
	GetByID(context.Context, string) (*Metadata, error)
}

// GetMetadata 按 ID 查询单条数据
func (c Core) GetMetadata(ctx context.Context, id string) (*Metadata, error) {
	out, err := c.store.Metadata().GetByID(ctx, id)
	if err != nil {
		if orm.IsErrRecordNotFound(err) {
			return nil, reason.ErrNotFound.Withf("Get id[%v] err[%s]", id, err.Error())
		}
		return nil, reason.ErrDB.Withf("Get id[%v] err[%s]", id, err.Error())
	}
	return out, nil
}

// CreateMetadata 创建数据记录
func (c Core) CreateMetadata(ctx context.Context, in *CreateMetadataInput) (*Metadata, error) {
	var out Metadata
	if err := copier.Copy(&out, in); err != nil {
		slog.ErrorContext(ctx, "Copy", "err", err)
	}

	if err := c.store.Metadata().Create(ctx, &out); err != nil {
		if orm.IsDuplicatedKey(err) {
			return nil, reason.ErrBadRequest.WithMsg("数据重复").Withf("key[%s]", in.ID)
		}
		return nil, reason.ErrDB.Withf("Create err[%s]", err.Error())
	}
	return &out, nil
}

// UpdateMetadata 更新数据记录
func (c Core) UpdateMetadata(ctx context.Context, in *UpdateMetadataInput) (*Metadata, error) {
	out := Metadata{ID: in.ID}
	if err := c.store.Metadata().Update(ctx, &out, func(b *Metadata) error {
		if err := copier.Copy(b, in); err != nil {
			slog.ErrorContext(ctx, "Copy", "err", err)
		}
		b.LastUpdatedBy = in.LastUpdatedBy
		return nil
	}); err != nil {
		return nil, reason.ErrDB.Withf("Edit id[%v] err[%s]", in.ID, err.Error())
	}
	return &out, nil
}

// SaveMetadata 幂等保存：先尝试更新已有记录，不存在则创建
func (c Core) SaveMetadata(ctx context.Context, in *SaveMetadataInput, id string) (*Metadata, error) {
	out := Metadata{ID: id}
	err := c.store.Metadata().Update(ctx, &out, func(b *Metadata) error {
		b.Ext = in.Ext
		b.LastUpdatedBy = in.LastUpdatedBy
		return nil
	})
	if err == nil {
		return &out, nil
	}
	if !orm.IsErrRecordNotFound(err) {
		return nil, reason.ErrDB.Withf("Edit id[%v] err[%s]", id, err.Error())
	}

	out = Metadata{
		ID:            id,
		Ext:           in.Ext,
		CreatedBy:     in.CreatedBy,
		LastUpdatedBy: in.LastUpdatedBy,
	}
	if err := c.store.Metadata().Create(ctx, &out); err != nil {
		if orm.IsDuplicatedKey(err) {
			return nil, reason.ErrBadRequest.WithMsg("数据重复").Withf("key[%s]", id)
		}
		return nil, reason.ErrDB.Withf("Create err[%s]", err.Error())
	}
	return &out, nil
}
