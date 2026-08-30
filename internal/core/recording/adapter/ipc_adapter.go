package adapter

import (
	"context"

	"github.com/gowvp/owl/internal/core/ipc"
	"github.com/gowvp/owl/internal/core/recording"
)

var _ recording.IPCProvider = (*IPCAdapter)(nil)

// IPCAdapter 桥接 ipc.Core → recording.IPCProvider
// 为录制同步提供所有在线通道列表
type IPCAdapter struct {
	ipcCore ipc.Core
}

func NewIPCAdapter(ipcCore ipc.Core) recording.IPCProvider {
	return &IPCAdapter{ipcCore: ipcCore}
}

// ListOnlineChannels 查询所有在线通道，包含 RecordMode 供同步逻辑分类
func (a *IPCAdapter) ListOnlineChannels(ctx context.Context) ([]recording.ChannelInfo, error) {
	channels, _, err := a.ipcCore.ListChannels(ctx, &ipc.FindChannelInput{
		Page: 1, Size: 9999,
		IsOnline: "true",
	})
	if err != nil {
		return nil, err
	}

	result := make([]recording.ChannelInfo, 0, len(channels))
	for _, ch := range channels {
		result = append(result, recording.ChannelInfo{
			ID:         ch.ID,
			App:        ch.GetApp(),
			Stream:     ch.GetStream(),
			Type:       ch.GetType(),
			RecordMode: ch.Ext.GetRecordMode(),
		})
	}
	return result, nil
}
