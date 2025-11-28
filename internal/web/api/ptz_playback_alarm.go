package api

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gowvp/gb28181/internal/core/ipc"
	"github.com/gowvp/gb28181/internal/core/sms"
	"github.com/gowvp/gb28181/pkg/gbs"
	"github.com/ixugo/goddd/pkg/reason"
	"github.com/ixugo/goddd/pkg/web"
)

// registerPTZAPI 注册云台控制 API
func registerPTZAPI(g gin.IRouter, api IPCAPI, handler ...gin.HandlerFunc) {
	group := g.Group("/ptz", handler...)
	group.POST("/control", web.WrapH(api.ptzControl))
	group.POST("/preset", web.WrapH(api.ptzPreset))
}

// ptzControlInput 云台控制输入
type ptzControlInput struct {
	DeviceID  string `json:"device_id" binding:"required"` // 设备 ID
	ChannelID string `json:"channel_id" binding:"required"` // 通道 ID
	Command   string `json:"command" binding:"required"`   // 控制命令: stop, left, right, up, down, zoom_in, zoom_out, left_up, left_down, right_up, right_down, iris_in, iris_out, focus_in, focus_out
	Speed     int    `json:"speed"`                        // 速度 (0-255), 默认 50
}

// ptzControl 云台方向控制
func (a IPCAPI) ptzControl(c *gin.Context, in *ptzControlInput) (gin.H, error) {
	if in.Speed <= 0 {
		in.Speed = 50
	}
	if in.Speed > 255 {
		in.Speed = 255
	}

	var cmd byte
	switch in.Command {
	case "stop":
		cmd = gbs.PTZCmdStop
	case "left":
		cmd = gbs.PTZCmdLeft
	case "right":
		cmd = gbs.PTZCmdRight
	case "up":
		cmd = gbs.PTZCmdUp
	case "down":
		cmd = gbs.PTZCmdDown
	case "zoom_in":
		cmd = gbs.PTZCmdZoomIn
	case "zoom_out":
		cmd = gbs.PTZCmdZoomOut
	case "left_up":
		cmd = gbs.PTZCmdLeftUp
	case "left_down":
		cmd = gbs.PTZCmdLeftDown
	case "right_up":
		cmd = gbs.PTZCmdRightUp
	case "right_down":
		cmd = gbs.PTZCmdRightDown
	case "iris_in":
		cmd = gbs.PTZCmdIrisIn
	case "iris_out":
		cmd = gbs.PTZCmdIrisOut
	case "focus_in":
		cmd = gbs.PTZCmdFocusIn
	case "focus_out":
		cmd = gbs.PTZCmdFocusOut
	default:
		return nil, reason.ErrBadRequest.SetMsg("不支持的控制命令: " + in.Command)
	}

	speed := byte(in.Speed)
	ptzCmd := gbs.BuildPTZCmd(cmd, speed, speed, 0)

	if err := a.uc.SipServer.PTZControl(in.DeviceID, in.ChannelID, ptzCmd); err != nil {
		return nil, reason.ErrServer.SetMsg(err.Error())
	}

	return gin.H{"msg": "ok"}, nil
}

// ptzPresetInput 预置位控制输入
type ptzPresetInput struct {
	DeviceID    string `json:"device_id" binding:"required"`    // 设备 ID
	ChannelID   string `json:"channel_id" binding:"required"`   // 通道 ID
	Action      string `json:"action" binding:"required"`       // 动作: set, call, delete
	PresetIndex int    `json:"preset_index" binding:"required"` // 预置位编号 (1-255)
}

