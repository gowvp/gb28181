package api

import (
	"github.com/gin-gonic/gin"
	"github.com/gowvp/gb28181/internal/core/sms"
	"github.com/gowvp/gb28181/pkg/zlm"
	"github.com/ixugo/goddd/pkg/reason"
	"github.com/ixugo/goddd/pkg/web"
)

// registerRecordAPI 注册录像相关 API
func registerRecordAPI(g gin.IRouter, api SmsAPI, handler ...gin.HandlerFunc) {
	group := g.Group("/records", handler...)
	group.POST("/start", web.WrapH(api.startRecord))
	group.POST("/stop", web.WrapH(api.stopRecord))
	group.GET("/status", web.WrapH(api.getRecordStatus))
	group.GET("/files", web.WrapH(api.getRecordFiles))
}

// startRecordInput 开始录像输入
type startRecordInput struct {
	App            string `json:"app" binding:"required"`    // 流的应用名，例如 rtp
	Stream         string `json:"stream" binding:"required"` // 流 ID
	Type           int    `json:"type"`                      // 0 为 HLS，1 为 MP4
	MaxSecond      int    `json:"max_second,omitempty"`      // MP4 录像切片时间大小，单位秒
	CustomizedPath string `json:"customized_path,omitempty"` // 录像文件保存自定义根目录
}

// startRecord 开始录像
func (a SmsAPI) startRecord(c *gin.Context, in *startRecordInput) (gin.H, error) {
	svr, err := a.smsCore.GetMediaServer(c.Request.Context(), sms.DefaultMediaServerID)
	if err != nil {
		return nil, err
	}

	resp, err := a.smsCore.StartRecord(svr, zlm.StartRecordRequest{
		Type:           in.Type,
		Vhost:          "__defaultVhost__",
		App:            in.App,
		Stream:         in.Stream,
		CustomizedPath: in.CustomizedPath,
		MaxSecond:      in.MaxSecond,
	})
	if err != nil {
		return nil, reason.ErrServer.SetMsg(err.Error())
	}
	if !resp.Result {
		return nil, reason.ErrServer.SetMsg(resp.Msg)
	}

	return gin.H{"msg": "录像已开始"}, nil
}

// stopRecordInput 停止录像输入
type stopRecordInput struct {
	App    string `json:"app" binding:"required"`    // 流的应用名，例如 rtp
	Stream string `json:"stream" binding:"required"` // 流 ID
	Type   int    `json:"type"`                      // 0 为 HLS，1 为 MP4
}

// stopRecord 停止录像
func (a SmsAPI) stopRecord(c *gin.Context, in *stopRecordInput) (gin.H, error) {
	svr, err := a.smsCore.GetMediaServer(c.Request.Context(), sms.DefaultMediaServerID)
	if err != nil {
		return nil, err
	}

	resp, err := a.smsCore.StopRecord(svr, zlm.StopRecordRequest{
		Type:   in.Type,
		Vhost:  "__defaultVhost__",
		App:    in.App,
		Stream: in.Stream,
	})
	if err != nil {
		return nil, reason.ErrServer.SetMsg(err.Error())
	}
	if !resp.Result {
		return nil, reason.ErrServer.SetMsg(resp.Msg)
	}

	return gin.H{"msg": "录像已停止"}, nil
}

// getRecordStatusInput 获取录像状态输入
type getRecordStatusInput struct {
	App    string `form:"app" binding:"required"`    // 流的应用名，例如 rtp
	Stream string `form:"stream" binding:"required"` // 流 ID
	Type   int    `form:"type"`                      // 0 为 HLS，1 为 MP4
}

// getRecordStatusOutput 获取录像状态输出
type getRecordStatusOutput struct {
	Status bool `json:"status"` // true 表示正在录像
}

// getRecordStatus 获取流录像状态
func (a SmsAPI) getRecordStatus(c *gin.Context, in *getRecordStatusInput) (*getRecordStatusOutput, error) {
	svr, err := a.smsCore.GetMediaServer(c.Request.Context(), sms.DefaultMediaServerID)
	if err != nil {
		return nil, err
	}

	resp, err := a.smsCore.IsRecording(svr, zlm.IsRecordingRequest{
		Type:   in.Type,
		Vhost:  "__defaultVhost__",
		App:    in.App,
		Stream: in.Stream,
	})
	if err != nil {
		return nil, reason.ErrServer.SetMsg(err.Error())
	}

	return &getRecordStatusOutput{Status: resp.Status}, nil
}

// getRecordFilesInput 获取录像文件输入
type getRecordFilesInput struct {
	App       string `form:"app" binding:"required"`    // 流的应用名，例如 rtp
	Stream    string `form:"stream" binding:"required"` // 流 ID
	Period    string `form:"period" binding:"required"` // 日期，格式为 2020-02-01
	Customize string `form:"customize"`                 // 是否为自定义路径录像
}

// getRecordFilesOutput 获取录像文件输出
type getRecordFilesOutput struct {
	RootPath string   `json:"root_path"`
	Paths    []string `json:"paths"`
	Folders  []string `json:"folders"`
}

// getRecordFiles 获取录像文件列表
func (a SmsAPI) getRecordFiles(c *gin.Context, in *getRecordFilesInput) (*getRecordFilesOutput, error) {
	svr, err := a.smsCore.GetMediaServer(c.Request.Context(), sms.DefaultMediaServerID)
	if err != nil {
		return nil, err
	}

	resp, err := a.smsCore.GetMp4RecordFile(svr, zlm.GetMp4RecordFileRequest{
		Vhost:     "__defaultVhost__",
		App:       in.App,
		Stream:    in.Stream,
		Period:    in.Period,
		Customize: in.Customize,
	})
	if err != nil {
		return nil, reason.ErrServer.SetMsg(err.Error())
	}

	out := &getRecordFilesOutput{
		RootPath: resp.RootPath,
	}
	if resp.Data != nil {
		out.Paths = resp.Data.Paths
		out.Folders = resp.Data.Folders
	}
	return out, nil
}
