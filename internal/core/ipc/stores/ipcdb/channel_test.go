package ipcdb

import (
	"context"
	"testing"

	"github.com/gowvp/owl/internal/core/ipc"
	"github.com/ixugo/goddd/pkg/orm"
	"github.com/ixugo/goddd/pkg/web"
)

func seedChannel(t *testing.T, store ipc.ChannelStorer, id, did, name, chType, app, stream string) *ipc.Channel {
	t.Helper()
	ch := &ipc.Channel{
		ID:     id,
		DID:    did,
		Name:   name,
		Type:   chType,
		App:    app,
		Stream: stream,
	}
	if err := store.Create(context.Background(), ch); err != nil {
		t.Fatalf("seed channel %s: %v", id, err)
	}
	return ch
}

func TestChannelCreate(t *testing.T) {
	s := testStore(t)
	store := s.Channel()
	ctx := context.Background()

	ch := &ipc.Channel{ID: "ch_001", DID: "dev_001", Name: "cam1", Type: ipc.TypeGB28181}
	if err := store.Create(ctx, ch); err != nil {
		t.Fatal(err)
	}

	dup := &ipc.Channel{ID: "ch_001", DID: "dev_001", Name: "cam1_dup", Type: ipc.TypeGB28181}
	if err := store.Create(ctx, dup); err == nil {
		t.Fatal("expected duplicate key error, got nil")
	}
}

func TestChannelGetByID(t *testing.T) {
	s := testStore(t)
	store := s.Channel()
	ctx := context.Background()

	seedChannel(t, store, "ch_get_1", "dev_001", "cam1", ipc.TypeGB28181, "rtp", "ch_get_1")

	out, err := store.GetByID(ctx, "ch_get_1")
	if err != nil {
		t.Fatal(err)
	}
	if out.Name != "cam1" {
		t.Fatalf("expected name=cam1, got %s", out.Name)
	}

	// 不存在
	_, err = store.GetByID(ctx, "not_exist")
	if !orm.IsErrRecordNotFound(err) {
		t.Fatalf("expected record not found, got %v", err)
	}
}

func TestChannelGetByAppStream(t *testing.T) {
	s := testStore(t)
	store := s.Channel()
	ctx := context.Background()

	seedChannel(t, store, "ch_as_1", "dev_001", "cam1", ipc.TypeRTMP, "push", "stream1")

	out, err := store.GetByAppStream(ctx, "push", "stream1")
	if err != nil {
		t.Fatal(err)
	}
	if out.ID != "ch_as_1" {
		t.Fatalf("expected id=ch_as_1, got %s", out.ID)
	}
}

func TestChannelGetByStream(t *testing.T) {
	s := testStore(t)
	store := s.Channel()
	ctx := context.Background()

	seedChannel(t, store, "ch_s_1", "dev_001", "cam1", ipc.TypeRTMP, "push", "unique_stream")

	out, err := store.GetByStream(ctx, "unique_stream")
	if err != nil {
		t.Fatal(err)
	}
	if out.ID != "ch_s_1" {
		t.Fatalf("expected id=ch_s_1, got %s", out.ID)
	}
}

