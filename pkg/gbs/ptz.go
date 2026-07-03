package gbs

import (
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"log/slog"
	"net/http"
	"sync/atomic"

	"github.com/gowvp/owl/pkg/gbs/sip"
)

// ptzSNCounter PTZ 命令序列号计数器（原子操作保证线程安全）
var ptzSNCounter uint64

// getNextPTZSN 获取下一个 PTZ 序列号
func getNextPTZSN() int {
	return int(atomic.AddUint64(&ptzSNCounter, 1))
}

// PTZControlInfo PTZ 控制附加信息
type PTZControlInfo struct {
	ControlPriority int `xml:"ControlPriority"` // 控制优先级
}

// PTZControlRequest GB28181 云台控制请求
type PTZControlRequest struct {
	XMLName  xml.Name       `xml:"Control"`
	CmdType  string         `xml:"CmdType"`  // 命令类型：DeviceControl 或 PTZCmd
	SN       int            `xml:"SN"`       // 序列号
	DeviceID string         `xml:"DeviceID"` // 目标设备编码
	PTZCmd   interface{}    `xml:"PTZCmd"`   // 云台控制码（可以是字符串或结构化对象）
	Info     PTZControlInfo `xml:"Info"`     // 控制信息
}

// PTZCmdStructured 结构化的 PTZ 命令（海康等厂家可能需要）
type PTZCmdStructured struct {
	Code  string `xml:"Code"`  // 动作代码：ZoomIn, ZoomOut, Left, Right, Up, Down, Stop
	Speed int    `xml:"Speed"` // 速度：1-8
}

// NewPTZControlRequest 创建云台控制请求（使用十六进制字符串格式）
func NewPTZControlRequest(deviceID string, ptzCmd string) *PTZControlRequest {
	return &PTZControlRequest{
		CmdType:  "DeviceControl",
		SN:       getNextPTZSN(), // 使用递增的序列号
		DeviceID: deviceID,
		PTZCmd:   ptzCmd,
		Info: PTZControlInfo{
			ControlPriority: 5, // 设置控制优先级，与其他平台一致
		},
	}
}

// NewPTZControlRequestStructured 创建结构化 PTZ 控制请求（海康等厂家可能需要）
func NewPTZControlRequestStructured(deviceID string, code string, speed int) *PTZControlRequest {
	return &PTZControlRequest{
		CmdType:  "DeviceControl", // 尝试使用 DeviceControl 而不是 PTZCmd
		SN:       getNextPTZSN(),  // 使用递增的序列号
		DeviceID: deviceID,
		PTZCmd: PTZCmdStructured{
			Code:  code,
			Speed: speed,
		},
	}
}

// Marshal 序列化为 XML
func (p *PTZControlRequest) Marshal() []byte {
	// 使用 GB2312 编码声明，与其他平台保持一致
	b, _ := xml.Marshal(p)
	xmlHeader := "<?xml version=\"1.0\" encoding=\"GB2312\"?>\n"
	return append([]byte(xmlHeader), b...)
}

// PTZCmdBuilder GB28181 云台控制码构建器
type PTZCmdBuilder struct {
	address   byte // 地址
	direction byte // 方向控制（字节4）
	horzSpeed byte // 水平速度（字节5）
	vertSpeed byte // 垂直速度（字节6）
	zoomSpeed byte // 变倍速度（字节7高4位）
}

// 云台控制命令常量（对应字节4的Bit位）
const (
	PTZ_BIT_ZOOM_OUT = 0x20 // Bit5
	PTZ_BIT_ZOOM_IN  = 0x10 // Bit4
	PTZ_BIT_UP       = 0x08 // Bit3
	PTZ_BIT_DOWN     = 0x04 // Bit2
	PTZ_BIT_LEFT     = 0x02 // Bit1
	PTZ_BIT_RIGHT    = 0x01 // Bit0
)

