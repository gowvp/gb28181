package gbs

import (
	"encoding/xml"
	"fmt"
	"time"

	"github.com/gowvp/gb28181/pkg/gbs/sip"
)

// AlarmMethod 报警通知方法
const (
	NotifyMethodAlarm = "alarm"
)

// AlarmPriority 报警优先级
const (
	AlarmPriorityLow      = "1" // 一般
	AlarmPriorityMedium   = "2" // 较大
	AlarmPriorityHigh     = "3" // 重大
	AlarmPriorityCritical = "4" // 特大
)

// AlarmMethodType 报警方式
const (
	AlarmMethodDevice = "1" // 设备报警
	AlarmMethodZone   = "2" // 防区报警
	AlarmMethodVideo  = "5" // 视频报警
	AlarmMethodOther  = "6" // 其他
)

// AlarmType 报警类型
const (
	// 报警类型(可选) GB2312
	AlarmTypeMotion      = "1" // 运动检测
	AlarmTypeVideoLoss   = "2" // 视频丢失
	AlarmTypeVideoBlind  = "3" // 视频遮挡
	AlarmTypeInput       = "4" // 输入报警
	AlarmTypeFaceDetect  = "5" // 人脸检测
	AlarmTypeCrowdGather = "6" // 人群聚集
)

// AlarmNotify 报警通知 (设备上报)
type AlarmNotify struct {
	XMLName          xml.Name   `xml:"Notify"`
	CmdType          string     `xml:"CmdType"`
	SN               int        `xml:"SN"`
	DeviceID         string     `xml:"DeviceID"`
	AlarmPriority    string     `xml:"AlarmPriority"`              // 报警级别
	AlarmMethod      string     `xml:"AlarmMethod"`                // 报警方式
	AlarmTime        string     `xml:"AlarmTime"`                  // 报警时间
	AlarmDescription string     `xml:"AlarmDescription,omitempty"` // 报警描述
	Longitude        float64    `xml:"Longitude,omitempty"`        // 经度
	Latitude         float64    `xml:"Latitude,omitempty"`         // 纬度
	Info             *AlarmInfo `xml:"Info,omitempty"`
}

// AlarmInfo 报警详细信息
type AlarmInfo struct {
	AlarmType      string `xml:"AlarmType,omitempty"`      // 报警类型
	AlarmTypeParam string `xml:"AlarmTypeParam,omitempty"` // 报警类型参数
}

// AlarmSubscribeRequest 报警订阅请求
type AlarmSubscribeRequest struct {
	XMLName            xml.Name `xml:"Query"`
	CmdType            string   `xml:"CmdType"`
	SN                 int      `xml:"SN"`
	DeviceID           string   `xml:"DeviceID"`
	StartAlarmPriority string   `xml:"StartAlarmPriority,omitempty"` // 起始报警级别
	EndAlarmPriority   string   `xml:"EndAlarmPriority,omitempty"`   // 结束报警级别
	AlarmMethod        string   `xml:"AlarmMethod,omitempty"`        // 报警方式
	AlarmType          string   `xml:"AlarmType,omitempty"`          // 报警类型
	StartTime          string   `xml:"StartTime,omitempty"`          // 开始时间
	EndTime            string   `xml:"EndTime,omitempty"`            // 结束时间
}

// AlarmEvent 报警事件(用于 API 返回)
type AlarmEvent struct {
	ID            string    `json:"id"`
	DeviceID      string    `json:"device_id"`
	ChannelID     string    `json:"channel_id,omitempty"`
	AlarmPriority string    `json:"alarm_priority"`
	AlarmMethod   string    `json:"alarm_method"`
	AlarmType     string    `json:"alarm_type,omitempty"`
	AlarmTime     time.Time `json:"alarm_time"`
	Description   string    `json:"description,omitempty"`
	Longitude     float64   `json:"longitude,omitempty"`
	Latitude      float64   `json:"latitude,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
}

// AlarmSubscribe 报警订阅
func (g *GB28181API) AlarmSubscribe(deviceID string, expireSeconds int) error {
	dev, ok := g.svr.memoryStorer.Load(deviceID)
	if !ok {
		return ErrDeviceNotExist
	}

	req := AlarmSubscribeRequest{
		CmdType:  "Alarm",
		SN:       sip.RandInt(100000, 999999),
		DeviceID: deviceID,
	}

	body, err := xml.Marshal(req)
	if err != nil {
		return err
	}

	// 构建报警订阅请求 (使用 MESSAGE 方法，因为 SUBSCRIBE 方法需要设备支持)
	hb := sip.NewHeaderBuilder().
		SetTo(dev.to).
		SetFrom(&g.svr.fromAddress).
		AddVia(&sip.ViaHop{
			Params: sip.NewParams().Add("branch", sip.String{Str: sip.GenerateBranch()}),
		}).
		SetContentType(&sip.ContentTypeXML).
		SetMethod(sip.MethodMessage)

	sipReq := sip.NewRequest("", sip.MethodMessage, dev.to.URI, sip.DefaultSipVersion, hb.Build(), body)
	sipReq.SetDestination(dev.source)

	// 添加 Event 和 Expires 头
	sipReq.AppendHeader(&sip.GenericHeader{HeaderName: "Event", Contents: "presence"})
	sipReq.AppendHeader(&sip.GenericHeader{HeaderName: "Expires", Contents: fmt.Sprintf("%d", expireSeconds)})

	tx, err := g.svr.Request(sipReq)
	if err != nil {
		return err
	}

	_, err = sipResponse(tx)
	return err
}

// AlarmSubscribe 报警订阅 (Server 方法)
func (s *Server) AlarmSubscribe(deviceID string, expireSeconds int) error {
	return s.gb.AlarmSubscribe(deviceID, expireSeconds)
}

// AlarmUnsubscribe 取消报警订阅
func (s *Server) AlarmUnsubscribe(deviceID string) error {
	return s.gb.AlarmSubscribe(deviceID, 0) // Expires=0 表示取消订阅
}

// HandleAlarmNotify 处理报警通知 (由消息处理器调用)
func (g *GB28181API) handleAlarmNotify(body []byte) (*AlarmNotify, error) {
	var alarm AlarmNotify
	if err := sip.XMLDecode(body, &alarm); err != nil {
		return nil, err
	}
	return &alarm, nil
}
