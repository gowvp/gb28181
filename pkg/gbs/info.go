package gbs

import (
	"encoding/hex"

	"github.com/gowvp/gb28181/internal/core/ipc"
	"github.com/gowvp/gb28181/pkg/gbs/sip"
)

// QueryDeviceInfo 设备信息查询请求
// GB/T28181 81 页 A.2.4.4
func (g GB28181API) QueryDeviceInfo(ctx *sip.Context) {
	tx, err := ctx.SendRequest(sip.MethodMessage, sip.GetDeviceInfoXML(ctx.DeviceID))
	if err != nil {
		ctx.Log.Error("sipDeviceInfo", "err", err)
		return
	}
	if _, err := sipResponse(tx); err != nil {
		ctx.Log.Error("sipResponse", "err", err)
		return
	}
}

// MessageDeviceInfoResponse 设备信息查询应答结构
type MessageDeviceInfoResponse struct {
	CmdType      string `xml:"CmdType"`
	SN           int    `xml:"SN"`
	DeviceID     string `xml:"DeviceID"`     // 目标设备的编码(必选)
	DeviceName   string `xml:"DeviceName"`   // 目标设备的名称(可选
	Manufacturer string `xml:"Manufacturer"` // 设备生产商(可选)
	Model        string `xml:"Model"`        // 设备型号(可选)
	Firmware     string `xml:"Firmware"`     // 设备固件版本(可选)
	Result       string `xml:"Result"`       // 査询结果(必选)
}

// sipMessageDeviceInfo 设备信息查询应答
// GB/T28181 91 页 A.2.6.5
func (g GB28181API) sipMessageDeviceInfo(ctx *sip.Context) {
	var msg MessageDeviceInfoResponse
	if err := sip.XMLDecode(ctx.Request.Body(), &msg); err != nil {
		ctx.Log.Error("sipMessageDeviceInfo", "err", err, "body", hex.EncodeToString(ctx.Request.Body()))
		ctx.String(400, ErrXMLDecode.Error())
		return
	}

	if err := g.core.Edit(ctx.DeviceID, func(d *ipc.Device) {
		d.Ext.Firmware = msg.Firmware
		d.Ext.Manufacturer = msg.Manufacturer
		d.Ext.Model = msg.Model
		d.Ext.Name = msg.DeviceName

		d.Address = ctx.Source.String()
		d.Transport = ctx.Source.Network()
	}); err != nil {
		ctx.Log.Error("Edit", "err", err)
		ctx.String(500, ErrDatabase.Error())
		return
	}

	ctx.String(200, "OK")
}
