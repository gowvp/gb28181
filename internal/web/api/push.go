package api

import (
	"fmt"
	"log/slog"

	"github.com/gin-gonic/gin"
	"github.com/gowvp/owl/internal/conf"
	"github.com/gowvp/owl/internal/core/push"
	"github.com/gowvp/owl/internal/core/sms"
	"github.com/ixugo/goddd/pkg/hook"
	"github.com/ixugo/goddd/pkg/web"
)

type PushAPI struct {
	pushCore push.Core
	smsCore  sms.Core
	conf     *conf.Bootstrap
}

// NewPushAPI 媒体相关接口
func NewPushAPI(mc push.Core, sc sms.Core, conf *conf.Bootstrap) PushAPI {
	return PushAPI{pushCore: mc, smsCore: sc, conf: conf}
}

// streamPushIDInput 推流 ID 路径参数
type streamPushIDInput = push.GetStreamPushInput

// updateStreamPushInput 更新推流的请求参数（路径 ID + 请求体）
type updateStreamPushInput = push.UpdateStreamPushInput

func registerPushAPI(g gin.IRouter, api PushAPI, handler ...gin.HandlerFunc) {
	group := g.Group("/stream_pushs", handler...)
	group.GET("", web.WrapH(api.listStreamPushs))
	group.GET("/:id", web.WrapH(api.getStreamPush))
	group.PUT("/:id", web.WrapH(api.updateStreamPush))
	group.POST("", web.WrapH(api.createStreamPush))
	group.DELETE("/:id", web.WrapH(api.deleteStreamPush))
}

// listStreamPushs 分页查询推流列表
func (a PushAPI) listStreamPushs(c *gin.Context, in *push.ListStreamPushInput) (*web.PageOutput[*push.ListStreamPushOutputItem], error) {
	items, total, err := a.pushCore.ListStreamPushs(c.Request.Context(), in)
	if err != nil {
		return nil, err
	}

	cacheFn := func(s string) (*sms.MediaServer, error) {
		v, err := a.smsCore.GetMediaServer(c.Request.Context(), s)
		if err != nil {
			slog.ErrorContext(c.Request.Context(), "GetMediaServer", "err", err)
		}
		return v, err
	}

	out := make([]*push.ListStreamPushOutputItem, len(items))
	for i, item := range items {
		rtmpAddrs := []string{"unknow"}
		mediaID := item.MediaServerID
		if mediaID == "" {
			mediaID = sms.DefaultMediaServerID
		}
		if svr, _ := cacheFn(mediaID); svr != nil {
			port := svr.Ports.RTMP
			if port == 0 {
				port = 1935
			}
			addr := fmt.Sprintf("rtmp://%s:%d/%s/%s", web.GetHost(c.Request), port, item.App, item.Stream)
			if !item.IsAuthDisabled {
				addr += fmt.Sprintf("?sign=%s", hook.MD5(a.conf.Server.RTMPSecret))
			}
			rtmpAddrs[0] = addr
		}

		out[i] = &push.ListStreamPushOutputItem{
			StreamPush: *item,
			PushAddrs:  rtmpAddrs,
		}
	}
	return &web.PageOutput[*push.ListStreamPushOutputItem]{Items: out, Total: total}, err
}

// getStreamPush 按 ID 查询推流
func (a PushAPI) getStreamPush(c *gin.Context, in *streamPushIDInput) (*push.StreamPush, error) {
	return a.pushCore.GetStreamPush(c.Request.Context(), in.ID)
}

// updateStreamPush 更新推流信息
func (a PushAPI) updateStreamPush(c *gin.Context, in *updateStreamPushInput) (*push.StreamPush, error) {
	return a.pushCore.UpdateStreamPush(c.Request.Context(), in)
}

// createStreamPush 创建推流
func (a PushAPI) createStreamPush(c *gin.Context, in *push.CreateStreamPushInput) (*push.StreamPush, error) {
	return a.pushCore.CreateStreamPush(c.Request.Context(), in)
}

// deleteStreamPush 删除推流
func (a PushAPI) deleteStreamPush(c *gin.Context, in *streamPushIDInput) (*push.StreamPush, error) {
	return a.pushCore.DeleteStreamPush(c.Request.Context(), in.ID)
}
