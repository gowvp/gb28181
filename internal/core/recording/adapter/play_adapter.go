package adapter

import (
	"context"
	"fmt"

	"github.com/gowvp/owl/internal/core/recording"
	"github.com/gowvp/owl/internal/core/sms"
)

var _ recording.PlayProvider = (*PlayAdapter)(nil)

// PlayAdapter 通过 ZLM getSnap 触发拉流，适配 recording.PlayProvider
// 所有协议统一走 RTSP 地址取快照 → ZLM 自动拉流 → on_publish 回调触发录制
type PlayAdapter struct {
	smsCore sms.Core
}

func NewPlayAdapter(smsCore sms.Core) recording.PlayProvider {
	return &PlayAdapter{smsCore: smsCore}
}

// TriggerStream 通过取最新快照触发 ZLM 拉流
// 构造 RTSP 本地回环地址，调 ZLM getSnap（expire=1 强制刷新，timeout=30s）
// ZLM 收到 getSnap 请求后若流不存在会自动拉流 → 触发 on_publish webhook → 录制闭环
func (a *PlayAdapter) TriggerStream(ctx context.Context, info recording.ChannelInfo) error {
	ms, err := a.smsCore.GetDefaultMediaServer()
	if err != nil {
		return err
	}

	rtspURL := fmt.Sprintf("rtsp://127.0.0.1:%d/%s/%s", ms.Ports.RTSP, info.App, info.Stream)

	_, err = a.smsCore.GetSnapshot(ms, sms.GetSnapRequest{
		URL:        rtspURL,
		TimeoutSec: 30,
		ExpireSec:  1,
		Stream:     info.ID,
	})
	return err
}
