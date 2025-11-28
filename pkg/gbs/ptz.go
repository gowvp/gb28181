package gbs

import (
	"encoding/xml"
	"fmt"

	"github.com/gowvp/gb28181/pkg/gbs/sip"
)

// PTZCmd PTZ 云台控制指令
// 根据 GB/T 28181-2016 标准附录 A.3.1
type PTZCmd struct {
	// 指令字节1: 0xA5
	// 指令字节2: 组合码1(高4位) + 版本号(低4位,固定为0)
	// 指令字节3: 组合码2(高4位) + PTZ设备地址(低4位)
	// 指令字节4: 控制指令(高4位) + 停止码(低4位)
	// 指令字节5: 水平速度(0-255)
	// 指令字节6: 垂直速度(0-255)
	// 指令字节7: 变倍速度(高4位) + 保留(低4位)
	// 指令字节8: 校验码(前7字节相加取低8位)
}

const (
	// PTZ 方向控制
	PTZCmdStop      = 0x00 // 停止
	PTZCmdRight     = 0x01 // 右移
	PTZCmdLeft      = 0x02 // 左移
	PTZCmdDown      = 0x04 // 下移
	PTZCmdUp        = 0x08 // 上移
	PTZCmdZoomIn    = 0x10 // 放大
	PTZCmdZoomOut   = 0x20 // 缩小
	PTZCmdLeftUp    = 0x0A // 左上
	PTZCmdLeftDown  = 0x06 // 左下
	PTZCmdRightUp   = 0x09 // 右上
	PTZCmdRightDown = 0x05 // 右下

	// 镜头控制
	PTZCmdIrisIn  = 0x44 // 光圈放大
	PTZCmdIrisOut = 0x48 // 光圈缩小
	PTZCmdFocusIn = 0x41 // 聚焦+
	PTZCmdFocusOut = 0x42 // 聚焦-

	// 预置位控制
	PTZCmdPresetSet    = 0x81 // 设置预置位
	PTZCmdPresetCall   = 0x82 // 调用预置位
	PTZCmdPresetDelete = 0x83 // 删除预置位
)

// PTZControlRequest PTZ 控制请求
type PTZControlRequest struct {
	XMLName   xml.Name `xml:"Control"`
	CmdType   string   `xml:"CmdType"`
	SN        int      `xml:"SN"`
	DeviceID  string   `xml:"DeviceID"`
	PTZCmd    string   `xml:"PTZCmd"`
}

// BuildPTZCmd 构建 PTZ 控制指令
// cmd: 控制指令
// hSpeed: 水平速度 (0-255)
// vSpeed: 垂直速度 (0-255)
// zSpeed: 变倍速度 (0-15)
func BuildPTZCmd(cmd byte, hSpeed, vSpeed, zSpeed byte) string {
	// 字节1: 固定 0xA5
	b1 := byte(0xA5)
	// 字节2: 组合码1(高4位=0) + 版本号(低4位=0)
	b2 := byte(0x0F)
	// 字节3: 组合码2(高4位=0) + PTZ设备地址(低4位=1)
	b3 := byte(0x01)
	// 字节4: 控制指令
	b4 := cmd
	// 字节5: 水平速度
	b5 := hSpeed
	// 字节6: 垂直速度
	b6 := vSpeed
	// 字节7: 变倍速度(高4位) + 保留(低4位=0)
	b7 := (zSpeed & 0x0F) << 4
	// 字节8: 校验码
	b8 := (b1 + b2 + b3 + b4 + b5 + b6 + b7) & 0xFF

	return fmt.Sprintf("A50F01%02X%02X%02X%02X%02X", b4, b5, b6, b7, b8)
}

// BuildPresetCmd 构建预置位控制指令
// cmd: 预置位控制指令 (PTZCmdPresetSet/PTZCmdPresetCall/PTZCmdPresetDelete)
// presetIndex: 预置位编号 (1-255)
func BuildPresetCmd(cmd byte, presetIndex byte) string {
	b1 := byte(0xA5)
	b2 := byte(0x0F)
	b3 := byte(0x01)
	b4 := cmd
	b5 := byte(0x00)
	b6 := presetIndex
	b7 := byte(0x00)
	b8 := (b1 + b2 + b3 + b4 + b5 + b6 + b7) & 0xFF

	return fmt.Sprintf("A50F01%02X%02X%02X%02X%02X", b4, b5, b6, b7, b8)
}

// PTZControl 云台控制
func (g *GB28181API) PTZControl(deviceID, channelID string, ptzCmd string) error {
	ch, ok := g.svr.memoryStorer.GetChannel(deviceID, channelID)
	if !ok {
		return ErrChannelNotExist
	}

	req := PTZControlRequest{
		CmdType:  "DeviceControl",
		SN:       sip.RandInt(100000, 999999),
		DeviceID: channelID,
		PTZCmd:   ptzCmd,
	}

	body, err := xml.Marshal(req)
	if err != nil {
		return err
	}

	_, err = g.svr.wrapRequest(ch, sip.MethodMessage, &sip.ContentTypeXML, body, nil)
	return err
}

// PTZControl 云台控制 (Server 方法)
func (s *Server) PTZControl(deviceID, channelID string, ptzCmd string) error {
	return s.gb.PTZControl(deviceID, channelID, ptzCmd)
}
