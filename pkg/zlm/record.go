package zlm

const (
	startRecordPath  = "/index/api/startRecord"
	stopRecordPath   = "/index/api/stopRecord"
	getMediaListPath = "/index/api/getMediaList"
)

// StartRecordRequest 开始录制请求参数
type StartRecordRequest struct {
	Type       int    `json:"type"`                 // 0 为 hls，1 为 mp4
	Vhost      string `json:"vhost"`                // 虚拟主机
	App        string `json:"app"`                  // 应用名
	Stream     string `json:"stream"`               // 流 ID
	CustomPath string `json:"customized_path"`      // 自定义存储路径
	MaxSecond  int    `json:"max_second,omitempty"` // 录制时长，单位秒，置 0 则不限制
}

// StartRecordResponse 开始录制响应
type StartRecordResponse struct {
	FixedHeader
	Result bool `json:"result"` // 是否成功
}

// StopRecordRequest 停止录制请求参数
type StopRecordRequest struct {
	Type   int    `json:"type"`   // 0 为 hls，1 为 mp4
	Vhost  string `json:"vhost"`  // 虚拟主机
	App    string `json:"app"`    // 应用名
	Stream string `json:"stream"` // 流 ID
}

// StopRecordResponse 停止录制响应
type StopRecordResponse struct {
	FixedHeader
	Result bool `json:"result"` // 是否成功
}

// StartRecord 开始录制，触发 ZLM 对指定流进行 MP4 录制
func (e *Engine) StartRecord(req StartRecordRequest) (*StartRecordResponse, error) {
	data := map[string]any{
		"type":   req.Type,
		"vhost":  req.Vhost,
		"app":    req.App,
		"stream": req.Stream,
	}
	if req.CustomPath != "" {
		data["customized_path"] = req.CustomPath
	}
	if req.MaxSecond > 0 {
		data["max_second"] = req.MaxSecond
	}

	var resp StartRecordResponse
	if err := e.post(startRecordPath, data, &resp); err != nil {
		return nil, err
	}
	if err := e.ErrHandle(resp.Code, resp.Msg); err != nil {
		return nil, err
	}
	return &resp, nil
}

// StopRecord 停止录制
func (e *Engine) StopRecord(req StopRecordRequest) (*StopRecordResponse, error) {
	data := map[string]any{
		"type":   req.Type,
		"vhost":  req.Vhost,
		"app":    req.App,
		"stream": req.Stream,
	}

	var resp StopRecordResponse
	if err := e.post(stopRecordPath, data, &resp); err != nil {
		return nil, err
	}
	if err := e.ErrHandle(resp.Code, resp.Msg); err != nil {
		return nil, err
	}
	return &resp, nil
}

// MediaTrack 流的音视频轨道信息
type MediaTrack struct {
	CodecID     int     `json:"codec_id"`      // H264=0, H265=1, AAC=2, G711A=3, G711U=4
	CodecIDName string  `json:"codec_id_name"` // 编码类型名称
	CodecType   int     `json:"codec_type"`    // Video=0, Audio=1
	Ready       bool    `json:"ready"`         // 轨道是否就绪
	FPS         float64 `json:"fps"`           // 视频帧率
	Width       int     `json:"width"`         // 视频宽
	Height      int     `json:"height"`        // 视频高
	Channels    int     `json:"channels"`      // 音频通道数
	SampleBit   int     `json:"sample_bit"`    // 音频采样位数
	SampleRate  int     `json:"sample_rate"`   // 音频采样率
	Frames      int64   `json:"frames"`        // 累计帧数
	KeyFrames   int64   `json:"key_frames"`    // 累计关键帧数
	Loss        float64 `json:"loss"`          // 丢包率
	Duration    int64   `json:"duration"`      // 时长(毫秒)
}

// MediaItem getMediaList 返回的单条流信息
type MediaItem struct {
	App              string       `json:"app"`
	Stream           string       `json:"stream"`
	Schema           string       `json:"schema"`
	Vhost            string       `json:"vhost"`
	IsRecordingMP4   bool         `json:"isRecordingMP4"`
	IsRecordingHLS   bool         `json:"isRecordingHLS"`
	OriginType       int          `json:"originType"`
	OriginTypeStr    string       `json:"originTypeStr"`
	OriginURL        string       `json:"originUrl"`
	ReaderCount      int          `json:"readerCount"`
	TotalReaderCount int          `json:"totalReaderCount"`
	AliveSecond      int          `json:"aliveSecond"`
	Tracks           []MediaTrack `json:"tracks"`
}

// GetMediaListResponse getMediaList 响应
type GetMediaListResponse struct {
	FixedHeader
	Data []MediaItem `json:"data"`
}

// GetMediaList 批量获取所有在线流列表（含录制状态）
// 一次请求获取全部流的 isRecordingMP4 状态，避免逐流查询
func (e *Engine) GetMediaList() (*GetMediaListResponse, error) {
	var resp GetMediaListResponse
	if err := e.post(getMediaListPath, nil, &resp); err != nil {
		return nil, err
	}
	if err := e.ErrHandle(resp.Code, resp.Msg); err != nil {
		return nil, err
	}
	return &resp, nil
}

// GetMediaInfoRequest 获取单条流详细信息的请求参数
type GetMediaInfoRequest struct {
	Schema string `json:"schema"` // 协议名，如 rtsp
	Vhost  string `json:"vhost"`
	App    string `json:"app"`
	Stream string `json:"stream"`
}

// GetMediaInfoResponse 获取单条流详细信息的响应（复用 getMediaList 接口按条件过滤）
type GetMediaInfoResponse struct {
	FixedHeader
	Data []MediaItem `json:"data"`
}

// GetMediaInfo 获取指定流的详细信息（音视频编码、分辨率、帧率等）
// 基于 getMediaList 接口带精确参数过滤单条流
func (e *Engine) GetMediaInfo(req GetMediaInfoRequest) (*GetMediaInfoResponse, error) {
	data := map[string]any{
		"schema": req.Schema,
		"vhost":  req.Vhost,
		"app":    req.App,
		"stream": req.Stream,
	}
	var resp GetMediaInfoResponse
	if err := e.post(getMediaListPath, data, &resp); err != nil {
		return nil, err
	}
	if err := e.ErrHandle(resp.Code, resp.Msg); err != nil {
		return nil, err
	}
	return &resp, nil
}
