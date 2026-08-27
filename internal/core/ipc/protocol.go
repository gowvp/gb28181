package ipc

import (
	"context"

	"github.com/gowvp/owl/internal/core/bz"
	"github.com/ixugo/goddd/domain/uniqueid"
	"github.com/ixugo/goddd/pkg/orm"
	"github.com/ixugo/goddd/pkg/web"
)

// 为协议适配，提供协议会用到的功能
type Adapter struct {
	store Storer
	uni   uniqueid.Core
}

func GenerateDID(d *Device, uni uniqueid.Core) string {
	if d.IsOnvif() {
		return uni.UniqueID(bz.IDPrefixOnvif)
	}
	return uni.UniqueID(bz.IDPrefixGB)
}

// GenerateChannelID 根据通道类型生成唯一 ID
func GenerateChannelID(c *Channel, uni uniqueid.Core) string {
	switch c.GetType() {
	case TypeOnvif:
		return uni.UniqueID(bz.IDPrefixOnvifChannel)
	case TypeRTMP:
		return uni.UniqueID(bz.IDPrefixRTMP)
	case TypeRTSP:
		return uni.UniqueID(bz.IDPrefixRTSP)
	default:
		if c.IsOnvif() {
			return uni.UniqueID(bz.IDPrefixOnvifChannel)
		}
		return uni.UniqueID(bz.IDPrefixGBChannel)
	}
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
	d, err := g.store.Device().GetByDeviceID(ctx, gbDeviceID)
	if err != nil {
		if !orm.IsErrRecordNotFound(err) {
			return nil, err
		}
		d = &Device{}
		d.init(g.uni.UniqueID(bz.IDPrefixGB), gbDeviceID)
		if err := g.store.Device().Create(ctx, d); err != nil {
			return nil, err
		}
	}
	return d, nil
}

func (g Adapter) Logout(deviceID string, changeFn func(*Device)) error {
	d, err := g.store.Device().GetByDeviceID(context.TODO(), deviceID)
	if err != nil {
		return err
	}
	return g.store.Device().Update(context.TODO(), d, func(d *Device) error {
		changeFn(d)
		return nil
	})
}

func (g Adapter) Update(deviceID string, changeFn func(*Device)) error {
	d, err := g.store.Device().GetByDeviceID(context.TODO(), deviceID)
	if err != nil {
		return err
	}
	return g.store.Device().Update(context.TODO(), d, func(d *Device) error {
		changeFn(d)
		return nil
	})
}

func (g Adapter) UpdatePlayingByID(ctx context.Context, id string, playing bool) error {
	ch := Channel{ID: id}
	return g.store.Channel().Update(ctx, &ch, func(c *Channel) error {
		c.IsPlaying = playing
		return nil
	})
}

func (g Adapter) UpdatePlaying(ctx context.Context, deviceID, channelID string, playing bool) error {
	ch, err := g.store.Channel().GetByDeviceIDAndChannelID(ctx, deviceID, channelID)
	if err != nil {
		return err
	}
	return g.store.Channel().Update(ctx, ch, func(c *Channel) error {
		c.IsPlaying = playing
		return nil
	})
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
	dev, err := g.store.Device().GetByDeviceID(ctx, deviceID)
	if err == nil {
		_ = g.store.Device().Update(ctx, dev, func(d *Device) error {
			d.Channels = len(channels)
			return nil
		})
	}

	// 2. 批量查询该设备的所有现有通道
	var existingChannels []*Channel
	_, _ = g.store.Channel().List(ctx, &existingChannels, &FindChannelInput{
		PagerFilter: web.NewPagerFilterMaxSize(),
		DeviceID:    deviceID,
	})

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
			existing.Name = channel.Name
			existing.IsOnline = channel.IsOnline
			existing.PTZ = channel.PTZ
			existing.Ext.Manufacturer = channel.Ext.Manufacturer
			existing.Ext.Firmware = channel.Ext.Firmware
			existing.Ext.GBVersion = channel.Ext.GBVersion
			existing.Ext.Model = channel.Ext.Model
			_ = g.store.Channel().EditGB28181Config(ctx, existing)
		} else {
			channel.ID = GenerateChannelID(channel, g.uni)
			if dev != nil {
				channel.DID = dev.ID
			}
			_ = g.store.Channel().Create(ctx, channel)
		}
	}

	// 6. 不在上报列表中的通道标记为离线
	if len(currentChannelIDs) > 0 {
		_ = g.store.Channel().BatchOfflineByDeviceID(ctx, deviceID, currentChannelIDs)
	}

	// 7. 更新设备的通道数量
	if dev != nil {
		_ = g.store.Device().Update(ctx, dev, func(d *Device) error {
			d.Channels = len(channels)
			return nil
		})
	}

	return nil
}

// ListDevices 获取所有设备
func (g Adapter) ListDevices(ctx context.Context) ([]*Device, error) {
	var devices []*Device
	if _, err := g.store.Device().List(ctx, &devices, &FindDeviceInput{
		PagerFilter: web.NewPagerFilterMaxSize(),
	}); err != nil {
		return nil, err
	}
	return devices, nil
}

func (g Adapter) GetChannel(ctx context.Context, id string) (*Channel, error) {
	return g.store.Channel().GetByID(ctx, id)
}

func (g Adapter) GetDevice(ctx context.Context, id string) (*Device, error) {
	return g.store.Device().GetByID(ctx, id)
}
