package ipccache

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/gowvp/owl/internal/core/ipc"
	"github.com/gowvp/owl/pkg/gbs"
	"github.com/gowvp/owl/pkg/gbs/sip"
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

// Begin 开启事务，透传底层 DB
func (c *Cache) Begin() (orm.Tx, error) {
	return c.Storer.Begin()
}

func (c *Cache) Device() ipc.DeviceStorer {
	return (*DeviceCache)(c)
}

func (c *Cache) Channel() ipc.ChannelStorer {
	return (*ChannelCache)(c)
}

func NewCache(store ipc.Storer) *Cache {
	return &Cache{
		Storer:  store,
		devices: &conc.Map[string, *gbs.Device]{},
	}
}

// LoadDeviceToMemory implements gbs.MemoryStorer.
func (c *Cache) LoadDeviceToMemory(conn sip.Connection) {
	devices := make([]*ipc.Device, 0, 100)
	_, err := c.Storer.Device().List(context.TODO(), &devices, &ipc.FindDeviceInput{
		PagerFilter: web.NewPagerFilterMaxSize(),
		ExcludeType: ipc.TypeOnvif,
	})
	if err != nil {
		panic(err)
	}

	for _, d := range devices {
		if strings.ToLower(d.Transport) == "tcp" {
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
			_, err := c.Storer.Channel().List(context.TODO(), &channels, &ipc.FindChannelInput{
				PagerFilter: web.NewPagerFilterMaxSize(),
				DeviceID:    d.GetGB28181DeviceID(),
			})
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
	dev, err := c.Storer.Device().GetByDeviceID(context.TODO(), deviceID)
	if err != nil {
		return err
	}
	if err := c.Storer.Device().Update(context.TODO(), dev, changeFn); err != nil {
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
		if err := c.Storer.Channel().BatchOfflineByDID(context.TODO(), dev.ID); err != nil {
			slog.Error("更新通道离线状态失败", "error", err)
		}
	}
	return nil
}

// DeleteDevice implements gbs.MemoryStorer.
func (c *Cache) DeleteDevice(deviceID string) {
	c.devices.Delete(deviceID)
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
