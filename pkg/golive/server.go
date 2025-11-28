package golive

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"
)

// StreamType 流类型
type StreamType string

const (
	StreamTypeRTMP   StreamType = "rtmp"
	StreamTypeRTSP   StreamType = "rtsp"
	StreamTypeHLS    StreamType = "hls"
	StreamTypeFLV    StreamType = "flv"
	StreamTypeWebRTC StreamType = "webrtc"
)

// StreamInfo 流信息
type StreamInfo struct {
	App       string     `json:"app"`
	Stream    string     `json:"stream"`
	Type      StreamType `json:"type"`
	URL       string     `json:"url"`
	StartTime time.Time  `json:"start_time"`
	Bitrate   int64      `json:"bitrate,omitempty"`
	Viewers   int        `json:"viewers,omitempty"`
}

// ServerConfig Go 流媒体服务器配置
type ServerConfig struct {
	Enabled      bool   `json:"enabled" toml:"enabled"`
	RTMPPort     int    `json:"rtmp_port" toml:"rtmp_port"`         // RTMP 端口
	RTSPPort     int    `json:"rtsp_port" toml:"rtsp_port"`         // RTSP 端口
	HTTPFLVPort  int    `json:"http_flv_port" toml:"http_flv_port"` // HTTP-FLV 端口
	HLSPort      int    `json:"hls_port" toml:"hls_port"`           // HLS 端口
	WebRTCPort   int    `json:"webrtc_port" toml:"webrtc_port"`     // WebRTC 端口
	PublicIP     string `json:"public_ip" toml:"public_ip"`         // 公网 IP
	EnableAuth   bool   `json:"enable_auth" toml:"enable_auth"`     // 启用推流鉴权
	AuthSecret   string `json:"auth_secret" toml:"auth_secret"`     // 推流鉴权密钥
	HLSFragment  int    `json:"hls_fragment" toml:"hls_fragment"`   // HLS 分片时长(秒)
	HLSWindow    int    `json:"hls_window" toml:"hls_window"`       // HLS 窗口大小
	RecordPath   string `json:"record_path" toml:"record_path"`     // 录像存储路径
	EnableRecord bool   `json:"enable_record" toml:"enable_record"` // 启用录像
}

// Server Go 流媒体服务器
type Server struct {
	config     ServerConfig
	streams    sync.Map // map[string]*StreamInfo
	httpServer *http.Server
	ctx        context.Context
	cancel     context.CancelFunc
	running    bool
	mu         sync.RWMutex
}

// NewServer 创建 Go 流媒体服务器
func NewServer(config ServerConfig) *Server {
	ctx, cancel := context.WithCancel(context.Background())
	return &Server{
		config: config,
		ctx:    ctx,
		cancel: cancel,
	}
}

// Start 启动服务器
func (s *Server) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return fmt.Errorf("server already running")
	}

	if !s.config.Enabled {
		slog.Info("Go streaming server is disabled")
		return nil
	}

	slog.Info("Starting Go streaming server",
		"rtmp_port", s.config.RTMPPort,
		"http_flv_port", s.config.HTTPFLVPort,
		"hls_port", s.config.HLSPort,
	)

	// 启动 HTTP 服务 (HTTP-FLV/HLS)
	if s.config.HTTPFLVPort > 0 {
		mux := http.NewServeMux()
		mux.HandleFunc("/live/", s.handleFLV)
		mux.HandleFunc("/hls/", s.handleHLS)
		mux.HandleFunc("/api/streams", s.handleListStreams)
		mux.HandleFunc("/api/stream/", s.handleStreamInfo)

		s.httpServer = &http.Server{
			Addr:    fmt.Sprintf(":%d", s.config.HTTPFLVPort),
			Handler: mux,
		}

		go func() {
			if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				slog.Error("HTTP server error", "err", err)
			}
		}()
	}

	// TODO: 启动 RTMP 服务器
	// 这里可以集成 LAL 或其他纯 Go RTMP 库
	// 目前提供框架结构，实际的 RTMP 处理需要引入相应的库

	s.running = true
	slog.Info("Go streaming server started")

	return nil
}

// Stop 停止服务器
func (s *Server) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return nil
	}

	s.cancel()

	if s.httpServer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := s.httpServer.Shutdown(ctx); err != nil {
			slog.Error("HTTP server shutdown error", "err", err)
		}
	}

	s.running = false
	slog.Info("Go streaming server stopped")

	return nil
}

