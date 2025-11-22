package onvifadapter

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/gowvp/gb28181/internal/core/ipc"
	"github.com/gowvp/gb28181/internal/core/sms"
	"github.com/gowvp/gb28181/pkg/zlm"
	"github.com/ixugo/goddd/pkg/orm"
)

func (a *Adapter) OnStreamChanged(ctx context.Context, stream string) error {
	var ch ipc.Channel
	if err := a.adapter.Store().Channel().Get(ctx, &ch, orm.Where("id=?", stream)); err != nil {
		return err
	}
	if err := a.adapter.EditPlaying(ctx, ch.DeviceID, ch.ChannelID, false); err != nil {
		slog.ErrorContext(ctx, "编辑播放状态失败", "err", err)
	}
	return nil
}

func (a *Adapter) OnStreamNotFound(ctx context.Context, app, stream string) error {
	var ch ipc.Channel
	if err := a.adapter.Store().Channel().Get(ctx, &ch, orm.Where("id=?", stream)); err != nil {
		return err
	}

	onvifDev, ok := a.devices.Load(ch.DeviceID)
	if !ok {
		return fmt.Errorf("ONVIF 设备未初始化")
	}

	streamURI, err := a.getStreamURI(ctx, onvifDev, ch.ChannelID)
	if err != nil {
		return err
	}
	svr, err := a.sms.GetMediaServer(ctx, sms.DefaultMediaServerID)
	if err != nil {
		return err
	}

	_, err = a.sms.AddStreamProxy(svr, zlm.AddStreamProxyRequest{
		Vhost:         "__defaultVhost__",
		App:           app,
		Stream:        stream,
		URL:           streamURI,
		RetryCount:    3,
		TimeoutSec:    10,
		EnableHLSFMP4: zlm.NewBool(true),
		EnableAudio:   zlm.NewBool(true),
		EnableRTSP:    zlm.NewBool(true),
		EnableRTMP:    zlm.NewBool(true),
		AddMuteAudio:  zlm.NewBool(true),
		AutoClose:     zlm.NewBool(true),
	})
	if err == nil {
		if err := a.adapter.EditPlaying(ctx, ch.DeviceID, ch.ChannelID, true); err != nil {
			slog.ErrorContext(ctx, "编辑播放状态失败", "err", err)
		}
	}
	return err
}