// ptzPreset 预置位控制
func (a IPCAPI) ptzPreset(c *gin.Context, in *ptzPresetInput) (gin.H, error) {
	if in.PresetIndex < 1 || in.PresetIndex > 255 {
		return nil, reason.ErrBadRequest.SetMsg("预置位编号必须在 1-255 之间")
	}

	var cmd byte
	switch in.Action {
	case "set":
		cmd = gbs.PTZCmdPresetSet
	case "call":
		cmd = gbs.PTZCmdPresetCall
	case "delete":
		cmd = gbs.PTZCmdPresetDelete
	default:
		return nil, reason.ErrBadRequest.SetMsg("不支持的预置位操作: " + in.Action)
	}

	ptzCmd := gbs.BuildPresetCmd(cmd, byte(in.PresetIndex))

	if err := a.uc.SipServer.PTZControl(in.DeviceID, in.ChannelID, ptzCmd); err != nil {
		return nil, reason.ErrServer.SetMsg(err.Error())
	}

	return gin.H{"msg": "ok"}, nil
}

// registerPlaybackAPI 注册录像回放 API
func registerPlaybackAPI(g gin.IRouter, api IPCAPI, handler ...gin.HandlerFunc) {
	group := g.Group("/playback", handler...)
	group.POST("/start", web.WrapH(api.startPlayback))
	group.POST("/stop", web.WrapH(api.stopPlayback))
	group.POST("/control", web.WrapH(api.playbackControl))
	group.GET("/records", web.WrapH(api.queryRecordInfo))
}

// startPlaybackInput 开始回放输入
type startPlaybackInput struct {
	ChannelID string `json:"channel_id" binding:"required"` // 通道 ID
	StartTime int64  `json:"start_time" binding:"required"` // 开始时间戳(秒)
	EndTime   int64  `json:"end_time" binding:"required"`   // 结束时间戳(秒)
}

// startPlaybackOutput 开始回放输出
type startPlaybackOutput struct {
	StreamID string `json:"stream_id"`
	App      string `json:"app"`
	Stream   string `json:"stream"`
}

// startPlayback 开始录像回放
func (a IPCAPI) startPlayback(c *gin.Context, in *startPlaybackInput) (*startPlaybackOutput, error) {
	ch, err := a.ipc.GetChannel(c.Request.Context(), in.ChannelID)
	if err != nil {
		return nil, err
	}

	svr, err := a.uc.SMSAPI.smsCore.GetMediaServer(c.Request.Context(), sms.DefaultMediaServerID)
	if err != nil {
		return nil, err
	}

	streamID := "playback_" + ch.ID + "_" + time.Now().Format("20060102150405")

	if err := a.uc.SipServer.Playback(&gbs.PlaybackInput{
		Channel: &ipc.Channel{
			ID:        ch.ID,
			DeviceID:  ch.DeviceID,
			ChannelID: ch.ChannelID,
		},
		SMS:        svr,
		StreamMode: 1, // TCP 被动
		StartTime:  in.StartTime,
		EndTime:    in.EndTime,
	}); err != nil {
		return nil, reason.ErrServer.SetMsg(err.Error())
	}

	return &startPlaybackOutput{
		StreamID: streamID,
		App:      "rtp",
		Stream:   streamID,
	}, nil
}

// stopPlaybackInput 停止回放输入
type stopPlaybackInput struct {
	ChannelID string `json:"channel_id" binding:"required"` // 通道 ID
}

// stopPlayback 停止录像回放
func (a IPCAPI) stopPlayback(c *gin.Context, in *stopPlaybackInput) (gin.H, error) {
	ch, err := a.ipc.GetChannel(c.Request.Context(), in.ChannelID)
	if err != nil {
		return nil, err
	}

	if err := a.uc.SipServer.StopPlayback(c.Request.Context(), &gbs.StopPlaybackInput{
		Channel: &ipc.Channel{
			ID:        ch.ID,
			DeviceID:  ch.DeviceID,
			ChannelID: ch.ChannelID,
		},
	}); err != nil {
		return nil, reason.ErrServer.SetMsg(err.Error())
	}

	return gin.H{"msg": "ok"}, nil
}

// playbackControlInput 回放控制输入
type playbackControlInput struct {
	DeviceID  string  `json:"device_id" binding:"required"`  // 设备 ID
	ChannelID string  `json:"channel_id" binding:"required"` // 通道 ID
	Action    string  `json:"action" binding:"required"`     // 动作: play, pause, scale
	Scale     float64 `json:"scale,omitempty"`               // 倍速 (0.5, 1, 2, 4 等)
}

