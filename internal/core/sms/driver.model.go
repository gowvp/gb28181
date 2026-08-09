package sms

import "github.com/gowvp/owl/pkg/zlm"

type AddStreamProxyRequest struct {
	App     string `json:"app"`      // 添加的流的应用名，例如 live
	Stream  string `json:"stream"`   // 添加的流的 id 名，例如 test
	URL     string `json:"url"`      // 拉流地址，例如 rtmp://live.hkstv.hk.lxdns.com/live/hks2
	RTPType int    `json:"rtp_type"` // rtsp 拉流时，拉流方式，0：tcp，1：udp，2：组播

	// Vhost         string  `json:"vhost"`                     // 添加的流的虚拟主机，例如__defaultVhost__
	// RetryCount    int     `json:"retry_count"`               // 拉流重试次数，默认为-1 无限重试
	// TimeoutSec    float32 `json:"timeout_sec"`               // 拉流超时时间，单位秒，float 类型
	// EnableHLS     *bool   `json:"enable_hls,omitempty"`      // 是否转换成 hls-mpegts 协议
	// EnableHLSFMP4 *bool   `json:"enable_hls_fmp4,omitempty"` // 是否转换成 hls-fmp4 协议
	// EnableMP4     *bool   `json:"enable_mp4,omitempty"`      // 是否允许 mp4 录制
	// EnableRTSP    *bool   `json:"enable_rtsp,omitempty"`     // 是否转 rtsp 协议
	// EnableRTMP    *bool   `json:"enable_rtmp,omitempty"`     // 是否转 rtmp/flv 协议
	// EnableTS      *bool   `json:"enable_ts,omitempty"`       // 是否转 http-ts/ws-ts 协议
	// EnableFMP4    *bool   `json:"enable_fmp4,omitempty"`     // 是否转 http-fmp4/ws-fmp4 协议
	// HLSDemand     *bool   `json:"hls_demand,omitempty"`      // 该协议是否有人观看才生成
	// RTSPDemand    *bool   `json:"rtsp_demand,omitempty"`     // 该协议是否有人观看才生成
	// RTMPDemand    *bool   `json:"rtmp_demand,omitempty"`     // 该协议是否有人观看才生成
	// TSDemand      *bool   `json:"ts_demand,omitempty"`       // 该协议是否有人观看才生成
	// FMP4Demand    *bool   `json:"fmp4_demand,omitempty"`     // 该协议是否有人观看才生成
	// EnableAudio   *bool   `json:"enable_audio,omitempty"`    // 转协议时是否开启音频
	// AddMuteAudio  *bool   `json:"add_mute_audio,omitempty"`  // 转协议时，无音频是否添加静音 aac 音频
	// MP4SavePath   *string `json:"mp4_save_path,omitempty"`   // mp4 录制文件保存根目录，置空使用默认
	// MP4MaxSecond  *int    `json:"mp4_max_second,omitempty"`  // mp4 录制切片大小，单位秒
	// MP4AsPlayer   *bool   `json:"mp4_as_player,omitempty"`   // MP4 录制是否当作观看者参与播放人数计数
	// HLSSavePath   *string `json:"hls_save_path,omitempty"`   // hls 文件保存保存根目录，置空使用默认
	// ModifyStamp   *int    `json:"modify_stamp,omitempty"`    // 该流是否开启时间戳覆盖(0:绝对时间戳/1:系统时间戳/2:相对时间戳)
	// AutoClose     *bool   `json:"auto_close,omitempty"`      // 无人观看是否自动关闭流(不触发无人观看 hook)
}

type GetSnapRequest struct {
	zlm.GetSnapRequest
	// lalmax
	Stream string `json:"stream"`
}

type StreamLiveAddr struct {
	Label  string `json:"label"`
	WSFLV  string `json:"ws-flv"`
	FLV    string `json:"flv"`
	RTMP   string `json:"rtmp"`
	RTSP   string `json:"rtsp"`
	WebRTC string `json:"webrtc"`
	HLS    string `json:"hls"`
}
