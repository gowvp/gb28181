package gbs

import (
	"github.com/gowvp/gb28181/internal/core/gb28181"
	"github.com/gowvp/gb28181/pkg/gbs/sip"
	"github.com/ixugo/goweb/pkg/orm"
	// "github.com/panjjo/gosip/db"
)

// MessageNotify 心跳包xml结构
type MessageNotify struct {
	CmdType  string `xml:"CmdType"`
	SN       int    `xml:"SN"`
	DeviceID string `xml:"DeviceID"`
	Status   string `xml:"Status"`
	Info     string `xml:"Info"`
}

func (g *GB28181API) sipMessageKeepalive(ctx *sip.Context) {
	var msg MessageNotify
	if err := sip.XMLDecode(ctx.Request.Body(), &msg); err != nil {
		ctx.Log.Error("Message Unmarshal xml err", "err", err)
		return
	}

	// device, ok := _activeDevices.Get(ctx.DeviceID)
	// if !ok {
	// device = Devices{DeviceID: ctx.DeviceID}
	// if err := db.Get(db.DBClient, &device); err != nil {
	// logrus.Warnln("Device Keepalive not found ", u.DeviceID, err)
	// }
	// }

	ipc, ok := g.svr.memoryStorer.Load(ctx.DeviceID)
	if ok {
		g.svr.memoryStorer.Store(ctx.DeviceID, ipc)
	}

	if err := g.svr.memoryStorer.Change(ctx.DeviceID, func(d *gb28181.Device) {
		d.KeepaliveAt = orm.Now()
		d.IsOnline = msg.Status == "OK" || msg.Status == "ON"
		d.Address = ctx.Source.String()
		d.Trasnport = ctx.Source.Network()
	}, func(d *Device) {
	}); err != nil {
		ctx.Log.Error("keepalive", "err", err)
	}

	ctx.String(200, "OK")
}
