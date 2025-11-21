package onvifadapter

import (
	"context"
	"fmt"
	"log/slog"

	gb28181 "github.com/gowvp/gb28181/internal/core/ipc"
	"github.com/gowvp/gb28181/internal/core/port"
	"github.com/gowvp/onvif"
	m "github.com/gowvp/onvif/media"
	sdkmedia "github.com/gowvp/onvif/sdk/media"
	"github.com/ixugo/goddd/pkg/orm"
)

var _ port.Protocol = (*Adapter)(nil)

// Adapter ONVIF 协议适配器
type Adapter struct {
	devices map[string]*onvif.Device // ONVIF 设备连接缓存
	store   gb28181.Storer           // 协议适配器可以依赖领域的存储接口
}

func NewAdapter(store gb28181.Storer) *Adapter {
	return &Adapter{
		devices: make(map[string]*onvif.Device),
		store:   store,
	}
}

// ValidateDevice 实现 port.Protocol 接口 - ONVIF 设备验证
func (a *Adapter) ValidateDevice(ctx context.Context, device port.Device) error {
	// 尝试连接 ONVIF 设备并验证可以获取 Profiles
	dev, err := onvif.NewDevice(onvif.DeviceParams{
		Xaddr:    fmt.Sprintf("%s:%d", device.GetIP(), device.GetPort()),
		Username: device.GetUsername(),
		Password: device.GetPassword(),
	})
	if err != nil {
		return fmt.Errorf("ONVIF 连接失败: %w", err)
	}

	// 验证可以获取 Profiles
	_, err = sdkmedia.Call_GetProfiles(ctx, dev, m.GetProfiles{})
	if err != nil {
		return fmt.Errorf("获取 ONVIF Profiles 失败: %w", err)
	}

	return nil
}

// InitDevice 实现 port.Protocol 接口 - 初始化 ONVIF 设备
// ONVIF 设备初始化时，自动查询 Profiles 并创建为通道
func (a *Adapter) InitDevice(ctx context.Context, device port.Device) error {
	// 创建 ONVIF 连接
	dev, err := onvif.NewDevice(onvif.DeviceParams{
		Xaddr:    fmt.Sprintf("%s:%d", device.GetIP(), device.GetPort()),
		Username: device.GetUsername(),
		Password: device.GetPassword(),
	})
	if err != nil {
		return err
	}

	// 缓存设备连接
	a.devices[device.GetID()] = dev

	// 自动查询 Profiles 作为通道
	return a.queryAndSaveProfiles(ctx, device, dev)
}

// QueryCatalog 实现 port.Protocol 接口 - ONVIF 查询 Profiles
func (a *Adapter) QueryCatalog(ctx context.Context, device port.Device) error {
	dev, ok := a.devices[device.GetID()]
	if !ok {
		// 设备连接不在缓存中，尝试重新连接
		var err error
		dev, err = onvif.NewDevice(onvif.DeviceParams{
			Xaddr:    fmt.Sprintf("%s:%d", device.GetIP(), device.GetPort()),
			Username: device.GetUsername(),
			Password: device.GetPassword(),
		})
		if err != nil {
			return fmt.Errorf("ONVIF 设备未初始化: %w", err)
		}
		a.devices[device.GetID()] = dev
	}

	return a.queryAndSaveProfiles(ctx, device, dev)
}

// StartPlay 实现 port.Protocol 接口 - ONVIF 播放
func (a *Adapter) StartPlay(ctx context.Context, device port.Device, channel port.Channel) (*port.PlayResponse, error) {
	dev, ok := a.devices[device.GetID()]
	if !ok {
		return nil, fmt.Errorf("ONVIF 设备未初始化")
	}

	// 获取 RTSP 地址
	streamURI, err := a.getStreamURI(ctx, dev, channel.GetChannelID())
	if err != nil {
		return nil, err
	}

	return &port.PlayResponse{
		RTSP: streamURI,
	}, nil
}

// StopPlay 实现 port.Protocol 接口 - ONVIF 停止播放
func (a *Adapter) StopPlay(ctx context.Context, device port.Device, channel port.Channel) error {
	// ONVIF 通常不需要显式停止播放
	return nil
}

// queryAndSaveProfiles 查询 ONVIF Profiles 并保存为通道
func (a *Adapter) queryAndSaveProfiles(ctx context.Context, device port.Device, dev *onvif.Device) error {
	// 查询 ONVIF Profiles
	resp, err := sdkmedia.Call_GetProfiles(ctx, dev, m.GetProfiles{})
	if err != nil {
		return fmt.Errorf("获取 ONVIF Profiles 失败: %w", err)
	}

	// 将 Profiles 转换为通道并保存
	for _, profile := range resp.Profiles {
		channel := &gb28181.Channel{
			DeviceID:  device.GetDeviceID(),
			ChannelID: string(profile.Token),
			Name:      string(profile.Name),
			DID:       device.GetID(),
		}

		// 保存到数据库（使用领域层的存储接口）
		if err := a.store.Channel().Add(ctx, channel); err != nil {
			// 如果是重复错误，忽略
			if orm.IsDuplicatedKey(err) {
				slog.DebugContext(ctx, "通道已存在", "channel_id", channel.ChannelID)
				continue
			}
			slog.ErrorContext(ctx, "保存通道失败", "err", err, "channel_id", channel.ChannelID)
			continue
		}
		slog.InfoContext(ctx, "ONVIF Profile 保存为通道", "channel_id", channel.ChannelID, "name", channel.Name)
	}

	return nil
}

// getStreamURI 获取 RTSP 流地址
func (a *Adapter) getStreamURI(ctx context.Context, dev *onvif.Device, profileToken string) (string, error) {
	// TODO: 调用 ONVIF GetStreamUri 方法
	// 这里需要根据 onvif SDK 的实际 API 来实现

	// 临时实现：假设 profileToken 可以直接构造 RTSP 地址
	params := dev.GetDeviceParams()
	return fmt.Sprintf("rtsp://%s:%s@%s/stream/%s", params.Username, params.Password, params.Xaddr, profileToken), nil
}
