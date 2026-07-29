package sms

import (
	"context"

	"github.com/gowvp/owl/pkg/zlm"
)

const (
	ProtocolZLMediaKit = "zlm"
	ProtocolLalmax     = "lalmax"
)

// Driver 定义流媒体服务的通用行为
type Driver interface {
	// Protocol 返回协议/类型名称，如 "zlm", "srs"
	Protocol() string

	// Connect 测试连接并获取初始信息 (对应目前的 connection 方法中的部分逻辑)
	Connect(ctx context.Context, ms *MediaServer) error

	// Setup 下发配置到流媒体服务 (对应目前的 connection 方法中的配置逻辑)
	Setup(ctx context.Context, ms *MediaServer, webhookURL string) error

	// Ping 主动探测服务是否在线
	Ping(ctx context.Context, ms *MediaServer) error

	// Stream Operations
	OpenRTPServer(ctx context.Context, ms *MediaServer, req *zlm.OpenRTPServerRequest) (*zlm.OpenRTPServerResponse, error)
	CloseRTPServer(ctx context.Context, ms *MediaServer, req *zlm.CloseRTPServerRequest) (*zlm.CloseRTPServerResponse, error)
	CloseStreams(ctx context.Context, ms *MediaServer, req *zlm.CloseStreamsRequest) (*zlm.CloseStreamsResponse, error)
	AddStreamProxy(ctx context.Context, ms *MediaServer, req *AddStreamProxyRequest) (*zlm.AddStreamProxyResponse, error)
	GetSnapshot(ctx context.Context, ms *MediaServer, req *GetSnapRequest) ([]byte, error)

	GetStreamLiveAddr(ctx context.Context, ms *MediaServer, httpPrefix, host, app, stream, token string) StreamLiveAddr
	GetMediaInfo(ctx context.Context, ms *MediaServer, app, stream string) ([]zlm.MediaItem, error)

	// Recording Operations
	StartRecord(ctx context.Context, ms *MediaServer, req *zlm.StartRecordRequest) (*zlm.StartRecordResponse, error)
	StopRecord(ctx context.Context, ms *MediaServer, req *zlm.StopRecordRequest) (*zlm.StopRecordResponse, error)
	GetMediaList(ctx context.Context, ms *MediaServer) (*zlm.GetMediaListResponse, error)
}