// IsRunning 检查服务器是否运行中
func (s *Server) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}

// AddStream 添加流
func (s *Server) AddStream(info *StreamInfo) {
	key := fmt.Sprintf("%s/%s", info.App, info.Stream)
	s.streams.Store(key, info)
	slog.Info("Stream added", "app", info.App, "stream", info.Stream)
}

// RemoveStream 移除流
func (s *Server) RemoveStream(app, stream string) {
	key := fmt.Sprintf("%s/%s", app, stream)
	s.streams.Delete(key)
	slog.Info("Stream removed", "app", app, "stream", stream)
}

// GetStream 获取流信息
func (s *Server) GetStream(app, stream string) (*StreamInfo, bool) {
	key := fmt.Sprintf("%s/%s", app, stream)
	v, ok := s.streams.Load(key)
	if !ok {
		return nil, false
	}
	return v.(*StreamInfo), true
}

// ListStreams 列出所有流
func (s *Server) ListStreams() []*StreamInfo {
	var result []*StreamInfo
	s.streams.Range(func(_, value any) bool {
		if info, ok := value.(*StreamInfo); ok {
			result = append(result, info)
		}
		return true
	})
	return result
}

// GetPlayURLs 获取播放地址
func (s *Server) GetPlayURLs(app, stream string) map[StreamType]string {
	urls := make(map[StreamType]string)

	if s.config.PublicIP == "" {
		s.config.PublicIP = "localhost"
	}

	if s.config.RTMPPort > 0 {
		urls[StreamTypeRTMP] = fmt.Sprintf("rtmp://%s:%d/%s/%s", s.config.PublicIP, s.config.RTMPPort, app, stream)
	}
	if s.config.RTSPPort > 0 {
		urls[StreamTypeRTSP] = fmt.Sprintf("rtsp://%s:%d/%s/%s", s.config.PublicIP, s.config.RTSPPort, app, stream)
	}
	if s.config.HTTPFLVPort > 0 {
		urls[StreamTypeFLV] = fmt.Sprintf("http://%s:%d/live/%s/%s.flv", s.config.PublicIP, s.config.HTTPFLVPort, app, stream)
	}
	if s.config.HLSPort > 0 {
		urls[StreamTypeHLS] = fmt.Sprintf("http://%s:%d/hls/%s/%s.m3u8", s.config.PublicIP, s.config.HLSPort, app, stream)
	}

	return urls
}

// handleFLV 处理 HTTP-FLV 请求
func (s *Server) handleFLV(w http.ResponseWriter, r *http.Request) {
	// TODO: 实现 FLV 流传输
	// 这里需要集成实际的 FLV 封装和流传输逻辑
	w.Header().Set("Content-Type", "video/x-flv")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	// 简单返回 501，表示功能待实现
	http.Error(w, "FLV streaming not fully implemented. Please use ZLMediaKit or integrate LAL.", http.StatusNotImplemented)
}

// handleHLS 处理 HLS 请求
func (s *Server) handleHLS(w http.ResponseWriter, r *http.Request) {
	// TODO: 实现 HLS 流传输
	// 这里需要集成实际的 HLS 分片和播放列表生成逻辑
	http.Error(w, "HLS streaming not fully implemented. Please use ZLMediaKit or integrate LAL.", http.StatusNotImplemented)
}

// handleListStreams 处理流列表请求
func (s *Server) handleListStreams(w http.ResponseWriter, r *http.Request) {
	streams := s.ListStreams()
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"streams":%d}`, len(streams))
}

// handleStreamInfo 处理流信息请求
func (s *Server) handleStreamInfo(w http.ResponseWriter, r *http.Request) {
	// TODO: 解析路径获取 app/stream
	http.Error(w, "Not implemented", http.StatusNotImplemented)
}

// DefaultServerConfig 默认 Go 流媒体服务器配置
func DefaultServerConfig() ServerConfig {
	return ServerConfig{
		Enabled:      false,
		RTMPPort:     1936,
		RTSPPort:     8555,
		HTTPFLVPort:  8088,
		HLSPort:      8088,
		WebRTCPort:   0,
		PublicIP:     "",
		EnableAuth:   false,
		AuthSecret:   "",
		HLSFragment:  2,
		HLSWindow:    6,
		RecordPath:   "./records",
		EnableRecord: false,
	}
}
