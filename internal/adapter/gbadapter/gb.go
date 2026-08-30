package gbadapter

import (
	"context"
	"fmt"

	"github.com/gowvp/owl/internal/core/ipc"
	"github.com/gowvp/owl/internal/core/sms"
	"github.com/gowvp/owl/pkg/gbs"
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
	return nil
}

// OnStreamChanged implements ipc.Protocoler.
// 流注销时停止播放并更新播放状态（仅在 regist=false 时由 zlm_webhook 调用）
// GB28181 协议的 stream 就是 channel.ID，app 固定为 rtp
func (a *Adapter) OnStreamChanged(ctx context.Context, app, stream string) error {
	ch, err := a.adapter.GetChannel(ctx, stream)
	if err != nil {
		return err
	}
	// 更新播放状态为 false
	if err := a.adapter.UpdatePlayingByID(ctx, ch.ID, false); err != nil {
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
	return a.gbs.QueryCatalog(device.DeviceID)
}

// StartPlay implements ipc.Protocoler.
func (a *Adapter) StartPlay(ctx context.Context, device *ipc.Device, channel *ipc.Channel) (*ipc.PlayResponse, error) {
	panic("unimplemented")
}

// StopPlay 通过 SIP BYE 通知设备停止推流，并关闭 ZLM 的 RTP 接收端口
func (a *Adapter) StopPlay(ctx context.Context, device *ipc.Device, channel *ipc.Channel) error {
	return a.gbs.StopPlay(ctx, &gbs.StopPlayInput{Channel: channel})
}

// ValidateDevice implements ipc.Protocoler.
func (a *Adapter) ValidateDevice(ctx context.Context, device *ipc.Device) error {
	return nil
}

// PTZControl implements ipc.Protocoler.
// GB28181 协议云台控制实现
func (a *Adapter) PTZControl(ctx context.Context, device *ipc.Device, channel *ipc.Channel, cmd ipc.PTZCommand) error {
	// 检查是否为海康摄像头（通过 manufacturer 判断）
	isHikvision := device.Ext.Manufacturer == "Hikvision"

	if isHikvision {
		// 海康摄像头使用结构化 PTZ 命令格式
		return a.hikPTZControlStructured(device, channel, cmd)
	}

	// 其他厂家使用标准十六进制字符串格式
	return a.standardPTZControl(device, channel, cmd)
}

// hikPTZControlStructured 海康摄像头结构化 PTZ 控制
func (a *Adapter) hikPTZControlStructured(device *ipc.Device, channel *ipc.Channel, cmd ipc.PTZCommand) error {
	var code string
	speed := max(
		// 将 0-1 转换为 1-8
		int(cmd.Speed*8), 1)
	if speed > 8 {
		speed = 8
	}

	switch cmd.Action {
	case "continuous":
		// 映射方向到海康的代码（尝试小写）
		switch cmd.Direction {
		case "up":
			code = "up"
		case "down":
			code = "down"
		case "left":
			code = "left"
		case "right":
			code = "right"
		case "upleft":
			code = "upleft"
		case "upright":
			code = "upright"
		case "downleft":
			code = "downleft"
		case "downright":
			code = "downright"
		case "zoomin":
			code = "zoomin"
		case "zoomout":
			code = "zoomout"
		default:
			return fmt.Errorf("不支持的方向: %s", cmd.Direction)
		}
	case "stop":
		code = "stop"
	default:
		return fmt.Errorf("不支持的动作类型: %s", cmd.Action)
	}

	// 发送结构化 PTZ 命令
	// 第一个参数是设备内部 ID（用于查找设备对象）
	// 第二个参数是通道国标编码（用于 XML 中的 DeviceID 字段）
	return a.gbs.SendPTZCommandStructured(device.ID, channel.ChannelID, code, speed)
}

// standardPTZControl 标准 GB28181 PTZ 控制（十六进制字符串格式）
func (a *Adapter) standardPTZControl(device *ipc.Device, channel *ipc.Channel, cmd ipc.PTZCommand) error {
	var ptzCmd string

	// 根据动作类型构建 PTZ 命令
	switch cmd.Action {
	case "continuous":
		// 连续移动
		ptzCmd = gbs.BuildContinuousMove(cmd.Direction, cmd.Speed)
		if ptzCmd == "" {
			return fmt.Errorf("不支持的方向: %s", cmd.Direction)
		}
	case "stop":
		// 停止移动
		ptzCmd = gbs.BuildStop()
	default:
		return fmt.Errorf("GB28181 暂不支持的动作类型: %s，仅支持 continuous 和 stop", cmd.Action)
	}

	// 发送 PTZ 命令到设备
	// 使用通道的 DeviceID（国标编码）作为目标
	return a.gbs.PTZControl(device.DeviceID, channel.ChannelID, ptzCmd)
}
