package ipcdb

import (
	"context"
	"testing"

	"github.com/gowvp/owl/internal/core/ipc"
	"github.com/ixugo/goddd/pkg/orm"
	"github.com/ixugo/goddd/pkg/web"
)

func seedDevice(t *testing.T, store ipc.DeviceStorer, id, deviceID, name, devType string) *ipc.Device {
	t.Helper()
	dev := &ipc.Device{
		ID:       id,
		DeviceID: deviceID,
		Name:     name,
		Type:     devType,
	}
	if err := store.Create(context.Background(), dev); err != nil {
		t.Fatalf("seed device %s: %v", id, err)
	}
	return dev
}

func TestDeviceCreate(t *testing.T) {
	s := testStore(t)
	store := s.Device()
	ctx := context.Background()

	dev := &ipc.Device{ID: "dev_001", DeviceID: "34020000001320000001", Name: "test_cam", Type: ipc.TypeGB28181}
	if err := store.Create(ctx, dev); err != nil {
		t.Fatal(err)
	}

	dup := &ipc.Device{ID: "dev_002", DeviceID: "34020000001320000001", Name: "dup_cam", Type: ipc.TypeGB28181}
	if err := store.Create(ctx, dup); err == nil {
		t.Fatal("expected duplicate key error, got nil")
	}
}

func TestDeviceGetByID(t *testing.T) {
	s := testStore(t)
	store := s.Device()
	ctx := context.Background()

	seedDevice(t, store, "dev_g1", "34020000001320000001", "cam1", ipc.TypeGB28181)

	out, err := store.GetByID(ctx, "dev_g1")
	if err != nil {
		t.Fatal(err)
	}
	if out.Name != "cam1" {
		t.Fatalf("expected name=cam1, got %s", out.Name)
	}

	_, err = store.GetByID(ctx, "not_exist")
	if !orm.IsErrRecordNotFound(err) {
		t.Fatalf("expected not found, got %v", err)
	}
}

func TestDeviceGetByDeviceID(t *testing.T) {
	s := testStore(t)
	store := s.Device()
	ctx := context.Background()

	seedDevice(t, store, "dev_gd1", "34020000001320000002", "cam2", ipc.TypeGB28181)

	out, err := store.GetByDeviceID(ctx, "34020000001320000002")
	if err != nil {
		t.Fatal(err)
	}
	if out.ID != "dev_gd1" {
		t.Fatalf("expected id=dev_gd1, got %s", out.ID)
	}
}

func TestDeviceUpdate(t *testing.T) {
	s := testStore(t)
	store := s.Device()
	ctx := context.Background()

	seedDevice(t, store, "dev_up1", "34020000001320000003", "old_name", ipc.TypeGB28181)

	out := ipc.Device{ID: "dev_up1"}
	if err := store.Update(ctx, &out, func(d *ipc.Device) error {
		d.Name = "new_name"
		d.IsOnline = true
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if out.Name != "new_name" || !out.IsOnline {
		t.Fatalf("expected name=new_name online=true, got name=%s online=%v", out.Name, out.IsOnline)
	}
}

func TestDeviceDelete(t *testing.T) {
	s := testStore(t)
	store := s.Device()
	ctx := context.Background()

	seedDevice(t, store, "dev_del1", "34020000001320000004", "to_delete", ipc.TypeGB28181)

	out := ipc.Device{ID: "dev_del1"}
	if err := store.Delete(ctx, &out); err != nil {
		t.Fatal(err)
	}

	_, err := store.GetByID(ctx, "dev_del1")
	if !orm.IsErrRecordNotFound(err) {
		t.Fatalf("expected not found after delete, got %v", err)
	}
}

func TestDeviceList(t *testing.T) {
	s := testStore(t)
	store := s.Device()
	ctx := context.Background()

	seedDevice(t, store, "dev_l1", "34020000001320000010", "cam1", ipc.TypeGB28181)
	seedDevice(t, store, "dev_l2", "34020000001320000011", "cam2", ipc.TypeGB28181)
	seedDevice(t, store, "dev_l3", "on_001", "onvif1", ipc.TypeOnvif)

	// 全量查询
	_, total, err := store.List(ctx, &ipc.FindDeviceInput{PagerFilter: web.NewPagerFilterMaxSize()})
	if err != nil {
		t.Fatal(err)
	}
	if total != 3 {
		t.Fatalf("expected total=3, got %d", total)
	}

	// ExcludeType
	_, total, err = store.List(ctx, &ipc.FindDeviceInput{
		PagerFilter: web.NewPagerFilterMaxSize(),
		ExcludeType: ipc.TypeOnvif,
	})
	if err != nil {
		t.Fatal(err)
	}
	if total != 2 {
		t.Fatalf("expected total=2 excluding ONVIF, got %d", total)
	}

	// 关键词搜索
	_, total, err = store.List(ctx, &ipc.FindDeviceInput{
		PagerFilter: web.NewPagerFilterMaxSize(),
		Key:         "onvif",
	})
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 {
		t.Fatalf("expected total=1 for key=onvif, got %d", total)
	}

	// 分页
	page1, total, err := store.List(ctx, &ipc.FindDeviceInput{PagerFilter: web.PagerFilter{Size: 2}})
	if err != nil {
		t.Fatal(err)
	}
	if total != 3 || len(page1) != 2 {
		t.Fatalf("expected total=3 len=2, got total=%d len=%d", total, len(page1))
	}
}

func TestDeviceWithTx(t *testing.T) {
	s := testStore(t)
	store := s.Device()
	ctx := context.Background()

	seedDevice(t, store, "dev_tx1", "34020000001320000020", "tx_test", ipc.TypeGB28181)

	tx, err := s.Begin()
	if err != nil {
		t.Fatal(err)
	}

	txStore, _ := store.WithTx(tx)
	dev := ipc.Device{ID: "dev_tx1"}
	if err := txStore.Delete(ctx, &dev); err != nil {
		tx.Rollback()
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}

	_, err = store.GetByID(ctx, "dev_tx1")
	if !orm.IsErrRecordNotFound(err) {
		t.Fatal("expected not found after tx delete")
	}
}

// 验证空 ID 返回参数错误而非 panic
func TestDeviceEmptyID(t *testing.T) {
	store := DeviceDB{}
	ctx := context.Background()

	if _, err := store.GetByID(ctx, ""); err == nil {
		t.Fatal("GetByID 空 ID 应返回错误")
	}
	if err := store.Update(ctx, &ipc.Device{}, func(*ipc.Device) error { return nil }); err == nil {
		t.Fatal("Update 空 ID 应返回错误")
	}
	if err := store.Delete(ctx, &ipc.Device{}); err == nil {
		t.Fatal("Delete 空 ID 应返回错误")
	}
}
