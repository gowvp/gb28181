package api

import (
	"log/slog"
	"net/url"

	"github.com/gin-gonic/gin"
	"github.com/gowvp/gb28181/internal/conf"
	"github.com/gowvp/gb28181/internal/core/bz"
	"github.com/gowvp/gb28181/internal/core/ipc"
	"github.com/gowvp/gb28181/internal/core/push"
	"github.com/gowvp/gb28181/internal/core/sms"
	"github.com/gowvp/gb28181/pkg/gbs"
	"github.com/ixugo/goddd/pkg/web"
)

type WebHookAPI struct {
	smsCore     sms.Core
	mediaCore   push.Core
	gb28181Core ipc.Core
	conf        *conf.Bootstrap
	log         *slog.Logger
	gbs         *gbs.Server
	uc          *Usecase

	protocols map[string]ipc.Protocoler
}

func NewWebHookAPI(core sms.Core, mediaCore push.Core, conf *conf.Bootstrap, gbs *gbs.Server, gb28181 ipc.Core, protocols map[string]ipc.Protocoler) WebHookAPI {
	return WebHookAPI{
		smsCore:     core,
		mediaCore:   mediaCore,
		conf:        conf,
		log:         slog.With("hook", "zlm"),
		gbs:         gbs,
		gb28181Core: gb28181,
		protocols:   protocols,
	}
}

func registerZLMWebhookAPI(r gin.IRouter, api WebHookAPI, handler ...gin.HandlerFunc) {
	{
		group := r.Group("/webhook", handler...)
		group.POST("/on_server_keepalive", web.WrapH(api.onServerKeepalive))
		group.POST("/on_stream_changed", web.WrapH(api.onStreamChanged))
		group.POST("/on_publish", web.WrapH(api.onPublish))
		group.POST("/on_play", web.WrapH(api.onPlay))
		group.POST("/on_stream_none_reader", web.WrapH(api.onStreamNoneReader))
		group.POST("/on_rtp_server_timeout", web.WrapH(api.onRTPServerTimeout))
		group.POST("/on_stream_not_found", web.WrapH(api.onStreamNotFound))
	}
}

// onServerKeepalive 服务器定时上报时间，上报间隔可配置，默认 10s 上报一次
// https://docs.zlmediakit.com/zh/guide/media_server/web_hook_api.html#_16%E3%80%81on-server-keepalive
func (w WebHookAPI) onServerKeepalive(_ *gin.Context, in *onServerKeepaliveInput) (DefaultOutput, error) {
	w.smsCore.Keepalive(in.MediaServerID)
	return newDefaultOutputOK(), nil
}

// onPublish rtsp/rtmp/rtp 推流鉴权事件。
// https://docs.zlmediakit.com/zh/guide/media_server/web_hook_api.html#_7%E3%80%81on-publish
func (w WebHookAPI) onPublish(c *gin.Context, in *onPublishInput) (*onPublishOutput, error) {
	w.log.Info("webhook onPublish", "app", in.App, "stream", in.Stream, "schema", in.Schema, "mediaServerID", in.MediaServerID)
	if in.Schema == "rtmp" {
		params, err := url.ParseQuery(in.Params)
		if err != nil {
			return &onPublishOutput{DefaultOutput: DefaultOutput{Code: 1, Msg: err.Error()}}, nil
		}
		sign := params.Get("sign")
		if err := w.mediaCore.Publish(c.Request.Context(), push.PublishInput{
			App:           in.App,
			Stream:        in.Stream,
			MediaServerID: in.MediaServerID,
			Sign:          sign,
			Secret:        w.conf.Server.RTMPSecret,
			Session:       params.Get("session"),
		}); err != nil {
			return &onPublishOutput{DefaultOutput: DefaultOutput{Code: 1, Msg: err.Error()}}, nil
		}
	}
	return &onPublishOutput{
		DefaultOutput: newDefaultOutputOK(),
	}, nil
}

// onStreamChanged rtsp/rtmp 流注册或注销时触发此事件；此事件对回复不敏感。
// https://docs.zlmediakit.com/zh/guide/media_server/web_hook_api.html#_12%E3%80%81on-stream-changed
func (w WebHookAPI) onStreamChanged(c *gin.Context, in *onStreamChangedInput) (DefaultOutput, error) {
	w.log.InfoContext(c.Request.Context(), "webhook onStreamChanged", "app", in.App, "stream", in.Stream, "schema", in.Schema, "mediaServerID", in.MediaServerID, "regist", in.Regist)
	if in.Regist || in.Schema != "rtmp" {
		return newDefaultOutputOK(), nil
	}

	// TODO: 待重构
	if bz.IsRTMP(in.Stream) {
		if err := w.mediaCore.UnPublish(c.Request.Context(), in.App, in.Stream); err != nil {
			w.log.ErrorContext(c.Request.Context(), "webhook onStreamChanged", "err", err)
		}
		return newDefaultOutputOK(), nil
	}

	r := ipc.GetType(in.Stream)
	protocol, ok := w.protocols[r]
	if ok {
		if err := protocol.OnStreamChanged(c.Request.Context(), in.Stream); err != nil {
			slog.ErrorContext(c.Request.Context(), "webhook onStreamChanged", "err", err)
		}
	}
	return newDefaultOutputOK(), nil
}

