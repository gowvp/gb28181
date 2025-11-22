package ipc

import (
	"context"
)

// Protocoler 协议抽象接口（端口）
//
// 设计原则:
// 1. 接口在 ipc 包内定义，避免循环依赖
// 2. 接口方法直接使用领域模型 (*Device, *Channel)
// 3. 适配器实现此接口，可以直接依赖和修改领域模型
// 4. 符合依赖倒置原则 (DIP):
//   - ipc (高层) 依赖 Protocoler 接口
//   - adapter (低层) 实现 Protocoler 接口
//   - adapter (低层) 依赖 ipc.Device (高层) ✅ 合理
//
// 这就是依赖反转！
type Protocoler interface {
	// ValidateDevice 验证设备连接（添加设备前调用）
	// 可以修改设备信息（如从 ONVIF 获取的固件版本等）
	ValidateDevice(ctx context.Context, device *Device) error

	// InitDevice 初始化设备连接（添加设备后调用）
	// 例如: GB28181 不需要主动初始化，ONVIF 需要查询 Profiles 作为通道
	InitDevice(ctx context.Context, device *Device) error

	// QueryCatalog 查询设备目录/通道
	QueryCatalog(ctx context.Context, device *Device) error

	// StartPlay 开始播放
	StartPlay(ctx context.Context, device *Device, channel *Channel) (*PlayResponse, error)

	// StopPlay 停止播放
	StopPlay(ctx context.Context, device *Device, channel *Channel) error

	DeleteDevice(ctx context.Context, device *Device) error

	Hooker
}

type Hooker interface {
	OnStreamNotFound(ctx context.Context, app, stream string) error
	OnStreamChanged(ctx context.Context, stream string) error
}

// PlayResponse 播放响应
type PlayResponse struct {
	SSRC   string // GB28181 SSRC
	Stream string // 流 ID
	RTSP   string // RTSP 地址 (ONVIF)
}