// BuildContinuousMove 构建连续移动命令（按照 GB28181 标准）
func BuildContinuousMove(direction string, speed float64) string {
	builder := &PTZCmdBuilder{
		address:   0x01, // 默认地址
		direction: 0x00,
	}

	speedByte := uint8(speed * 255)
	if speedByte == 0 {
		speedByte = 128 // 默认速度，与其他平台保持一致
	}
	// 强制使用 0x80 (128) 作为速度值，与其他平台完全一致
	speedByte = 0x80

	// 根据方向设置字节4的Bit位和对应的速度
	switch direction {
	case "up":
		builder.direction = PTZ_BIT_UP
		builder.horzSpeed = speedByte // 水平速度也设置
		builder.vertSpeed = speedByte
	case "down":
		builder.direction = PTZ_BIT_DOWN
		builder.horzSpeed = speedByte // 水平速度也设置
		builder.vertSpeed = speedByte
	case "left":
		builder.direction = PTZ_BIT_LEFT
		builder.horzSpeed = speedByte
		builder.vertSpeed = speedByte // 垂直速度也设置
	case "right":
		builder.direction = PTZ_BIT_RIGHT
		builder.horzSpeed = speedByte
		builder.vertSpeed = speedByte // 垂直速度也设置
	case "upleft":
		builder.direction = PTZ_BIT_UP | PTZ_BIT_LEFT
		builder.horzSpeed = speedByte
		builder.vertSpeed = speedByte
	case "upright":
		builder.direction = PTZ_BIT_UP | PTZ_BIT_RIGHT
		builder.horzSpeed = speedByte
		builder.vertSpeed = speedByte
	case "downleft":
		builder.direction = PTZ_BIT_DOWN | PTZ_BIT_LEFT
		builder.horzSpeed = speedByte
		builder.vertSpeed = speedByte
	case "downright":
		builder.direction = PTZ_BIT_DOWN | PTZ_BIT_RIGHT
		builder.horzSpeed = speedByte
		builder.vertSpeed = speedByte
	case "zoomin":
		builder.direction = PTZ_BIT_ZOOM_IN
		builder.zoomSpeed = speedByte >> 4 // 高4位
		builder.horzSpeed = 0x00       
		builder.vertSpeed = 0x00 
	case "zoomout":
		builder.direction = PTZ_BIT_ZOOM_OUT
		builder.zoomSpeed = speedByte >> 4 // 高4位
		builder.horzSpeed = 0x00       
		builder.vertSpeed = 0x00 
	default:
		return ""
	}

	return builder.build()
}

// BuildStop 构建停止命令
func BuildStop() string {
	builder := &PTZCmdBuilder{
		address:   0x01,
		direction: 0x00, // 所有方向为0表示停止
		horzSpeed: 0x00,
		vertSpeed: 0x00,
		zoomSpeed: 0x00,
	}
	return builder.build()
}

// build 构建最终的云台控制码（8字节十六进制字符串）
// 格式：A5 | 地址低8位 | 地址高4位+其他 | 方向控制 | 水平速度 | 垂直速度 | 变倍速度+地址 | 校验和
func (p *PTZCmdBuilder) build() string {
	// 字节1: 固定为 A5
	byte1 := byte(0xA5)

	// 字节2: 地址低8位（使用 0x0F 作为默认值，与其他平台一致）
	byte2 := byte(0x0F)

	// 字节3: 地址高4位 + 其他控制（使用 0x01 作为默认值）
	byte3 := byte(0x01)

	// 字节4: 方向控制
	byte4 := p.direction

	// 字节5: 水平速度
	byte5 := p.horzSpeed

	// 字节6: 垂直速度
	byte6 := p.vertSpeed

	// 字节7: 变倍速度(高4位) + 地址(低4位)
	// 即使不变倍，也设置一个默认值（与其他平台保持一致）
	byte7 := (p.zoomSpeed << 4) | 0x00
	if byte7 == 0 {
		byte7 = 0x80 // 默认值，与其他平台一致
	}

	// 调试日志
	slog.Info("PTZ命令构建详情",
		"direction", fmt.Sprintf("0x%02X", p.direction),
		"horz_speed", fmt.Sprintf("0x%02X", p.horzSpeed),
		"vert_speed", fmt.Sprintf("0x%02X", p.vertSpeed),
		"zoom_speed_raw", fmt.Sprintf("0x%02X", p.zoomSpeed),
		"byte7_final", fmt.Sprintf("0x%02X", byte7))

	// 字节8: 校验和（前面7个字节的累加和的低8位）
	checksum := byte(byte1) + byte(byte2) + byte(byte3) + byte(byte4) + byte(byte5) + byte(byte6) + byte(byte7)

	cmdBytes := []byte{
		byte1,
		byte2,
		byte3,
		byte4,
		byte5,
		byte6,
		byte7,
		checksum,
	}

	return hex.EncodeToString(cmdBytes)
}

