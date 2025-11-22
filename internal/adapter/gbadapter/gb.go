package gbadapter

import (
	"context"

	"github.com/gowvp/gb28181/internal/core/ipc"
	"github.com/gowvp/gb28181/internal/core/sms"
	"github.com/gowvp/gb28181/pkg/gbs"
)

var _ ipc.Protocoler = (*Adapter)(nil)

type Adapter struct {
	adapter ipc.Adapter
	gbs     *gbs.Server
	smsCore sms.Core
}

// DeleteDevice implements ipc.Protocoler.
func (a *Adapter) DeleteDevice(ctx context.Context, device *ipc.Device) error {
	return nil
}

func NewAdapter(adapter ipc.Adapter, gbs *gbs.Server, smsCore sms.Core) *Adapter {
	return &Adapter{adapter: adapter, gbs: gbs, smsCore: smsCore}
}

// InitDevice implements ipc.Protocoler.
func (a *Adapter) InitDevice(ctx context.Context, device *ipc.Device) error {
	panic("unimplemented")
}

// OnStreamChanged implements ipc.Protocoler.
func (a *Adapter) OnStreamChanged(ctx context.Context, stream string) error {
	ch, err := a.adapter.GetChannel(ctx, stream)
	if err != nil {
		return err
	}
	return a.gbs.StopPlay(ctx, &gbs.StopPlayInput{Channel: ch})
}

// OnStreamNotFound implements ipc.Protocoler.
func (a *Adapter) OnStreamNotFound(ctx context.Context, app string, stream string) error {
	ch, err := a.adapter.GetChannel(ctx, stream)
	if err != nil {
		return err
	}

	dev, err := a.adapter.GetDevice(ctx, ch.DID)
	if err != nil {
		return err
	}

	svr, err := a.smsCore.GetMediaServer(ctx, sms.DefaultMediaServerID)
	if err != nil {
		return err
	}

	return a.gbs.Play(&gbs.PlayInput{
		Channel:    ch,
		StreamMode: dev.StreamMode,
		SMS:        svr,
	})
}

// QueryCatalog implements ipc.Protocoler.
func (a *Adapter) QueryCatalog(ctx context.Context, device *ipc.Device) error {
	panic("unimplemented")
}

// StartPlay implements ipc.Protocoler.
func (a *Adapter) StartPlay(ctx context.Context, device *ipc.Device, channel *ipc.Channel) (*ipc.PlayResponse, error) {
	panic("unimplemented")
}

// StopPlay implements ipc.Protocoler.
func (a *Adapter) StopPlay(ctx context.Context, device *ipc.Device, channel *ipc.Channel) error {
	panic("unimplemented")
}

// ValidateDevice implements ipc.Protocoler.
func (a *Adapter) ValidateDevice(ctx context.Context, device *ipc.Device) error {
	panic("unimplemented")
}
