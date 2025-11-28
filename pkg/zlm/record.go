package zlm

// StartRecordRequest 开始录像请求
type StartRecordRequest struct {
	Type           int    `json:"type"`                      // 0 为 HLS，1 为 MP4
	Vhost          string `json:"vhost"`                     // 流的虚拟主机，例如 __defaultVhost__
	App            string `json:"app"`                       // 流的应用名，例如 rtp
	Stream         string `json:"stream"`                    // 流 ID
	CustomizedPath string `json:"customized_path,omitempty"` // 录像文件保存自定义根目录
	MaxSecond      int    `json:"max_second,omitempty"`      // MP4 录像切片时间大小，单位秒
}

// StartRecordResponse 开始录像响应
type StartRecordResponse struct {
	Code   int    `json:"code"`
	Result bool   `json:"result"`
	Msg    string `json:"msg"`
}

// StartRecord 开始录像
// https://docs.zlmediakit.com/zh/guide/media_server/restful_api.html#_15-startrecord
func (e *Engine) StartRecord(req StartRecordRequest) (*StartRecordResponse, error) {
	data := map[string]any{
		"type":   req.Type,
		"vhost":  req.Vhost,
		"app":    req.App,
		"stream": req.Stream,
	}
	if req.CustomizedPath != "" {
		data["customized_path"] = req.CustomizedPath
	}
	if req.MaxSecond > 0 {
		data["max_second"] = req.MaxSecond
	}

	var resp StartRecordResponse
	if err := e.post("/index/api/startRecord", data, &resp); err != nil {
		return nil, err
	}
	return &resp, e.ErrHandle(resp.Code, resp.Msg)
}

// StopRecordRequest 停止录像请求
type StopRecordRequest struct {
	Type   int    `json:"type"`   // 0 为 HLS，1 为 MP4
	Vhost  string `json:"vhost"`  // 流的虚拟主机
	App    string `json:"app"`    // 流的应用名
	Stream string `json:"stream"` // 流 ID
}

// StopRecordResponse 停止录像响应
type StopRecordResponse struct {
	Code   int    `json:"code"`
	Result bool   `json:"result"`
	Msg    string `json:"msg"`
}

// StopRecord 停止录像
// https://docs.zlmediakit.com/zh/guide/media_server/restful_api.html#_16-stoprecord
func (e *Engine) StopRecord(req StopRecordRequest) (*StopRecordResponse, error) {
	data := map[string]any{
		"type":   req.Type,
		"vhost":  req.Vhost,
		"app":    req.App,
		"stream": req.Stream,
	}

	var resp StopRecordResponse
	if err := e.post("/index/api/stopRecord", data, &resp); err != nil {
		return nil, err
	}
	return &resp, e.ErrHandle(resp.Code, resp.Msg)
}

// IsRecordingRequest 是否正在录像请求
type IsRecordingRequest struct {
	Type   int    `json:"type"`   // 0 为 HLS，1 为 MP4
	Vhost  string `json:"vhost"`  // 流的虚拟主机
	App    string `json:"app"`    // 流的应用名
	Stream string `json:"stream"` // 流 ID
}

// IsRecordingResponse 是否正在录像响应
type IsRecordingResponse struct {
	Code   int    `json:"code"`
	Status bool   `json:"status"` // true 表示正在录像
	Msg    string `json:"msg"`
}

// IsRecording 获取流录像状态
// https://docs.zlmediakit.com/zh/guide/media_server/restful_api.html#_17-isrecording
func (e *Engine) IsRecording(req IsRecordingRequest) (*IsRecordingResponse, error) {
	data := map[string]any{
		"type":   req.Type,
		"vhost":  req.Vhost,
		"app":    req.App,
		"stream": req.Stream,
	}

	var resp IsRecordingResponse
	if err := e.post("/index/api/isRecording", data, &resp); err != nil {
		return nil, err
	}
	return &resp, e.ErrHandle(resp.Code, resp.Msg)
}

// GetMp4RecordFileRequest 获取录像文件请求
type GetMp4RecordFileRequest struct {
	Vhost     string `json:"vhost"`      // 流的虚拟主机
	App       string `json:"app"`        // 流的应用名
	Stream    string `json:"stream"`     // 流 ID
	Period    string `json:"period"`     // 日期，格式为 2020-02-01
	Customize string `json:"customize"`  // 是否为自定义路径录像
}

// GetMp4RecordFileResponse 获取录像文件响应
type GetMp4RecordFileResponse struct {
	Code   int                  `json:"code"`
	Data   *Mp4RecordFileFolder `json:"data,omitempty"`
	Msg    string               `json:"msg"`
	RootPath string             `json:"rootPath,omitempty"`
}

// Mp4RecordFileFolder 录像文件夹
type Mp4RecordFileFolder struct {
	Paths   []string                `json:"paths,omitempty"`
	Folders []string                `json:"folders,omitempty"`
}

// GetMp4RecordFile 获取录像文件列表
// https://docs.zlmediakit.com/zh/guide/media_server/restful_api.html#_18-getmp4recordfile
func (e *Engine) GetMp4RecordFile(req GetMp4RecordFileRequest) (*GetMp4RecordFileResponse, error) {
	data := map[string]any{
		"vhost":  req.Vhost,
		"app":    req.App,
		"stream": req.Stream,
		"period": req.Period,
	}
	if req.Customize != "" {
		data["customized_path"] = req.Customize
	}

	var resp GetMp4RecordFileResponse
	if err := e.post("/index/api/getMp4RecordFile", data, &resp); err != nil {
		return nil, err
	}
	return &resp, e.ErrHandle(resp.Code, resp.Msg)
}
