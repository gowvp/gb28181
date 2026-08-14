package gbs

import (
	"context"
	"log/slog"
	"strings"

	wsnotify "github.com/gowvp/owl/internal/notify"
	"github.com/gowvp/owl/pkg/gbs/sip"
	"github.com/gowvp/owl/pkg/zlm"

	sms "github.com/gowvp/owl/internal/core/sms"
)

// handleBYE 处理摄像头主动发起的 BYE 请求。
// 摄像头在 INVITE 建立媒体会话后，若 RTP 连接失败或内部异常，
// 会主动发送 BYE 终止会话。系统需据此清理 RTP 资源和播放状态。
func (g *GB28181API) handleBYE(ctx *sip.Context) {
	callID, ok := ctx.Request.CallID()
	if !ok {
		ctx.Log.Warn("BYE 缺少 CallID")
		ctx.String(400, "Bad Request")
		return
	}
	callIDStr := callID.String()
	ctx.Log.Debug("收到设备 BYE", "call_id", callIDStr, "from", ctx.Source)

	// 先回 200 OK，再同步清理（handler 已在独立 goroutine 中运行）
	ctx.String(200, "OK")

	var foundKey string
	g.streams.Range(func(key string, stream *Streams) bool {
		if stream.Resp == nil {
			return true
		}
		if respCallID, ok := stream.Resp.CallID(); ok && respCallID.String() == callIDStr {
			foundKey = key
			return false
		}
		return true
	})

	if foundKey == "" {
		ctx.Log.Debug("BYE 未匹配到活跃会话", "call_id", callIDStr)
		return
	}

	g.streams.Delete(foundKey)
	ctx.Log.Debug("BYE 已清理 streams 条目", "key", foundKey)

	// key 格式: "play:deviceID:channelID"
	parts := strings.SplitN(foundKey, ":", 3)
	if len(parts) != 3 {
		return
	}

	deviceID, channelID := parts[1], parts[2]
	ctx.Log.Warn("设备主动 BYE，终止推流", "device_id", deviceID, "channel_id", channelID, "call_id", callIDStr)
	g.svr.gb.core.UpdatePlaying(context.TODO(), deviceID, channelID, false)
	wsnotify.IPCWarn("设备主动终止推流(BYE)", deviceID, channelID)

	svr, err := g.svr.mediaService.GetMediaServer(context.Background(), sms.DefaultMediaServerID)
	if err != nil {
		slog.Warn("BYE closeRTP: 获取流媒体服务器失败", "err", err)
		return
	}
	if _, err := g.sms.CloseRTPServer(svr, zlm.CloseRTPServerRequest{StreamID: channelID}); err != nil {
		slog.Warn("BYE closeRTP: 关闭失败", "stream_id", channelID, "err", err)
	}
}
