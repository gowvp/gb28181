package recording

import (
	"context"
	"strings"
	"time"

	"github.com/gowvp/owl/internal/conf"
	"github.com/ixugo/goddd/pkg/orm"
)

// Storer 数据持久化聚合接口
type Storer interface {
	Begin() (orm.Tx, error)
	Recording() RecordingStorer
}

// SMSProvider 流媒体服务提供者接口，解耦录制领域与 sms 领域
type SMSProvider interface {
	StartRecord(app, stream, customPath string, maxSecond int) error
	StopRecord(app, stream string) error
	// ListRecordingStreams 批量获取所有在线流的录制状态
	// 返回 map[app/stream]bool，true 表示正在录制 MP4
	ListRecordingStreams() (map[string]bool, error)
}

// IPCProvider 通道信息提供者，解耦录制域与 ipc 域
// 用于定时同步时查询所有在线通道
type IPCProvider interface {
	ListOnlineChannels(ctx context.Context) ([]ChannelInfo, error)
}

// ChannelInfo 同步任务使用的通道信息摘要
type ChannelInfo struct {
	ID         string // 通道唯一 ID
	App        string // 应用名（如 rtp / live）
	Stream     string // 流 ID
	Type       string // 通道类型（gb28181 / onvif / rtmp / rtsp）
	RecordMode string // 录像模式（always / ai / none / 空串=always）
}

// PlayProvider 主动拉流能力，解耦录制域与播放域
// 仅用于"应录制但流不存在"时触发拉流
type PlayProvider interface {
	TriggerStream(ctx context.Context, info ChannelInfo) error
}

// Core business domain
type Core struct {
	store        Storer
	conf         *conf.ServerRecording
	smsProvider  SMSProvider
	ipcProvider  IPCProvider
	playProvider PlayProvider
	syncInterval time.Duration // 0 表示使用默认值 syncDefaultInterval
}

type Option func(*Core)

// WithSMSProvider 注入流媒体服务提供者，用于控制录制
func WithSMSProvider(provider SMSProvider) Option {
	return func(c *Core) {
		c.smsProvider = provider
	}
}

// WithConfig 注入录制配置
func WithConfig(conf *conf.ServerRecording) Option {
	return func(c *Core) {
		c.conf = conf
	}
}

// WithIPCProvider 注入通道信息提供者，用于定时同步时查询应录制的通道
func WithIPCProvider(provider IPCProvider) Option {
	return func(c *Core) {
		c.ipcProvider = provider
	}
}

// WithPlayProvider 注入拉流能力，用于流不存在时主动触发拉流
func WithPlayProvider(provider PlayProvider) Option {
	return func(c *Core) {
		c.playProvider = provider
	}
}

// WithSyncInterval 设置同步周期，用于测试时缩短等待时间
func WithSyncInterval(d time.Duration) Option {
	return func(c *Core) {
		c.syncInterval = d
	}
}

// NewCore create business domain
func NewCore(store Storer, opts ...Option) Core {
	c := Core{store: store}
	for _, opt := range opts {
		opt(&c)
	}
	return c
}

// IsEnabled 检查是否启用录制（全局开关）
// 使用反转逻辑：Disabled=false 表示启用录制
func (c Core) IsEnabled() bool {
	return c.conf != nil && !c.conf.Disabled
}

// GetFullPath 获取录像文件的完整路径
// relativePath 可能是相对于 StorageDir 的路径，也可能是完整路径
func (c Core) GetFullPath(relativePath string) string {
	if c.conf == nil || c.conf.StorageDir == "" {
		return relativePath
	}
	// 如果 relativePath 已经包含 StorageDir，直接返回
	if len(relativePath) > 0 && (relativePath[0] == '/' || strings.HasPrefix(relativePath, c.conf.StorageDir)) {
		return relativePath
	}
	return c.conf.StorageDir + "/" + relativePath
}
