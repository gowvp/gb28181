package onvifadapter

import (
	"context"
	"log/slog"
	"time"

	"github.com/gowvp/gb28181/internal/core/ipc"
	devicemodel "github.com/gowvp/onvif/device"
	sdkdevice "github.com/gowvp/onvif/sdk/device"
	"github.com/ixugo/goddd/pkg/conc"
	"github.com/ixugo/goddd/pkg/orm"
)

// startHealthCheck 启动 ONVIF 设备健康检查（异步心跳 + 状态机方案）
//
// 设计思路:
// 1. 协程 1: 定期发送心跳（30秒），成功则更新内存中的最后心跳时间
// 2. 协程 2: 定期检查状态（1秒），超时未心跳则标记离线
// 3. 状态变化时才同步到数据库，减少数据库写入
func (a *Adapter) startHealthCheck(ctx context.Context) {
	const (
		heartbeatInterval = 30 * time.Second // 心跳间隔：30秒
		checkInterval     = 1 * time.Second  // 状态检查间隔：1秒
		heartbeatTimeout  = 70 * time.Second // 心跳超时：60秒
	)

	// 协程 1: 定期发送心跳
	go a.startHeartbeat(ctx, heartbeatInterval)

	// 协程 2: 定期检查状态
	go a.startStatusChecker(ctx, checkInterval, heartbeatTimeout)
}

// startHeartbeat 协程 1: 定期发送心跳
func (a *Adapter) startHeartbeat(ctx context.Context, interval time.Duration) {
	conc.Timer(ctx, interval, interval, func() {
		a.devices.Range(func(deviceID string, dev *Device) bool {
			// TODO: 设计上预期接入数量较少，可以开几个协程
			// 如果接入设备数量较多，需要优化
			go a.sendHeartbeat(dev)
			return true
		})
	})
}

func (a *Adapter) sendHeartbeat(dev *Device) {
	_, err := sdkdevice.Call_GetDeviceInformation(context.TODO(), dev.Device, devicemodel.GetDeviceInformation{})
	if err == nil {
		dev.KeepaliveAt = orm.Now()
	}
}

// startStatusChecker 协程 2: 定期检查状态
func (a *Adapter) startStatusChecker(ctx context.Context, interval, heartbeatTimeout time.Duration) {
	conc.Timer(ctx, interval, interval, func() {
		now := time.Now()

		a.devices.Range(func(did string, dev *Device) bool {
			if dev.KeepaliveAt.IsZero() {
				return true
			}

			timeSinceLastKeepalive := now.Sub(dev.KeepaliveAt.Time)
			isOnline := timeSinceLastKeepalive < heartbeatTimeout

			if dev.IsOnline == isOnline {
				return true
			}

			dev.IsOnline = isOnline

			// 记录状态变化日志
			if isOnline {
				slog.InfoContext(ctx, "ONVIF 设备上线",
					"device_id", did,
					"offline_duration", timeSinceLastKeepalive)
			} else {
				slog.WarnContext(ctx, "ONVIF 设备离线",
					"device_id", did,
					"last_keepalive", dev.KeepaliveAt.Time,
					"timeout", timeSinceLastKeepalive)
			}

			a.syncDeviceStatusToDB(ctx, did, isOnline)
			return true
		})
	})
}

// syncDeviceStatusToDB 同步设备状态到数据库（状态变化时调用）
func (a *Adapter) syncDeviceStatusToDB(ctx context.Context, did string, isOnline bool) {
	// 更新设备状态
	if err := a.adapter.Edit(did, func(d *ipc.Device) {
		d.IsOnline = isOnline
		if isOnline {
			d.KeepaliveAt = orm.Now()
		}
	}); err != nil {
		slog.ErrorContext(ctx, "更新设备在线状态失败", "err", err, "device_id", did)
		return
	}
}
