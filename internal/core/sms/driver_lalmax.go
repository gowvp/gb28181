package sms

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"strconv"
	"strings"

	"github.com/gowvp/owl/pkg/lalmax"
	"github.com/gowvp/owl/pkg/zlm"
)

const (
	PullTimeoutMs = 10000
	PullRetryNum  = 3
)

var _ Driver = (*LalmaxDriver)(nil)

type LalmaxDriver struct {
	engine lalmax.Engine
}

// GetStreamLiveAddr implements Driver.
func (l *LalmaxDriver) GetStreamLiveAddr(ctx context.Context, ms *MediaServer, httpPrefix string, host string, app string, stream string, token string) StreamLiveAddr {
	var out StreamLiveAddr
	out.Label = "StreamSVR"
	wsPrefix := strings.Replace(strings.Replace(httpPrefix, "https", "wss", 1), "http", "ws", 1)
	out.WSFLV = fmt.Sprintf("%s/proxy/sms/%s.flv?token=%s", wsPrefix, stream, token)
	out.HTTPFLV = fmt.Sprintf("%s/proxy/sms/%s.flv?token=%s", httpPrefix, stream, token)
	out.HLS = fmt.Sprintf("%s/proxy/sms/%s/hls.fmp4.m3u8?token=%s", httpPrefix, stream, token)
	rtcPrefix := strings.Replace(strings.Replace(httpPrefix, "https", "webrtc", 1), "http", "webrtc", 1)
	out.WebRTC = fmt.Sprintf("%s/proxy/sms/index/api/webrtc?app=%s&stream=%s&type=play&token=%s", rtcPrefix, app, stream, token)
	out.RTMP = fmt.Sprintf("rtmp://%s:%d/%s", host, ms.Ports.RTMP, stream)
	out.RTSP = fmt.Sprintf("rtsp://%s:%d/%s", host, ms.Ports.RTSP, stream)
	return out
}

// AddStreamProxy implements Driver.
func (l *LalmaxDriver) AddStreamProxy(ctx context.Context, ms *MediaServer, req *AddStreamProxyRequest) (*zlm.AddStreamProxyResponse, error) {
	engine := l.withConfig(ms)
	resp, err := engine.CtrlStartRelayPull(ctx, lalmax.ApiCtrlStartRelayPullReq{
		StreamName:    req.Stream,
		Url:           req.URL,
		PullTimeoutMs: PullTimeoutMs,
		PullRetryNum:  PullRetryNum,
		RtspMode:      req.RTPType,
	})
	if err != nil {
		return nil, err
	}
	var result zlm.AddStreamProxyResponse
	result.Data.Key = resp.Data.SessionId
	return &result, nil
}

// CloseRTPServer implements Driver.
func (l *LalmaxDriver) CloseRTPServer(ctx context.Context, ms *MediaServer, req *zlm.CloseRTPServerRequest) (*zlm.CloseRTPServerResponse, error) {
	panic("unimplemented")
}

// CloseStreams lalmax 暂不支持关闭流功能
func (l *LalmaxDriver) CloseStreams(ctx context.Context, ms *MediaServer, req *zlm.CloseStreamsRequest) (*zlm.CloseStreamsResponse, error) {
	return nil, fmt.Errorf("lalmax 暂不支持关闭流功能")
}

// Connect implements Driver.
func (l *LalmaxDriver) Connect(ctx context.Context, ms *MediaServer) error {
	engine := l.withConfig(ms)
	resp, err := engine.GetServerConfig(ctx)
	if err != nil {
		return err
	}

	http := ms.Ports.HTTP
	ms.Ports.FLV = http
	ms.Ports.WsFLV = http
	rtmp, err := net.ResolveTCPAddr("tcp", resp.RtmpConfig.Addr)
	if err != nil {
		return err
	}
	ms.Ports.RTMP = rtmp.Port
	rtmps, err := net.ResolveTCPAddr("tcp", resp.RtmpConfig.RtmpsAddr)
	if err != nil {
		return err
	}
	ms.Ports.RTMPs = rtmps.Port

	rtsp, err := net.ResolveTCPAddr("tcp", resp.RtspConfig.Addr)
	if err != nil {
		return err
	}
	ms.Ports.RTSP = rtsp.Port

	ms.HookAliveInterval = 10
	ms.Status = true
	return nil
}

