package ipccache

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/gowvp/gb28181/internal/core/ipc"
	"github.com/gowvp/gb28181/pkg/gbs"
	"github.com/gowvp/gb28181/pkg/gbs/sip"
	"github.com/ixugo/goddd/pkg/conc"
	"github.com/ixugo/goddd/pkg/orm"
	"github.com/ixugo/goddd/pkg/web"
)

var (
	_ gbs.MemoryStorer = &Cache{}
	_ ipc.Storer       = &Cache{}
)

type Cache struct {
	ipc.Storer

	devices *conc.Map[string, *gbs.Device]
}

// LoadOrStore implements gbs.MemoryStorer.
func (c *Cache) LoadOrStore(deviceID string, value *gbs.Device) {
	c.devices.LoadOrStore(deviceID, value)
}

func (c *Cache) Device() ipc.DeviceStorer {
	return (*Device)(c)
}

func (c *Cache) Channel() ipc.ChannelStorer {
	return (*Channel)(c)
}

func NewCache(store ipc.Storer) *Cache {
	return &Cache{
		Storer:  store,
		devices: &conc.Map[string, *gbs.Device]{},
	}
}

// LoadDeviceToMemory implements gbs.MemoryStorer.
func (c *Cache) LoadDeviceToMemory(conn sip.Connection) {
	// TODO: 加载 gb28181 设备
	devices := make([]*ipc.Device, 0, 100)
	_, err := c.Storer.Device().Find(context.TODO(), &devices, web.NewPagerFilterMaxSize(), orm.Where("type != ?", ipc.TypeOnvif))
	if err != nil {
		panic(err)
	}

	for _, d := range devices {
		if strings.ToLower(d.Transport) == "tcp" {
			// 通知相关设备/通道离线
			c.Change(d.GetGB28181DeviceID(), func(d *ipc.Device) error {
				d.IsOnline = false
				return nil
			}, func(d *gbs.Device) {
				d.IsOnline = false
			})
			continue
		}

		dev := gbs.NewDevice(conn, d)
		if dev != nil {
			if err := dev.CheckConnection(); err != nil {
				slog.Warn("检查设备连接失败", "err", err, "username", d.GetGB28181DeviceID(), "to", dev.To())
				continue
			}

			slog.Debug("load device to memory", "username", d.GetGB28181DeviceID(), "to", dev.To())
			channels := make([]*ipc.Channel, 0, 8)
			_, err := c.Storer.Channel().Find(context.TODO(), &channels, web.NewPagerFilterMaxSize(), orm.Where("device_id=?", d.GetGB28181DeviceID()))
			if err != nil {
				panic(err)
			}
			dev.LoadChannels(channels...)
			c.devices.Store(d.GetGB28181DeviceID(), dev)
		}
	}
}

// RangeDevices implements gbs.MemoryStorer.
func (c *Cache) RangeDevices(fn func(key string, value *gbs.Device) bool) {
	c.devices.Range(fn)
}

// Change implements gbs.MemoryStorer.
func (c *Cache) Change(deviceID string, changeFn func(*ipc.Device) error, changeFn2 func(*gbs.Device)) error {
	var dev ipc.Device
	if err := c.Storer.Device().Edit(context.TODO(), &dev, changeFn, orm.Where("device_id=?", deviceID)); err != nil {
		return err
	}

	dev2, ok := c.devices.Load(deviceID)
	if !ok {
		return fmt.Errorf("device not found")
	}
	dev2.IsOnline = dev.IsOnline
	dev2.LastKeepaliveAt = dev.KeepaliveAt.Time
	dev2.LastRegisterAt = dev.RegisteredAt.Time
	dev2.Expires = dev.Expires
	dev2.Password = dev.Password
	dev2.Address = dev.Address
	changeFn2(dev2)
	if !dev2.IsOnline {
		if err := c.Storer.Channel().BatchEdit(context.TODO(), "is_online", false, orm.Where("did=?", dev.ID)); err != nil {
			slog.Error("更新通道离线状态失败", "error", err)
		}
	}
	return nil
}

// GetChannel implements gbs.MemoryStorer.
func (c *Cache) GetChannel(deviceID string, channelID string) (*gbs.Channel, bool) {
	dev, ok := c.devices.Load(deviceID)
	if !ok {
		return nil, false
	}
	return dev.GetChannel(channelID)
}

// Load implements gbs.MemoryStorer.
func (c *Cache) Load(deviceID string) (*gbs.Device, bool) {
	return c.devices.Load(deviceID)
}

// Store implements gbs.MemoryStorer.
func (c *Cache) Store(deviceID string, value *gbs.Device) {
	c.devices.Store(deviceID, value)
}
