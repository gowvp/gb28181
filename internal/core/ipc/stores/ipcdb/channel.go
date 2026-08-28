package ipcdb

import (
	"context"
	"strconv"
	"strings"

	"github.com/gowvp/owl/internal/core/bz"
	"github.com/gowvp/owl/internal/core/ipc"
	"github.com/ixugo/goddd/pkg/orm"
	"github.com/ixugo/goddd/pkg/reason"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var _ ipc.ChannelStorer = ChannelDB{}

// ChannelDB 通道实体持久化实现
type ChannelDB struct {
	db *gorm.DB
}

// NewChannel 创建通道 Store 实例
func NewChannel(db *gorm.DB) ChannelDB {
	return ChannelDB{db: db}
}

// WithTx 返回使用指定事务的 Store 副本
func (d ChannelDB) WithTx(tx orm.Tx) (ipc.ChannelStorer, error) {
	return ChannelDB{db: orm.GormDB(tx)}, nil
}

// GetByID 按主键查询单条记录
func (d ChannelDB) GetByID(ctx context.Context, id string) (*ipc.Channel, error) {
	if id == "" {
		return nil, reason.ErrBadRequest.WithMsg("通道 ID 不能为空")
	}
	var model ipc.Channel
	if err := d.db.WithContext(ctx).Where("id = ?", id).Take(&model).Error; err != nil {
		return nil, err
	}
	return &model, nil
}

// GetByAppStream 按 app+stream 查询
func (d ChannelDB) GetByAppStream(ctx context.Context, app, stream string) (*ipc.Channel, error) {
	var model ipc.Channel
	if err := d.db.WithContext(ctx).Where("app = ? AND stream = ?", app, stream).Take(&model).Error; err != nil {
		return nil, err
	}
	return &model, nil
}

// GetByStream 按 stream 查询
func (d ChannelDB) GetByStream(ctx context.Context, stream string) (*ipc.Channel, error) {
	var model ipc.Channel
	if err := d.db.WithContext(ctx).Where("stream = ?", stream).Take(&model).Error; err != nil {
		return nil, err
	}
	return &model, nil
}

// GetByDeviceIDAndChannelID 按国标 device_id + channel_id 查询
func (d ChannelDB) GetByDeviceIDAndChannelID(ctx context.Context, deviceID, channelID string) (*ipc.Channel, error) {
	var model ipc.Channel
	if err := d.db.WithContext(ctx).Where("device_id = ? AND channel_id = ?", deviceID, channelID).Take(&model).Error; err != nil {
		return nil, err
	}
	return &model, nil
}

// Create 创建记录
func (d ChannelDB) Create(ctx context.Context, model *ipc.Channel) error {
	return d.db.WithContext(ctx).Create(model).Error
}

// Update 原子更新：SELECT FOR UPDATE + changeFn + Save
func (d ChannelDB) Update(ctx context.Context, model *ipc.Channel, changeFn func(*ipc.Channel) error) error {
	if model.ID == "" {
		return reason.ErrBadRequest.WithMsg("通道 ID 不能为空")
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
func (d ChannelDB) Delete(ctx context.Context, model *ipc.Channel) error {
	if model.ID == "" {
		return reason.ErrBadRequest.WithMsg("通道 ID 不能为空")
	}
	return d.db.WithContext(ctx).Clauses(clause.Returning{}).Delete(model).Error
}

// List 分页查询，过滤条件从 FindChannelInput 构建
func (d ChannelDB) List(ctx context.Context, out *[]*ipc.Channel, in *ipc.FindChannelInput) (int64, error) {
	db := d.db.Model(new(ipc.Channel)).WithContext(ctx)

	if in.DID != "" {
		db = db.Where("did = ?", in.DID)
	}
	if in.DeviceID != "" {
		db = db.Where("device_id = ?", in.DeviceID)
	}
	if in.Key != "" {
		if strings.HasPrefix(in.Key, bz.IDPrefixGBChannel) ||
			strings.HasPrefix(in.Key, bz.IDPrefixRTMP) ||
			strings.HasPrefix(in.Key, bz.IDPrefixRTSP) {
			db = db.Where("id = ?", in.Key)
		} else {
			db = db.Where("channel_id LIKE ? OR name LIKE ? OR app LIKE ? OR stream LIKE ?",
				"%"+in.Key+"%", "%"+in.Key+"%", "%"+in.Key+"%", "%"+in.Key+"%")
		}
	}
	if in.IsOnline == "true" || in.IsOnline == "false" {
		isOnline, _ := strconv.ParseBool(in.IsOnline)
		db = db.Where("is_online = ?", isOnline)
	}
	if in.Type != "" {
		db = db.Where("type = ?", in.Type)
	}
	if in.App != "" {
		db = db.Where("app = ?", in.App)
	}
	if in.Stream != "" {
		db = db.Where("stream = ?", in.Stream)
	}

	// 排序
	if in.OrderBy != "" {
		db = db.Order(in.OrderBy)
	} else if sortExpr := in.MustSortColumn(); sortExpr != "" {
		db = db.Order(sortExpr)
	} else {
		db = db.Order("channel_id, created_at DESC")
	}

	var total int64
	if err := db.Count(&total).Error; err != nil || total <= 0 {
		return total, err
	}
	return total, db.Limit(in.Limit()).Offset(in.Offset()).Find(out).Error
}

// EditGB28181Config 直接更新 GB28181 上报的字段，跳过 SELECT FOR UPDATE
func (d ChannelDB) EditGB28181Config(ctx context.Context, model *ipc.Channel) error {
	return d.db.WithContext(ctx).
		Model(model).
		Select("name", "is_online", "ptz", "ext").
		Updates(model).Error
}

// BatchOfflineByDID 按 did 批量设通道离线
func (d ChannelDB) BatchOfflineByDID(ctx context.Context, did string) error {
	return d.db.WithContext(ctx).Model(new(ipc.Channel)).
		Where("did = ?", did).UpdateColumn("is_online", false).Error
}

// BatchOfflineByType 按通道类型批量设离线
func (d ChannelDB) BatchOfflineByType(ctx context.Context, typ string) error {
	return d.db.WithContext(ctx).Model(new(ipc.Channel)).
		Where("type = ?", typ).UpdateColumn("is_online", false).Error
}

// BatchOfflineByDeviceID 按国标 device_id 批量设离线，排除指定 channel_id 列表
func (d ChannelDB) BatchOfflineByDeviceID(ctx context.Context, deviceID string, excludeChannelIDs []string) error {
	db := d.db.WithContext(ctx).Model(new(ipc.Channel)).Where("device_id = ?", deviceID)
	if len(excludeChannelIDs) > 0 {
		db = db.Where("channel_id NOT IN ?", excludeChannelIDs)
	}
	return db.UpdateColumn("is_online", false).Error
}

// DeleteByDID 按 did 批量删除通道
func (d ChannelDB) DeleteByDID(ctx context.Context, did string) error {
	return d.db.WithContext(ctx).Where("did = ?", did).Delete(new(ipc.Channel)).Error
}
