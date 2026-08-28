package ipccache

import (
	"context"

	"github.com/gowvp/owl/internal/core/ipc"
	"github.com/ixugo/goddd/pkg/orm"
)

var _ ipc.ChannelStorer = &ChannelCache{}

type ChannelCache Cache

// WithTx 事务内绕过缓存，透传底层 DB Store
func (c *ChannelCache) WithTx(tx orm.Tx) (ipc.ChannelStorer, error) {
	return c.Storer.Channel().WithTx(tx)
}

// Create implements ipc.ChannelStorer.
func (c *ChannelCache) Create(ctx context.Context, ch *ipc.Channel) error {
	if err := c.Storer.Channel().Create(ctx, ch); err != nil {
		return err
	}
	dev, ok := c.devices.Load(ch.DeviceID)
	if ok {
		dev.LoadChannels(ch)
	}
	return nil
}

// Update implements ipc.ChannelStorer.
func (c *ChannelCache) Update(ctx context.Context, ch *ipc.Channel, changeFn func(*ipc.Channel) error) error {
	return c.Storer.Channel().Update(ctx, ch, changeFn)
}

// Delete implements ipc.ChannelStorer.
func (c *ChannelCache) Delete(ctx context.Context, ch *ipc.Channel) error {
	return c.Storer.Channel().Delete(ctx, ch)
}

// List implements ipc.ChannelStorer.
func (c *ChannelCache) List(ctx context.Context, in *ipc.FindChannelInput) ([]*ipc.Channel, int64, error) {
	return c.Storer.Channel().List(ctx, in)
}

// GetByID implements ipc.ChannelStorer.
func (c *ChannelCache) GetByID(ctx context.Context, id string) (*ipc.Channel, error) {
	return c.Storer.Channel().GetByID(ctx, id)
}

// GetByAppStream implements ipc.ChannelStorer.
func (c *ChannelCache) GetByAppStream(ctx context.Context, app, stream string) (*ipc.Channel, error) {
	return c.Storer.Channel().GetByAppStream(ctx, app, stream)
}

// GetByStream implements ipc.ChannelStorer.
func (c *ChannelCache) GetByStream(ctx context.Context, stream string) (*ipc.Channel, error) {
	return c.Storer.Channel().GetByStream(ctx, stream)
}

// GetByDeviceIDAndChannelID implements ipc.ChannelStorer.
func (c *ChannelCache) GetByDeviceIDAndChannelID(ctx context.Context, deviceID, channelID string) (*ipc.Channel, error) {
	return c.Storer.Channel().GetByDeviceIDAndChannelID(ctx, deviceID, channelID)
}

// BatchOfflineByDID implements ipc.ChannelStorer.
func (c *ChannelCache) BatchOfflineByDID(ctx context.Context, did string) error {
	return c.Storer.Channel().BatchOfflineByDID(ctx, did)
}

// BatchOfflineByType implements ipc.ChannelStorer.
func (c *ChannelCache) BatchOfflineByType(ctx context.Context, typ string) error {
	return c.Storer.Channel().BatchOfflineByType(ctx, typ)
}

// BatchOfflineByDeviceID implements ipc.ChannelStorer.
func (c *ChannelCache) BatchOfflineByDeviceID(ctx context.Context, deviceID string, excludeChannelIDs []string) error {
	return c.Storer.Channel().BatchOfflineByDeviceID(ctx, deviceID, excludeChannelIDs)
}

// DeleteByDID implements ipc.ChannelStorer.
func (c *ChannelCache) DeleteByDID(ctx context.Context, did string) error {
	return c.Storer.Channel().DeleteByDID(ctx, did)
}

// EditGB28181Config implements ipc.ChannelStorer.
func (c *ChannelCache) EditGB28181Config(ctx context.Context, ch *ipc.Channel) error {
	return c.Storer.Channel().EditGB28181Config(ctx, ch)
}
