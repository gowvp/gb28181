package ipccache

import (
	"context"
	"log/slog"

	"github.com/gowvp/owl/internal/core/ipc"
	"github.com/gowvp/owl/pkg/gbs"
	"github.com/ixugo/goddd/pkg/orm"
)

var _ ipc.DeviceStorer = &DeviceCache{}

// DeviceCache 设备缓存层
// store 为底层 DB Storer（WithTx 时替换为事务副本），cache 回指聚合缓存，
// 共享 devices map 与其上的运行时操作，保证事务内外操作的是同一份内存状态。
// inTx 标记事务副本：事务可能回滚，副本内写操作只做缓存失效、读操作直连 DB，
// 避免内存残留未提交的数据
type DeviceCache struct {
	store ipc.DeviceStorer
	cache *Cache
	inTx  bool
}

// WithTx 返回保留缓存封装的事务副本：事务内写操作仅失效缓存、读操作直连 DB
func (d *DeviceCache) WithTx(tx orm.Tx) (ipc.DeviceStorer, error) {
	storer, err := d.store.WithTx(tx)
	if err != nil {
		return nil, err
	}
	return &DeviceCache{store: storer, cache: d.cache, inTx: true}, nil
}

// Create implements ipc.DeviceStorer.
// 事务副本内不写内存，防止回滚后残留未提交的设备
func (d *DeviceCache) Create(ctx context.Context, dev *ipc.Device) error {
	if err := d.store.Create(ctx, dev); err != nil {
		return err
	}
	if d.inTx {
		return nil
	}
	d.cache.devices.LoadOrStore(dev.GetGB28181DeviceID(), gbs.NewDevice(nil, dev))
	return nil
}

// Delete implements ipc.DeviceStorer.
// 缓存作废永远安全，事务内外皆删库成功即失效：即使事务回滚误删，
// 设备重注册时也会重新推入（GB 流程对未知设备会触发重注册挑战）
func (d *DeviceCache) Delete(ctx context.Context, dev *ipc.Device) error {
	if err := d.store.Delete(ctx, dev); err != nil {
		return err
	}
	d.cache.devices.Delete(dev.GetGB28181DeviceID())
	return nil
}

// Update implements ipc.DeviceStorer.
// 事务副本内仅失效内存条目，不写运行时状态，回滚不残留脏数据
func (d *DeviceCache) Update(ctx context.Context, dev *ipc.Device, changeFn func(*ipc.Device) error) error {
	if err := d.store.Update(ctx, dev, changeFn); err != nil {
		return err
	}
	if d.inTx {
		d.cache.devices.Delete(dev.GetGB28181DeviceID())
		return nil
	}
	dev2, ok := d.cache.devices.Load(dev.GetGB28181DeviceID())
	if dev.IsGB28181() && ok {
		if dev2.Password != dev.Password && dev.Password != "" {
			slog.InfoContext(ctx, " 修改密码，设备离线")
			d.cache.Change(dev.GetGB28181DeviceID(), func(d *ipc.Device) error {
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
	return d.store.List(ctx, devs, in)
}

// GetByID implements ipc.DeviceStorer.
func (d *DeviceCache) GetByID(ctx context.Context, id string) (*ipc.Device, error) {
	return d.store.GetByID(ctx, id)
}

// GetByDeviceID implements ipc.DeviceStorer.
func (d *DeviceCache) GetByDeviceID(ctx context.Context, deviceID string) (*ipc.Device, error) {
	return d.store.GetByDeviceID(ctx, deviceID)
}
