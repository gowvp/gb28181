package ipccache

import (
	"context"
	"errors"
	"testing"

	"github.com/gowvp/owl/internal/core/ipc"
	"github.com/gowvp/owl/pkg/gbs"
	"github.com/ixugo/goddd/pkg/conc"
	"github.com/ixugo/goddd/pkg/orm"
)

// fakeTx 实现 orm.Tx，仅供 WithTx 链路使用
type fakeTx struct{}

func (fakeTx) Commit() error   { return nil }
func (fakeTx) Rollback() error { return nil }

// fakeDeviceStorer 内嵌接口，仅实现测试用到的方法，其余方法被调用即 panic
type fakeDeviceStorer struct {
	ipc.DeviceStorer
	deleteErr error
}

func (f *fakeDeviceStorer) WithTx(orm.Tx) (ipc.DeviceStorer, error)   { return f, nil }
func (f *fakeDeviceStorer) Create(context.Context, *ipc.Device) error { return nil }
func (f *fakeDeviceStorer) Delete(context.Context, *ipc.Device) error { return f.deleteErr }
func (f *fakeDeviceStorer) Update(_ context.Context, dev *ipc.Device, changeFn func(*ipc.Device) error) error {
	return changeFn(dev)
}

// newTestDeviceCache 手工装配缓存层，避开 NewCache 对底层聚合 Storer 的依赖
func newTestDeviceCache(store ipc.DeviceStorer) *DeviceCache {
	cache := &Cache{devices: &conc.Map[string, *gbs.Device]{}}
	return &DeviceCache{store: store, cache: cache}
}

// 验证 WithTx 返回的副本带 inTx 标记，且共享同一份内存 map
func TestDeviceCacheWithTx(t *testing.T) {
	d := newTestDeviceCache(&fakeDeviceStorer{})
	s, err := d.WithTx(fakeTx{})
	if err != nil {
		t.Fatal(err)
	}
	txCache, ok := s.(*DeviceCache)
	if !ok {
		t.Fatalf("WithTx 应返回 *DeviceCache，实际 %T", s)
	}
	if !txCache.inTx {
		t.Fatal("WithTx 副本应带 inTx 标记")
	}
	if txCache.cache != d.cache {
		t.Fatal("WithTx 副本应共享同一份内存缓存")
	}
}

// 验证 Delete 成功后作废内存条目（事务内外皆失效）
func TestDeviceCacheDelete(t *testing.T) {
	dev := &ipc.Device{DeviceID: "34020000001320000001"}
	s := newTestDeviceCache(&fakeDeviceStorer{})
	s.cache.devices.Store(dev.GetGB28181DeviceID(), nil)

	if err := s.Delete(context.Background(), dev); err != nil {
		t.Fatal(err)
	}
	if _, ok := s.cache.devices.Load(dev.GetGB28181DeviceID()); ok {
		t.Fatal("删除成功后内存条目应被清理")
	}
}

// 验证底层删除失败时内存条目保持不动
func TestDeviceCacheDeleteError(t *testing.T) {
	dev := &ipc.Device{DeviceID: "34020000001320000001"}
	s := newTestDeviceCache(&fakeDeviceStorer{deleteErr: errors.New("db down")})
	s.cache.devices.Store(dev.GetGB28181DeviceID(), nil)

	if err := s.Delete(context.Background(), dev); err == nil {
		t.Fatal("expected delete error, got nil")
	}
	if _, ok := s.cache.devices.Load(dev.GetGB28181DeviceID()); !ok {
		t.Fatal("底层删除失败时内存条目不应被清理")
	}
}

// 验证事务副本内 Create 不写内存，回滚不残留未提交的设备
func TestDeviceCacheInTxCreateSkipsMemory(t *testing.T) {
	d := newTestDeviceCache(&fakeDeviceStorer{})
	s, err := d.WithTx(fakeTx{})
	if err != nil {
		t.Fatal(err)
	}
	dev := &ipc.Device{DeviceID: "34020000001320000001"}
	if err := s.Create(context.Background(), dev); err != nil {
		t.Fatal(err)
	}
	if _, ok := d.cache.devices.Load(dev.GetGB28181DeviceID()); ok {
		t.Fatal("事务副本内 Create 不应写内存")
	}
}

// 验证事务副本内 Update 仅失效内存条目，不写运行时状态
func TestDeviceCacheInTxUpdateInvalidatesOnly(t *testing.T) {
	d := newTestDeviceCache(&fakeDeviceStorer{})
	dev := &ipc.Device{DeviceID: "34020000001320000001"}
	d.cache.devices.Store(dev.GetGB28181DeviceID(), nil)

	s, err := d.WithTx(fakeTx{})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Update(context.Background(), dev, func(m *ipc.Device) error {
		m.Password = "new-password"
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if _, ok := d.cache.devices.Load(dev.GetGB28181DeviceID()); ok {
		t.Fatal("事务副本内 Update 应仅失效内存条目")
	}
}
