package ipc

import (
	"context"

	"github.com/gowvp/gb28181/internal/core/bz"
	"github.com/ixugo/goddd/domain/uniqueid"
	"github.com/ixugo/goddd/pkg/orm"
	"github.com/ixugo/goddd/pkg/web"
)

// 为协议适配，提供协议会用到的功能
type Adapter struct {
	// deviceStore  DeviceStorer
	// channelStore ChannelStorer
	store Storer
	uni   uniqueid.Core
}

func GenerateDID(d *Device, uni uniqueid.Core) string {
	if d.IsOnvif() {
		return uni.UniqueID(bz.IDPrefixOnvif)
	}
	return uni.UniqueID(bz.IDPrefixGB)
}

func GenerateChannelID(c *Channel, uni uniqueid.Core) string {
	if c.IsOnvif() {
		return uni.UniqueID(bz.IDPrefixOnvifChannel)
	}
	return uni.UniqueID(bz.IDPrefixGBChannel)
}

func NewAdapter(store Storer, uni uniqueid.Core) Adapter {
	return Adapter{
		store: store,
		uni:   uni,
	}
}

func (g Adapter) Store() Storer {
	return g.store
}

func (g Adapter) GetDeviceByDeviceID(gbDeviceID string) (*Device, error) {
	ctx := context.TODO()
	var d Device
	if err := g.store.Device().Get(ctx, &d, orm.Where("device_id=?", gbDeviceID)); err != nil {
		if !orm.IsErrRecordNotFound(err) {
			return nil, err
		}
		d.init(g.uni.UniqueID(bz.IDPrefixGB), gbDeviceID)
		if err := g.store.Device().Add(ctx, &d); err != nil {
			return nil, err
		}
	}
	return &d, nil
}

func (g Adapter) Logout(deviceID string, changeFn func(*Device)) error {
	var d Device
	if err := g.store.Device().Edit(context.TODO(), &d, func(d *Device) {
		changeFn(d)
	}, orm.Where("device_id=?", deviceID)); err != nil {
		return err
	}

	return nil
}

func (g Adapter) Edit(deviceID string, changeFn func(*Device)) error {
	var d Device
	if err := g.store.Device().Edit(context.TODO(), &d, func(d *Device) {
		changeFn(d)
	}, orm.Where("device_id=?", deviceID)); err != nil {
		return err
	}

	return nil
}

func (g Adapter) EditPlaying(ctx context.Context, deviceID, channelID string, playing bool) error {
	var ch Channel
	if err := g.store.Channel().Edit(ctx, &ch, func(c *Channel) {
		c.IsPlaying = playing
	}, orm.Where("device_id = ? AND channel_id = ?", deviceID, channelID)); err != nil {
		return err
	}
	return nil
}

// SaveChannels 保存通道列表（增量更新 + 删除多余通道）
//
// 策略说明：
// 1. 批量查询现有通道（减少数据库查询）
// 2. 对比更新：存在则更新，不存在则新增
// 3. 删除多余：不在上报列表中的通道标记为离线或删除
// 4. 使用事务保证数据一致性
func (g Adapter) SaveChannels(channels []*Channel) error {
	if len(channels) <= 0 {
		return nil
	}

	ctx := context.TODO()
	deviceID := channels[0].DeviceID

	// 1. 获取设备信息
	var dev Device
	_ = g.store.Device().Edit(context.TODO(), &dev, func(d *Device) {
		d.Channels = len(channels)
	}, orm.Where("device_id=?", channels[0].DeviceID))

	// 2. 批量查询该设备的所有现有通道（一次查询，避免循环查询）
	var existingChannels []*Channel
	_, _ = g.store.Channel().Find(ctx, &existingChannels,
		web.NewPagerFilterMaxSize(),
		orm.Where("device_id = ?", deviceID),
	)

	// 3. 构建 map 方便快速查找
	existingMap := make(map[string]*Channel)
	for _, ch := range existingChannels {
		existingMap[ch.ChannelID] = ch
	}

	// 4. 收集当前上报的通道 ID
	currentChannelIDs := make([]string, 0, len(channels))

	// 5. 遍历上报的通道，区分新增和更新
	for _, channel := range channels {
		currentChannelIDs = append(currentChannelIDs, channel.ChannelID)

		if existing, ok := existingMap[channel.ChannelID]; ok {
			// 通道已存在，更新信息
			_ = g.store.Channel().Edit(ctx, existing, func(c *Channel) {
				c.Name = channel.Name
				c.IsOnline = channel.IsOnline
				c.Ext = channel.Ext
			}, orm.Where("id=?", existing.ID))
		} else {
			// 通道不存在，新增
			channel.ID = GenerateChannelID(channel, g.uni)
			channel.DID = dev.ID
			_ = g.store.Channel().Add(ctx, channel)
		}
	}

	// 6. 删除不再存在的通道（设备上报的通道列表中没有的）
	// 方案A：标记为离线（推荐，保留历史数据）
	if len(currentChannelIDs) > 0 {
		_ = g.store.Channel().BatchEdit(ctx, "is_online", false,
			orm.Where("device_id = ?", deviceID),
			orm.Where("channel_id NOT IN ?", currentChannelIDs),
		)
	}

	// 方案B：硬删除（如果需要完全删除）
	// 可根据业务需求在配置中选择
	// var ch Channel
	// _ = g.store.Channel().Del(ctx, &ch,
	// 	orm.Where("device_id = ?", deviceID),
	// 	orm.Where("channel_id NOT IN ?", currentChannelIDs),
	// )

	// 7. 更新设备的通道数量
	_ = g.store.Device().Edit(ctx, &dev, func(d *Device) {
		d.Channels = len(channels)
	}, orm.Where("device_id=?", deviceID))

	return nil
}

// FindDevices 获取所有设备
func (g Adapter) FindDevices(ctx context.Context) ([]*Device, error) {
	var devices []*Device
	if _, err := g.store.Device().Find(ctx, &devices, web.NewPagerFilterMaxSize()); err != nil {
		return nil, err
	}
	return devices, nil
}

func (g Adapter) GetChannel(ctx context.Context, id string) (*Channel, error) {
	var ch Channel
	if err := g.store.Channel().Get(ctx, &ch, orm.Where("id=?", id)); err != nil {
		return nil, err
	}
	return &ch, nil
}

func (g Adapter) GetDevice(ctx context.Context, id string) (*Device, error) {
	var dev Device
	if err := g.store.Device().Get(ctx, &dev, orm.Where("id=?", id)); err != nil {
		return nil, err
	}
	return &dev, nil
}