// SendPTZCommand 发送云台控制命令到设备（使用十六进制字符串格式）
func (s *Server) SendPTZCommand(deviceID, channelID, ptzCmd string) error {
	dev, ok := s.memoryStorer.Load(deviceID)
	if !ok {
		return fmt.Errorf("设备不存在: %s", deviceID)
	}

	if !dev.IsOnline || dev.conn == nil {
		return fmt.Errorf("设备离线或连接不可用")
	}

	// 验证通道是否存在
	if _, ok := dev.GetChannel(channelID); !ok {
		return fmt.Errorf("通道不存在: %s", channelID)
	}

	// 但 XML 中的 DeviceID 应该是目标通道
	// 直接使用设备的 to、conn、source

	// 构建云台控制 XML
	ptzReq := NewPTZControlRequest(channelID, ptzCmd)
	body := ptzReq.Marshal()

	// 详细日志：输出完整的 PTZ 命令信息
	slog.Info("发送 PTZ 命令详情（十六进制格式）",
		"device_id", deviceID,
		"channel_id", channelID,
		"ptz_cmd_hex", ptzCmd,
		"xml_body", string(body),
		"device_online", dev.IsOnline,
		"device_conn", dev.conn != nil,
		"device_to", dev.to.String(),
		"xml_device_id", channelID)

	// 使用设备的 to 地址发送 SIP MESSAGE（海康特殊要求）
	contentType := sip.ContentType("Application/MANSCDP+xml")
	tx, err := s.wrapRequest(dev, sip.MethodMessage, &contentType, body)
	if err != nil {
		return fmt.Errorf("发送云台控制命令失败: %w", err)
	}

	resp, err := sipResponse(tx)
	if err != nil {
		return fmt.Errorf("云台控制响应错误: %w", err)
	}

	if resp.StatusCode() != http.StatusOK {
		return fmt.Errorf("云台控制失败: %d %s", resp.StatusCode(), resp.Reason())
	}

	slog.Info("云台控制成功",
		"device_id", deviceID,
		"channel_id", channelID,
		"ptz_cmd", ptzCmd)

	return nil
}

// SendPTZCommandStructured 发送结构化 PTZ 命令到设备（海康等厂家可能需要）
func (s *Server) SendPTZCommandStructured(deviceID, channelID, code string, speed int) error {
	dev, ok := s.memoryStorer.Load(deviceID)
	if !ok {
		return fmt.Errorf("设备不存在: %s", deviceID)
	}

	if !dev.IsOnline || dev.conn == nil {
		return fmt.Errorf("设备离线或连接不可用")
	}

	// 构建结构化云台控制 XML
	ptzReq := NewPTZControlRequestStructured(channelID, code, speed)
	body := ptzReq.Marshal()

	// 详细日志：输出完整的 PTZ 命令信息
	slog.Info("发送 PTZ 命令详情（结构化格式）",
		"device_id", deviceID,
		"channel_id", channelID,
		"code", code,
		"speed", speed,
		"xml_body", string(body),
		"device_online", dev.IsOnline,
		"device_conn", dev.conn != nil)

	// 使用 wrapRequest 发送 SIP MESSAGE
	contentType := sip.ContentType("Application/MANSCDP+xml")
	tx, err := s.wrapRequest(dev, sip.MethodMessage, &contentType, body)
	if err != nil {
		return fmt.Errorf("发送云台控制命令失败: %w", err)
	}

	resp, err := sipResponse(tx)
	if err != nil {
		return fmt.Errorf("云台控制响应错误: %w", err)
	}

	if resp.StatusCode() != http.StatusOK {
		return fmt.Errorf("云台控制失败: %d %s", resp.StatusCode(), resp.Reason())
	}

	slog.Info("云台控制成功",
		"device_id", deviceID,
		"channel_id", channelID,
		"code", code,
		"speed", speed)

	return nil
}
