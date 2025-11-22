package rtspadapter

import (
	"context"

	"github.com/gowvp/gb28181/internal/core/ipc"
	"github.com/gowvp/gb28181/internal/core/proxy"
	"github.com/gowvp/gb28181/internal/core/sms"
	"github.com/gowvp/gb28181/pkg/zlm"
)

var _ ipc.Protocoler = (*Adapter)(nil)

type Adapter struct {
	proxyCore *proxy.Core
	smsCore   sms.Core
}

// DeleteDevice implements ipc.Protocoler.
func (a *Adapter) DeleteDevice(ctx context.Context, device *ipc.Device) error {
	return nil
}

func NewAdapter(proxyCore *proxy.Core, smsCore sms.Core) *Adapter {
	return &Adapter{
		proxyCore: proxyCore,
		smsCore:   smsCore,
	}
}

// InitDevice implements ipc.Protocoler.
func (a *Adapter) InitDevice(ctx context.Context, device *ipc.Device) error {
	panic("unimplemented")
}

// OnStreamChanged implements ipc.Protocoler.
func (a *Adapter) OnStreamChanged(ctx context.Context, stream string) error {
	return nil
}

// OnStreamNotFound implements ipc.Protocoler.
func (a *Adapter) OnStreamNotFound(ctx context.Context, app string, stream string) error {
	proxy, err := a.proxyCore.GetStreamProxy(ctx, stream)
	if err != nil {
		return err
	}

	svr, err := a.smsCore.GetMediaServer(ctx, sms.DefaultMediaServerID)
	if err != nil {
		return err
	}
	resp, err := a.smsCore.AddStreamProxy(svr, zlm.AddStreamProxyRequest{
		Vhost:         "__defaultVhost__",
		App:           proxy.App,
		Stream:        proxy.Stream,
		URL:           proxy.SourceURL,
		RetryCount:    3,
		RTPType:       proxy.Transport,
		TimeoutSec:    10,
		EnableHLSFMP4: zlm.NewBool(true),
		EnableAudio:   zlm.NewBool(true),
		EnableRTSP:    zlm.NewBool(true),
		EnableRTMP:    zlm.NewBool(true),
		AddMuteAudio:  zlm.NewBool(true),
		AutoClose:     zlm.NewBool(true),
	})
	if err != nil {
		return err
	}
	// 用于关闭
	a.proxyCore.EditStreamProxyKey(ctx, resp.Data.Key, proxy.ID)

	return nil
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
