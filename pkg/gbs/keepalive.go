package gbs

import (
	"github.com/gowvp/gb28181/internal/core/ipc"
	"github.com/gowvp/gb28181/pkg/gbs/sip"
	"github.com/ixugo/goddd/pkg/orm"
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

	// 程序重启时会丢内存，收到 keepalive 时，补上
	// 并未补充到
	g.svr.memoryStorer.LoadOrStore(ctx.DeviceID, &Device{
		conn:   ctx.Request.GetConnection(),
		source: ctx.Source,
		to:     ctx.To,
	})

	if err := g.svr.memoryStorer.Change(ctx.DeviceID, func(d *ipc.Device) {
		d.KeepaliveAt = orm.Now()
		d.IsOnline = msg.Status == "OK" || msg.Status == "ON"
		d.Address = ctx.Source.String()
		d.Transport = ctx.Source.Network()
	}, func(d *Device) {
		d.conn = ctx.Request.GetConnection()
		d.source = ctx.Source
		d.to = ctx.To
	}); err != nil {
		ctx.Log.Error("keepalive", "err", err)
	}

	ctx.String(200, "OK")
}