func TestChannelUpdate(t *testing.T) {
	s := testStore(t)
	store := s.Channel()
	ctx := context.Background()

	seedChannel(t, store, "ch_up_1", "dev_001", "old_name", ipc.TypeRTMP, "push", "s1")

	out := ipc.Channel{ID: "ch_up_1"}
	if err := store.Update(ctx, &out, func(b *ipc.Channel) error {
		b.Name = "new_name"
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if out.Name != "new_name" {
		t.Fatalf("expected new_name, got %s", out.Name)
	}
}

func TestChannelDelete(t *testing.T) {
	s := testStore(t)
	store := s.Channel()
	ctx := context.Background()

	seedChannel(t, store, "ch_del_1", "dev_001", "to_delete", ipc.TypeGB28181, "", "")

	out := ipc.Channel{ID: "ch_del_1"}
	if err := store.Delete(ctx, &out); err != nil {
		t.Fatal(err)
	}

	_, err := store.GetByID(ctx, "ch_del_1")
	if !orm.IsErrRecordNotFound(err) {
		t.Fatalf("expected not found after delete, got %v", err)
	}
}

func TestChannelList(t *testing.T) {
	s := testStore(t)
	store := s.Channel()
	ctx := context.Background()

	seedChannel(t, store, "ch_l1", "dev_001", "cam1", ipc.TypeGB28181, "rtp", "ch_l1")
	seedChannel(t, store, "ch_l2", "dev_001", "cam2", ipc.TypeGB28181, "rtp", "ch_l2")
	seedChannel(t, store, "ch_l3", "dev_002", "rtmp1", ipc.TypeRTMP, "push", "s1")

	// 全量查询
	_, total, err := store.List(ctx, &ipc.FindChannelInput{
		PagerFilter: web.NewPagerFilterMaxSize(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if total != 3 {
		t.Fatalf("expected total=3, got %d", total)
	}

	// 按 DID 过滤
	_, total, err = store.List(ctx, &ipc.FindChannelInput{
		PagerFilter: web.NewPagerFilterMaxSize(),
		DID:         "dev_001",
	})
	if err != nil {
		t.Fatal(err)
	}
	if total != 2 {
		t.Fatalf("expected total=2 for did=dev_001, got %d", total)
	}

	// 按 type 过滤
	_, total, err = store.List(ctx, &ipc.FindChannelInput{
		PagerFilter: web.NewPagerFilterMaxSize(),
		Type:        ipc.TypeRTMP,
	})
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 {
		t.Fatalf("expected total=1 for type=RTMP, got %d", total)
	}

	// 分页
	page1, total, err := store.List(ctx, &ipc.FindChannelInput{
		Size: 2,
	})
	if err != nil {
		t.Fatal(err)
	}
	if total != 3 || len(page1) != 2 {
		t.Fatalf("expected total=3 len=2, got total=%d len=%d", total, len(page1))
	}
}

func TestChannelBatchOfflineByType(t *testing.T) {
	s := testStore(t)
	store := s.Channel()
	ctx := context.Background()
	db := testDB(t)
	chStore := NewChannel(db)

	ch1 := &ipc.Channel{ID: "ch_bt_1", DID: "dev_001", Name: "cam1", Type: ipc.TypeRTMP, IsOnline: true}
	chStore.Create(ctx, ch1)
	ch2 := &ipc.Channel{ID: "ch_bt_2", DID: "dev_001", Name: "cam2", Type: ipc.TypeRTMP, IsOnline: true}
	chStore.Create(ctx, ch2)
	ch3 := &ipc.Channel{ID: "ch_bt_3", DID: "dev_002", Name: "cam3", Type: ipc.TypeGB28181, IsOnline: true}
	chStore.Create(ctx, ch3)

	if err := chStore.BatchOfflineByType(ctx, ipc.TypeRTMP); err != nil {
		t.Fatal(err)
	}

	out, _ := chStore.GetByID(ctx, "ch_bt_1")
	if out.IsOnline {
		t.Fatal("expected is_online=false after batch offline")
	}
	out3, _ := chStore.GetByID(ctx, "ch_bt_3")
	if !out3.IsOnline {
		t.Fatal("GB28181 channel should remain online")
	}

	_ = store
}

func TestChannelDeleteByDID(t *testing.T) {
	s := testStore(t)
	store := s.Channel()
	ctx := context.Background()

	seedChannel(t, store, "ch_dd_1", "dev_dd", "cam1", ipc.TypeGB28181, "", "")
	seedChannel(t, store, "ch_dd_2", "dev_dd", "cam2", ipc.TypeGB28181, "", "")
	seedChannel(t, store, "ch_dd_3", "dev_other", "cam3", ipc.TypeGB28181, "", "")

	if err := store.DeleteByDID(ctx, "dev_dd"); err != nil {
		t.Fatal(err)
	}

	items, total, _ := store.List(ctx, &ipc.FindChannelInput{PagerFilter: web.NewPagerFilterMaxSize()})
	if total != 1 || items[0].ID != "ch_dd_3" {
		t.Fatalf("expected only ch_dd_3 remaining, got total=%d", total)
	}
}

func TestChannelWithTx(t *testing.T) {
	s := testStore(t)
	store := s.Channel()
	ctx := context.Background()

	seedChannel(t, store, "ch_tx_1", "dev_001", "cam1", ipc.TypeGB28181, "", "")
	seedChannel(t, store, "ch_tx_2", "dev_001", "cam2", ipc.TypeGB28181, "", "")

	tx, err := s.Begin()
	if err != nil {
		t.Fatal(err)
	}

	txStore, _ := store.WithTx(tx)

	// 事务内删除
	ch := ipc.Channel{ID: "ch_tx_1"}
	if err := txStore.Delete(ctx, &ch); err != nil {
		tx.Rollback()
		t.Fatal(err)
	}

	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}

	// 验证已删除
	_, err = store.GetByID(ctx, "ch_tx_1")
	if !orm.IsErrRecordNotFound(err) {
		t.Fatal("expected not found after tx delete")
	}

	// 另一条仍在
	_, err = store.GetByID(ctx, "ch_tx_2")
	if err != nil {
		t.Fatal(err)
	}
}

// 验证空 ID 返回参数错误而非 panic
func TestChannelEmptyID(t *testing.T) {
	store := ChannelDB{}
	ctx := context.Background()

	if _, err := store.GetByID(ctx, ""); err == nil {
		t.Fatal("GetByID 空 ID 应返回错误")
	}
	if err := store.Update(ctx, &ipc.Channel{}, func(*ipc.Channel) error { return nil }); err == nil {
		t.Fatal("Update 空 ID 应返回错误")
	}
	if err := store.Delete(ctx, &ipc.Channel{}); err == nil {
		t.Fatal("Delete 空 ID 应返回错误")
	}
}
