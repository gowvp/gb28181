package proxy

import "github.com/ixugo/goddd/pkg/web"

// ListStreamProxyInput 分页查询参数
type ListStreamProxyInput struct {
	web.PagerFilter
	App                       string `form:"app"`                          // 应用名
	Stream                    string `form:"stream"`                       // 流 id
	MediaServerID             string `form:"media_server_id"`              // 媒体服务器 id
	SourceURL                 string `form:"source_url"`                   // 原始 url
	TimeoutS                  int    `form:"timeout_s"`                    // 超时时间(秒)
	Transport                 int    `form:"transport"`                    // rtsp 拉流方式(0:udp;1:tcp)
	Enabled                   bool   `form:"enabled"`                      // 是否启用
	EnabledAudio              bool   `form:"enabled_audio"`                // 是否启用音频
	EnabledRemoveNoneReader   bool   `form:"enabled_remove_none_reader"`   // 是否无人观看时删除
	EnabledDisabledNoneReader bool   `form:"enabled_disabled_none_reader"` // 是否无人观看时禁用
	StreamKey                 string `form:"stream_key"`                   // zlm 返回的 key
	Pulling                   bool   `form:"pulling"`                      // 拉流状态
}

// GetStreamProxyInput 按 ID 查询参数
type GetStreamProxyInput struct {
	ID string `uri:"id" binding:"required"`
}

// CreateStreamProxyInput 新增拉流代理参数
type CreateStreamProxyInput struct {
	App    string `json:"app"`    // 应用名
	Stream string `json:"stream"` // 流 id
	// MediaServerID             string `json:"media_server_id"`
	SourceURL                 string `json:"source_url"`                   // 原始 url
	TimeoutS                  int    `json:"timeout_s"`                    // 超时时间(秒)
	Transport                 int    `json:"transport"`                    // rtsp 拉流方式(0:udp;1:tcp)
	Enabled                   bool   `json:"enabled"`                      // 是否启用
	EnabledAudio              bool   `json:"enabled_audio"`                // 是否启用音频
	EnabledRemoveNoneReader   bool   `json:"enabled_remove_none_reader"`   // 是否无人观看时删除
	EnabledDisabledNoneReader bool   `json:"enabled_disabled_none_reader"` // 是否无人观看时禁用
	StreamKey                 string `json:"stream_key"`                   // zlm 返回的 key
	Pulling                   bool   `json:"pulling"`                      // 拉流状态
}

// UpdateStreamProxyInput 编辑拉流代理参数
type UpdateStreamProxyInput struct {
	ID                        string `uri:"id"`
	App                       string `json:"app"`                          // 应用名
	Stream                    string `json:"stream"`                       // 流 id
	MediaServerID             string `json:"media_server_id"`              // 媒体服务器 id
	SourceURL                 string `json:"source_url"`                   // 原始 url
	TimeoutS                  int    `json:"timeout_s"`                    // 超时时间(秒)
	Transport                 int    `json:"transport"`                    // rtsp 拉流方式(0:udp;1:tcp)
	Enabled                   bool   `json:"enabled"`                      // 是否启用
	EnabledAudio              bool   `json:"enabled_audio"`                // 是否启用音频
	EnabledRemoveNoneReader   bool   `json:"enabled_remove_none_reader"`   // 是否无人观看时删除
	EnabledDisabledNoneReader bool   `json:"enabled_disabled_none_reader"` // 是否无人观看时禁用
	StreamKey                 string `json:"stream_key"`                   // zlm 返回的 key
	Pulling                   bool   `json:"pulling"`                      // 拉流状态
}

// DeleteStreamProxyInput 删除拉流代理参数
type DeleteStreamProxyInput struct {
	ID string `uri:"id" binding:"required"`
}
