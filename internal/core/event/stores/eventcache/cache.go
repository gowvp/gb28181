package eventcache

import (
	"github.com/gowvp/owl/internal/core/event"
	"github.com/ixugo/goddd/pkg/conc"
	"github.com/ixugo/goddd/pkg/orm"
)

var _ event.Storer = (*Cache)(nil)

// NewCache 创建缓存装饰层
func NewCache(store event.Storer, cache conc.Cacher) *Cache {
	return &Cache{
		store: store,
		event: cache,
	}
}

// Cache 事件缓存聚合
type Cache struct {
	store event.Storer
	event conc.Cacher
}

// Begin 透传至底层 DB
func (c *Cache) Begin() (orm.Tx, error) {
	return c.store.Begin()
}

// Event 获取缓存包装的事件 Store
func (c *Cache) Event() event.EventStorer {
	return (*EventCache)(c)
}