// onPlay rtsp/rtmp/http-flv/ws-flv/hls 播放触发播放器身份验证事件。
// 播放流时会触发此事件。如果流不存在，则首先触发 on_play 事件，然后触发 on_stream_not_found 事件。
// 播放rtsp流时，如果该流开启了rtsp专用认证（on_rtsp_realm），则不会触发on_play事件。
// https://docs.zlmediakit.com/guide/media_server/web_hook_api.html#_6-on-play
func (w WebHookAPI) onPlay(c *gin.Context, in *onPublishInput) (DefaultOutput, error) {
	return newDefaultOutputOK(), nil

	switch in.Schema {
	case "rtmp":
		params, err := url.ParseQuery(in.Params)
		if err != nil {
			w.log.InfoContext(c.Request.Context(), "webhook onPlay", "err", err)
			return DefaultOutput{Code: 1, Msg: err.Error()}, nil
		}
		session := params.Get("session")
		if err := w.mediaCore.OnPlay(c.Request.Context(), push.OnPlayInput{
			App:     in.App,
			Stream:  in.Stream,
			Session: session,
		}); err != nil {
			w.log.InfoContext(c.Request.Context(), "webhook onPlay", "err", err)
			return DefaultOutput{Code: 1, Msg: err.Error()}, nil
		}
	case "rtsp":
	}

	return newDefaultOutputOK(), nil
}

// onStreamNoneReader 流无人观看时事件，用户可以通过此事件选择是否关闭无人看的流。
// 一个直播流注册上线了，如果一直没人观看也会触发一次无人观看事件，触发时的协议 schema 是随机的，
// 看哪种协议最晚注册(一般为 hls)。
// 后续从有人观看转为无人观看，触发协议 schema 为最后一名观看者使用何种协议。
// 目前 mp4/hls 录制不当做观看人数(mp4 录制可以通过配置文件 mp4_as_player 控制，
// 但是 rtsp/rtmp/rtp 转推算观看人数，也会触发该事件。
// https://docs.zlmediakit.com/zh/guide/media_server/web_hook_api.html#_12%E3%80%81on-stream-changed
func (w WebHookAPI) onStreamNoneReader(c *gin.Context, in *onStreamNoneReaderInput) (onStreamNoneReaderOutput, error) {
	// rtmp 无人观看时，也允许推流
	w.log.InfoContext(c.Request.Context(), "webhook onStreamNoneReader", "app", in.App, "stream", in.Stream, "mediaServerID", in.MediaServerID)
	// 存在录像计划时，不关闭流
	return onStreamNoneReaderOutput{Close: true}, nil
}

// onRTPServerTimeout RTP 服务器超时事件
// 调用 openRtpServer 接口，rtp server 长时间未收到数据,执行此 web hook,对回复不敏感
// https://docs.zlmediakit.com/zh/guide/media_server/web_hook_api.html#_17%E3%80%81on-rtp-server-timeout
func (w WebHookAPI) onRTPServerTimeout(c *gin.Context, in *onRTPServerTimeoutInput) (DefaultOutput, error) {
	w.log.InfoContext(c.Request.Context(), "webhook onRTPServerTimeout", "local_port", in.LocalPort, "ssrc", in.SSRC, "stream_id", in.StreamID, "mediaServerID", in.MediaServerID)
	return newDefaultOutputOK(), nil
}

// onStreamNotFound 流不存在事件
// TODO: 重启后立即播放，会出发 "channel not exist" 待处理
func (w WebHookAPI) onStreamNotFound(c *gin.Context, in *onStreamNotFoundInput) (DefaultOutput, error) {
	w.log.InfoContext(c.Request.Context(), "webhook onStreamNotFound", "app", in.App, "stream", in.Stream, "schema", in.Schema, "mediaServerID", in.MediaServerID)
	if !(in.Schema == "rtmp" || in.Schema == "rtsp") {
		return newDefaultOutputOK(), nil
	}

	r := ipc.GetType(in.Stream)
	protocol, ok := w.protocols[r]
	if ok {
		if err := protocol.OnStreamNotFound(c.Request.Context(), in.App, in.Stream); err != nil {
			slog.ErrorContext(c.Request.Context(), "webhook onStreamNotFound", "err", err)
		}
	}

	return newDefaultOutputOK(), nil
}
