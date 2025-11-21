package port

import (
	"context"
)

// Device 设备接口（避免循环依赖）
// 协议适配器的实现会接收具体的 gb28181.Device 类型
type Device interface {
	GetID() string
	GetDeviceID() string
	GetType() string
	GetIP() string
	GetPort() int
	GetUsername() string
	GetPassword() string
}

// Channel 通道接口（避免循环依赖）
type Channel interface {
	GetID() string
	GetChannelID() string
	GetDeviceID() string
}

// Protocol 协议抽象接口（端口）
type Protocol interface {
	// ValidateDevice 验证设备连接（添加设备前调用）
	ValidateDevice(ctx context.Context, device Device) error

	// InitDevice 初始化设备连接（添加设备后调用）
	// 例如: GB28181 不需要主动初始化，ONVIF 需要查询 Profiles 作为通道
	InitDevice(ctx context.Context, device Device) error

	// QueryCatalog 查询设备目录/通道
	QueryCatalog(ctx context.Context, device Device) error

	// StartPlay 开始播放
	StartPlay(ctx context.Context, device Device, channel Channel) (*PlayResponse, error)

	// StopPlay 停止播放
	StopPlay(ctx context.Context, device Device, channel Channel) error
}

// PlayResponse 播放响应
type PlayResponse struct {
	SSRC   string // GB28181 SSRC
	Stream string // 流 ID
	RTSP   string // RTSP 地址 (ONVIF)
}
