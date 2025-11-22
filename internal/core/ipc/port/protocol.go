package port

import (
	"context"
)

// Device 设备接口
// 注意: 适配器实现时，参数类型为 *ipc.Device，满足此接口
type Device interface {
	// 这里不定义任何方法，让所有类型都能满足
	// 适配器实现时直接使用 *ipc.Device 类型
}

// Channel 通道接口
type Channel interface {
	// 同上
}

// Protocol 协议抽象接口（端口）
//
// 设计原则:
// 1. 接口参数使用 Device/Channel 接口（空接口）
// 2. 适配器实现时使用具体类型 *ipc.Device, *ipc.Channel
// 3. 由于是空接口，任何类型都满足，但语义上明确是 Device/Channel
// 4. 避免循环依赖，同时保持类型语义
//
// 使用示例:
//
//	// 适配器实现
//	func (a *Adapter) ValidateDevice(ctx context.Context, device Device) error {
//	    dev := device.(*ipc.Device)  // 类型断言
//	    // ...
//	}
//
//	// 或者更好的方式：在包级别声明具体签名
//	func (a *Adapter) ValidateDevice(ctx context.Context, device *ipc.Device) error
type Protocol interface {
	// ValidateDevice 验证设备连接（添加设备前调用）
	// 参数类型: *ipc.Device
	ValidateDevice(ctx context.Context, device Device) error

	// InitDevice 初始化设备连接（添加设备后调用）
	// 参数类型: *ipc.Device
	InitDevice(ctx context.Context, device Device) error

	// QueryCatalog 查询设备目录/通道
	// 参数类型: *ipc.Device
	QueryCatalog(ctx context.Context, device Device) error

	// StartPlay 开始播放
	// 参数类型: *ipc.Device, *ipc.Channel
	StartPlay(ctx context.Context, device Device, channel Channel) (*PlayResponse, error)

	// StopPlay 停止播放
	// 参数类型: *ipc.Device, *ipc.Channel
	StopPlay(ctx context.Context, device Device, channel Channel) error
}

// PlayResponse 播放响应
type PlayResponse struct {
	SSRC   string // GB28181 SSRC
	Stream string // 流 ID
	RTSP   string // RTSP 地址 (ONVIF)
}
