package ipccache

import (
	"context"

	gb28181 "github.com/gowvp/gb28181/internal/core/ipc"
	"github.com/ixugo/goddd/pkg/orm"
)

var _ gb28181.ChannelStorer = &Channel{}

type Channel Cache

// Add implements gb28181.ChannelStorer.
func (c *Channel) Add(ctx context.Context, ch *gb28181.Channel) error {
	if err := c.Storer.Channel().Add(ctx, ch); err != nil {
		return err
	}
	dev, ok := c.devices.Load(ch.DeviceID)
	if ok {
		dev.LoadChannels(ch)
	}
	return nil
}

// BatchEdit implements gb28181.ChannelStorer.
func (c *Channel) BatchEdit(ctx context.Context, field string, value any, opts ...orm.QueryOption) error {
	return c.Storer.Channel().BatchEdit(ctx, field, value, opts...)
}

// Del implements gb28181.ChannelStorer.
func (c *Channel) Del(ctx context.Context, ch *gb28181.Channel, opts ...orm.QueryOption) error {
	return c.Storer.Channel().Del(ctx, ch, opts...)
}

// Edit implements gb28181.ChannelStorer.
func (c *Channel) Edit(ctx context.Context, ch *gb28181.Channel, changeFn func(*gb28181.Channel), opts ...orm.QueryOption) error {
	return c.Storer.Channel().Edit(ctx, ch, changeFn, opts...)
}

// Find implements gb28181.ChannelStorer.
func (c *Channel) Find(ctx context.Context, chs *[]*gb28181.Channel, pager orm.Pager, opts ...orm.QueryOption) (int64, error) {
	return c.Storer.Channel().Find(ctx, chs, pager, opts...)
}

// Get implements gb28181.ChannelStorer.
func (c *Channel) Get(ctx context.Context, ch *gb28181.Channel, opts ...orm.QueryOption) error {
	return c.Storer.Channel().Get(ctx, ch, opts...)
}
