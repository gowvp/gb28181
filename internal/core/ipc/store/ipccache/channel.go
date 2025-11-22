package ipccache

import (
	"context"

	"github.com/gowvp/gb28181/internal/core/ipc"
	"github.com/ixugo/goddd/pkg/orm"
	"gorm.io/gorm"
)

var _ ipc.ChannelStorer = &Channel{}

type Channel Cache

// Session implements ipc.ChannelStorer.
func (c *Channel) Session(ctx context.Context, changeFns ...func(*gorm.DB) error) error {
	return c.Storer.Channel().Session(ctx, changeFns...)
}

// Add implements ipc.ChannelStorer.
func (c *Channel) Add(ctx context.Context, ch *ipc.Channel) error {
	if err := c.Storer.Channel().Add(ctx, ch); err != nil {
		return err
	}
	dev, ok := c.devices.Load(ch.DeviceID)
	if ok {
		dev.LoadChannels(ch)
	}
	return nil
}

// BatchEdit implements ipc.ChannelStorer.
func (c *Channel) BatchEdit(ctx context.Context, field string, value any, opts ...orm.QueryOption) error {
	return c.Storer.Channel().BatchEdit(ctx, field, value, opts...)
}

// Del implements ipc.ChannelStorer.
func (c *Channel) Del(ctx context.Context, ch *ipc.Channel, opts ...orm.QueryOption) error {
	return c.Storer.Channel().Del(ctx, ch, opts...)
}

// Edit implements ipc.ChannelStorer.
func (c *Channel) Edit(ctx context.Context, ch *ipc.Channel, changeFn func(*ipc.Channel), opts ...orm.QueryOption) error {
	return c.Storer.Channel().Edit(ctx, ch, changeFn, opts...)
}

// Find implements ipc.ChannelStorer.
func (c *Channel) Find(ctx context.Context, chs *[]*ipc.Channel, pager orm.Pager, opts ...orm.QueryOption) (int64, error) {
	return c.Storer.Channel().Find(ctx, chs, pager, opts...)
}

// Get implements ipc.ChannelStorer.
func (c *Channel) Get(ctx context.Context, ch *ipc.Channel, opts ...orm.QueryOption) error {
	return c.Storer.Channel().Get(ctx, ch, opts...)
}