// playbackControl 回放控制 (暂停/继续/倍速)
func (a IPCAPI) playbackControl(c *gin.Context, in *playbackControlInput) (gin.H, error) {
	var control gbs.PlaybackControl
	switch in.Action {
	case "play":
		control = gbs.PlaybackControlPlay
	case "pause":
		control = gbs.PlaybackControlPause
	case "scale":
		control = gbs.PlaybackControlScale
		if in.Scale <= 0 {
			in.Scale = 1.0
		}
	default:
		return nil, reason.ErrBadRequest.SetMsg("不支持的控制操作: " + in.Action)
	}

	if err := a.uc.SipServer.PlaybackControl(in.DeviceID, in.ChannelID, control, in.Scale); err != nil {
		return nil, reason.ErrServer.SetMsg(err.Error())
	}

	return gin.H{"msg": "ok"}, nil
}

// queryRecordInfoInput 查询录像信息输入
type queryRecordInfoInput struct {
	DeviceID  string `form:"device_id" binding:"required"`  // 设备 ID
	ChannelID string `form:"channel_id" binding:"required"` // 通道 ID
	StartTime int64  `form:"start_time" binding:"required"` // 开始时间戳(秒)
	EndTime   int64  `form:"end_time" binding:"required"`   // 结束时间戳(秒)
}

// queryRecordInfo 查询设备端录像信息
func (a IPCAPI) queryRecordInfo(c *gin.Context, in *queryRecordInfoInput) (*gbs.Records, error) {
	records, err := a.uc.SipServer.QueryRecordInfo(in.DeviceID, in.ChannelID, in.StartTime, in.EndTime)
	if err != nil {
		return nil, reason.ErrServer.SetMsg(err.Error())
	}
	return records, nil
}

// registerAlarmAPI 注册报警 API
func registerAlarmAPI(g gin.IRouter, api IPCAPI, handler ...gin.HandlerFunc) {
	group := g.Group("/alarms", handler...)
	group.POST("/subscribe", web.WrapH(api.alarmSubscribe))
	group.POST("/unsubscribe", web.WrapH(api.alarmUnsubscribe))
}

// alarmSubscribeInput 报警订阅输入
type alarmSubscribeInput struct {
	DeviceID      string `json:"device_id" binding:"required"` // 设备 ID
	ExpireSeconds int    `json:"expire_seconds"`               // 订阅有效期(秒), 默认 3600
}

// alarmSubscribe 报警订阅
func (a IPCAPI) alarmSubscribe(c *gin.Context, in *alarmSubscribeInput) (gin.H, error) {
	if in.ExpireSeconds <= 0 {
		in.ExpireSeconds = 3600 // 默认 1 小时
	}

	if err := a.uc.SipServer.AlarmSubscribe(in.DeviceID, in.ExpireSeconds); err != nil {
		return nil, reason.ErrServer.SetMsg(err.Error())
	}

	// 发送订阅成功通知
	NotifyAlarmSubscriptionChanged(in.DeviceID, true)

	return gin.H{"msg": "ok", "expires": in.ExpireSeconds}, nil
}

// alarmUnsubscribeInput 取消报警订阅输入
type alarmUnsubscribeInput struct {
	DeviceID string `json:"device_id" binding:"required"` // 设备 ID
}

// alarmUnsubscribe 取消报警订阅
func (a IPCAPI) alarmUnsubscribe(c *gin.Context, in *alarmUnsubscribeInput) (gin.H, error) {
	if err := a.uc.SipServer.AlarmUnsubscribe(in.DeviceID); err != nil {
		return nil, reason.ErrServer.SetMsg(err.Error())
	}

	// 发送取消订阅通知
	NotifyAlarmSubscriptionChanged(in.DeviceID, false)

	return gin.H{"msg": "ok"}, nil
}