// GetSnapshot implements Driver.
func (l *LalmaxDriver) GetSnapshot(ctx context.Context, ms *MediaServer, req *GetSnapRequest) ([]byte, error) {
	engine := l.withConfig(ms)
	return engine.GetKeyFrameImage(ctx, req.Stream)
}

// OpenRTPServer implements Driver.
func (l *LalmaxDriver) OpenRTPServer(ctx context.Context, ms *MediaServer, req *zlm.OpenRTPServerRequest) (*zlm.OpenRTPServerResponse, error) {
	engine := l.withConfig(ms)

	resp, err := engine.ApiCtrlStartRtpPub(ctx, lalmax.ApiCtrlStartRtpPubReq{
		StreamName:      req.StreamID,
		Port:            req.Port,
		TimeoutMs:       PullTimeoutMs,
		IsTcpFlag:       int(req.TCPMode),
		IsWaitKeyFrame:  0,
		IsTcpActive:     false,
		DebugDumpPacket: "",
	})
	if err != nil {
		return nil, err
	}
	return &zlm.OpenRTPServerResponse{
		Port: resp.Data.Port,
	}, nil
}

// Ping implements Driver.
func (l *LalmaxDriver) Ping(ctx context.Context, ms *MediaServer) error {
	return nil
}

// Protocol implements Driver.
func (l *LalmaxDriver) Protocol() string {
	return ProtocolLalmax
}

// Setup implements Driver.
func (l *LalmaxDriver) Setup(ctx context.Context, ms *MediaServer, webhookURL string) error {
	engine := l.withConfig(ms)

	ports := strings.Split(ms.RTPPortRange, "-")
	var minPort, maxPort int
	if len(ports) == 2 {
		minPort, _ = strconv.Atoi(ports[0])
		maxPort, _ = strconv.Atoi(ports[1])
	}
	if err := engine.SetHttpNotifyConfig(ctx, lalmax.HttpNotifyConfig{
		Enable:               true,
		KeepaliveIntervalSec: ms.HookAliveInterval,
		OnKeepalive:          fmt.Sprintf("%s/on_server_keepalive", webhookURL),
		// OnPubStart:              webhookURL,
		// OnPubStop:               webhookURL,
		OnSubStartWithoutStream: fmt.Sprintf("%s/on_stream_not_found", webhookURL),
		OnStreamChanged:         fmt.Sprintf("%s/on_stream_changed", webhookURL),
		ClientSize:              50,
	}, lalmax.MediaConfig{
		ListenPort:            minPort,
		MultiPortMaxIncrement: maxPort - minPort,
	}); err != nil {
		return err
	}
	slog.InfoContext(ctx, "Lalmax 服务节点配置设置成功")
	return nil
}

func NewLalmaxDriver() *LalmaxDriver {
	return &LalmaxDriver{
		engine: lalmax.NewEngine(),
	}
}

func (l *LalmaxDriver) withConfig(ms *MediaServer) lalmax.Engine {
	url := fmt.Sprintf("http://%s:%d", ms.IP, ms.Ports.HTTP)
	return l.engine.SetConfig(lalmax.Config{
		URL:    url,
		Secret: ms.Secret,
	})
}

// StartRecord lalmax 暂不支持录制功能
func (l *LalmaxDriver) StartRecord(ctx context.Context, ms *MediaServer, req *zlm.StartRecordRequest) (*zlm.StartRecordResponse, error) {
	return nil, fmt.Errorf("lalmax 暂不支持录制功能")
}

// StopRecord lalmax 暂不支持录制功能
func (l *LalmaxDriver) StopRecord(ctx context.Context, ms *MediaServer, req *zlm.StopRecordRequest) (*zlm.StopRecordResponse, error) {
	return nil, fmt.Errorf("lalmax 暂不支持录制功能")
}

// GetMediaList lalmax 暂不支持流列表查询
func (l *LalmaxDriver) GetMediaList(ctx context.Context, ms *MediaServer) (*zlm.GetMediaListResponse, error) {
	return &zlm.GetMediaListResponse{}, nil
}
