package event

import (
	"context"

	"github.com/ixugo/goddd/pkg/orm"
)

// Storer 持久层聚合入口
type Storer interface {
	Begin() (orm.Tx, error)
	Event() EventStorer
}

// Dispatcher 告警事件分发接口，解耦 push 包与 event 包，避免循环依赖
type Dispatcher interface {
	Dispatch(ctx context.Context, ev *Event)
}

// Core 事件领域核心
type Core struct {
	store    Storer
	notifier Dispatcher
}

// NewCore 创建领域核心
// notifier 为 nil 时，CreateEventAndNotify 只入库不推送
func NewCore(store Storer, notifier Dispatcher) Core {
	return Core{store: store, notifier: notifier}
}

// CreateEventAndNotify 入库成功后触发 webhook 推送，供 AI 回调 API 层使用
func (c Core) CreateEventAndNotify(ctx context.Context, in *CreateEventInput) (*Event, error) {
	out, err := c.CreateEvent(ctx, in)
	if err != nil {
		return nil, err
	}
	if c.notifier != nil {
		c.notifier.Dispatch(ctx, out)
	}
	return out, nil
}
