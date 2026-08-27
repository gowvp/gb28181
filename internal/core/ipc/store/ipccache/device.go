package ipccache

import (
	"context"
	"log/slog"

	"github.com/gowvp/owl/internal/core/ipc"
	"github.com/gowvp/owl/pkg/gbs"
	"github.com/ixugo/goddd/pkg/orm"
)

var _ ipc.DeviceStorer = &DeviceCache{}

type DeviceCache Cache

// WithTx 事务内绕过缓存，透传底层 DB Store
func (d *DeviceCache) WithTx(tx orm.Tx) (ipc.DeviceStorer, error) {
	return d.Storer.Device().WithTx(tx)
}

// Create implements ipc.DeviceStorer.
func (d *DeviceCache) Create(ctx context.Context, dev *ipc.Device) error {
	if err := d.Storer.Device().Create(ctx, dev); err != nil {
		return err
	}
	d.devices.LoadOrStore(dev.GetGB28181DeviceID(), gbs.NewDevice(nil, dev))
	return nil
}

// Delete implements ipc.DeviceStorer.
func (d *DeviceCache) Delete(ctx context.Context, dev *ipc.Device) error {
	if err := d.Storer.Device().Delete(ctx, dev); err != nil {
		return err
	}
	d.devices.Delete(dev.GetGB28181DeviceID())
	return nil
}

// Update implements ipc.DeviceStorer.
func (d *DeviceCache) Update(ctx context.Context, dev *ipc.Device, changeFn func(*ipc.Device) error) error {
	if err := d.Storer.Device().Update(ctx, dev, changeFn); err != nil {
		return err
	}
	dev2, ok := d.devices.Load(dev.GetGB28181DeviceID())
	if dev.IsGB28181() && ok {
		if dev2.Password != dev.Password && dev.Password != "" {
			slog.InfoContext(ctx, " 修改密码，设备离线")
			cache := (*Cache)(d)
			cache.Change(dev.GetGB28181DeviceID(), func(d *ipc.Device) error {
				d.Password = dev.Password
				d.IsOnline = false
				return nil
			}, func(d *gbs.Device) {
			})
		}
	}

	return nil
}

// List implements ipc.DeviceStorer.
func (d *DeviceCache) List(ctx context.Context, devs *[]*ipc.Device, in *ipc.FindDeviceInput) (int64, error) {
	return d.Storer.Device().List(ctx, devs, in)
}

// GetByID implements ipc.DeviceStorer.
func (d *DeviceCache) GetByID(ctx context.Context, id string) (*ipc.Device, error) {
	return d.Storer.Device().GetByID(ctx, id)
}

// GetByDeviceID implements ipc.DeviceStorer.
func (d *DeviceCache) GetByDeviceID(ctx context.Context, deviceID string) (*ipc.Device, error) {
	return d.Storer.Device().GetByDeviceID(ctx, deviceID)
}
