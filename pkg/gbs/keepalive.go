package gbs

import (
	"log/slog"

	"github.com/gowvp/owl/internal/core/ipc"
	"github.com/gowvp/owl/pkg/gbs/sip"
	"github.com/ixugo/goddd/pkg/orm"
)

// MessageNotify 心跳包 XML 结构
type MessageNotify struct {
	CmdType  string `xml:"CmdType"`
	SN       int    `xml:"SN"`
	DeviceID string `xml:"DeviceID"`
}

func (g *GB28181API) sipMessageKeepalive(ctx *sip.Context) {
	var msg MessageNotify
	if err := sip.XMLDecode(ctx.Request.Body(), &msg); err != nil {
		ctx.Log.Error("Message Unmarshal xml err", "err", err)
		return
	}

	// 程序重启后内存丢失，收到 keepalive 时补上
	_, alreadyLoaded := g.svr.memoryStorer.Load(ctx.DeviceID)
	g.svr.memoryStorer.LoadOrStore(ctx.DeviceID, &Device{
		conn:   ctx.Request.GetConnection(),
		source: ctx.Source,
		to:     ctx.To,
		region: ctx.To.URI.Host(),
	})

	if err := g.svr.memoryStorer.Change(ctx.DeviceID, func(d *ipc.Device) error {
		d.KeepaliveAt = orm.Now()
		d.IsOnline = true // 收到心跳即视为在线，不依赖 Status 字段值
		d.Address = ctx.Source.String()
		d.Transport = ctx.Source.Network()
		return nil
	}, func(d *Device) {
		d.conn = ctx.Request.GetConnection()
		d.source = ctx.Source
		d.to = ctx.To
		d.region = ctx.To.URI.Host()
	}); err != nil {
		ctx.Log.Error("keepalive", "err", err)
	}

	// 必须在 Change(IsOnline=true) 之后触发，QueryCatalog 会检查 IsOnline
	if !alreadyLoaded {
		slog.Info("keepalive 触发 Catalog 补载", "device_id", ctx.DeviceID)
		_ = g.QueryCatalog(ctx.DeviceID)
	}

	ctx.String(200, "OK")
}
