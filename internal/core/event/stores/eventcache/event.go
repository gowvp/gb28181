package eventcache

import (
	"context"
	"fmt"

	"github.com/gowvp/owl/internal/core/event"
	"github.com/ixugo/goddd/pkg/orm"
)

var _ event.EventStorer = (*EventCache)(nil)

// EventCache 事件实体缓存装饰器
type EventCache Cache

func (c *EventCache) cacheKey(key any) string {
	return fmt.Sprintf("EVENT:%v", key)
}

// WithTx 事务内绕过缓存，透传底层 DB Store
func (c *EventCache) WithTx(tx orm.Tx) (event.EventStorer, error) {
	return c.store.Event().WithTx(tx)
}

// GetByID 优先读缓存，miss 时穿透回填
func (c *EventCache) GetByID(ctx context.Context, id int64) (*event.Event, error) {
	key := c.cacheKey(id)
	var model event.Event
	if err := c.event.Get(ctx, key, &model); err == nil {
		return &model, nil
	}
	out, err := c.store.Event().GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	c.event.SetNX(ctx, key, out)
	return out, nil
}

// Create 创建后写入缓存
func (c *EventCache) Create(ctx context.Context, model *event.Event) error {
	if err := c.store.Event().Create(ctx, model); err != nil {
		return err
	}
	c.event.Set(ctx, c.cacheKey(model.ID), model)
	return nil
}

// Update 更新后刷新缓存
func (c *EventCache) Update(ctx context.Context, model *event.Event, changeFn func(*event.Event) error) error {
	if err := c.store.Event().Update(ctx, model, changeFn); err != nil {
		return err
	}
	c.event.Set(ctx, c.cacheKey(model.ID), model)
	return nil
}

// Delete 删除后清除缓存
func (c *EventCache) Delete(ctx context.Context, model *event.Event) error {
	if err := c.store.Event().Delete(ctx, model); err != nil {
		return err
	}
	c.event.Del(ctx, c.cacheKey(model.ID))
	return nil
}

// List 直接透传底层查询
func (c *EventCache) List(ctx context.Context, in *event.ListEventInput) ([]*event.Event, int64, error) {
	return c.store.Event().List(ctx, in)
}

// Count 直接透传底层查询
func (c *EventCache) Count(ctx context.Context, in *event.ListEventInput) (int64, error) {
	return c.store.Event().Count(ctx, in)
}

// BatchDeleteByIDs 批量删除后清除缓存
func (c *EventCache) BatchDeleteByIDs(ctx context.Context, ids []int64) error {
	if err := c.store.Event().BatchDeleteByIDs(ctx, ids); err != nil {
		return err
	}
	for _, id := range ids {
		c.event.Del(ctx, c.cacheKey(id))
	}
	return nil
}
