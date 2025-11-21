package gbs

import (
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"
	"unicode"

	"github.com/gowvp/gb28181/internal/conf"
	gb28181 "github.com/gowvp/gb28181/internal/core/ipc"
	"github.com/gowvp/gb28181/internal/core/sms"
	"github.com/gowvp/gb28181/pkg/gbs/sip"
	"github.com/ixugo/goddd/pkg/conc"
	"github.com/ixugo/goddd/pkg/orm"
)

const ignorePassword = "#"

type GB28181API struct {
	cfg  *conf.SIP
	core gb28181.GBDAdapter

	catalog *sip.Collector[Channels]

	// TODO: 待替换成 redis
	streams *conc.Map[string, *Streams]

	svr *Server

	sms *sms.NodeManager
}

func NewGB28181API(cfg *conf.Bootstrap, store gb28181.GBDAdapter, sms *sms.NodeManager) *GB28181API {
	g := GB28181API{
		cfg:  &cfg.Sip,
		core: store,
		sms:  sms,
		catalog: sip.NewCollector(func(c1, c2 *Channels) bool {
			return c1.ChannelID == c2.ChannelID
		}),
		streams: &conc.Map[string, *Streams]{},
	}
	go g.catalog.Start(func(s string, channel []*Channels) {
		// 零值不做变更，没有通道又何必注册上来
		if len(channel) == 0 {
			return
		}

		// ipc, ok := g.svr.devices.Load(s)
		// if ok {
		// 	ipc.channels.Clear()
		// 	for _, ch := range c {

		// 	}
		// }

		ipc, ok := g.svr.memoryStorer.Load(s)
		if ok {
			for _, ch := range channel {
				ch := Channel{
					ChannelID: ch.ChannelID,
					device:    ipc,
				}
				ch.init(g.cfg.Domain)
				ipc.Channels.Store(ch.ChannelID, &ch)
			}
		}

		out := make([]*gb28181.Channel, len(channel))
		for i, ch := range channel {
			out[i] = &gb28181.Channel{
				DeviceID:  s,
				ChannelID: ch.ChannelID,
				Name:      ch.Name,
				IsOnline:  ch.Status == "OK" || ch.Status == "ON",
				Ext: gb28181.DeviceExt{
					Manufacturer: ch.Manufacturer,
					Model:        ch.Model,
				},
			}
		}
		g.core.SaveChannels(out)
	})
	return &g
}

// filterUnknowDevices 国标 ID 校验，正常是长度为 20 的纯数字字符串
func filterUnknowDevices(deviceID string) error {
	if len(deviceID) < 18 {
		return fmt.Errorf("device id too short")
	}
	if len(deviceID) > 20 {
		return fmt.Errorf("device id too long")
	}
	// 验证必须全是数字
	for _, ch := range deviceID {
		if !unicode.IsNumber(ch) {
			return fmt.Errorf("device id must be all numbers")
		}
	}
	return nil
}

func (g *GB28181API) handlerRegister(ctx *sip.Context) {
	if err := filterUnknowDevices(ctx.DeviceID); err != nil {
		slog.Error("过滤设备，拒绝注册", "device_id", ctx.DeviceID, "err", err)
		ctx.String(http.StatusBadRequest, err.Error())
		return
	}

	dev, err := g.core.GetDeviceByDeviceID(ctx.DeviceID)
	if err != nil {
		ctx.Log.Error("GetDeviceByDeviceID", "err", err)
		ctx.String(http.StatusInternalServerError, "server db error")
		return
	}
	g.svr.memoryStorer.LoadOrStore(ctx.DeviceID, &Device{
		conn:   ctx.Request.GetConnection(),
		source: ctx.Source,
		to:     ctx.To,
	})

	password := dev.Password
	if password == "" {
		password = g.cfg.Password
	}
	// 免鉴权
	if dev.Password == ignorePassword {
		password = ""
	}

	hdrs := ctx.Request.GetHeaders("Authorization")
	if len(hdrs) == 0 {
		resp := sip.NewResponseFromRequest("", ctx.Request, http.StatusUnauthorized, http.StatusText(http.StatusUnauthorized), nil)
		resp.AppendHeader(&sip.GenericHeader{HeaderName: "WWW-Authenticate", Contents: fmt.Sprintf(`Digest realm="%s",qop="auth",nonce="%s"`, g.cfg.Domain, sip.RandString(32))})
		_ = ctx.Tx.Respond(resp)
		return
	}

	if password != "" {
		authenticateHeader := hdrs[0].(*sip.GenericHeader)
		auth := sip.AuthFromValue(authenticateHeader.Contents)
		auth.SetPassword(password)
		auth.SetUsername(dev.DeviceID)
		auth.SetMethod(ctx.Request.Method())
		auth.SetURI(auth.Get("uri"))
		if auth.CalcResponse() != auth.Get("response") {
			ctx.Log.Info("设备注册鉴权失败")
			ctx.String(http.StatusUnauthorized, "wrong password")
			return
		}
	}

	respFn := func() {
		resp := sip.NewResponseFromRequest("", ctx.Request, http.StatusOK, "OK", nil)
		resp.AppendHeader(&sip.GenericHeader{
			HeaderName: "Date",
			Contents:   time.Now().Format("2006-01-02T15:04:05.000"),
		})
		_ = ctx.Tx.Respond(resp)
	}

	expire := ctx.GetHeader("Expires")
	if expire == "0" {
		ctx.Log.Info("设备注销")
		g.logout(ctx.DeviceID, func(b *gb28181.Device) {
			b.IsOnline = false
			b.Address = ctx.Source.String()
		})
		respFn()
		return
	}

	g.login(ctx, func(b *gb28181.Device) {
		b.IsOnline = true
		b.RegisteredAt = orm.Now()
		b.KeepaliveAt = orm.Now()
		b.Expires, _ = strconv.Atoi(expire)
		b.Address = ctx.Source.String()
		b.Transport = ctx.Source.Network()
		b.Ext.GBVersion = ctx.XGBVer
	})

	// conn := ctx.Request.GetConnection()
	// fmt.Printf(">>> %p\n", conn

	ctx.Log.Info("设备注册成功")
	// ctx.Log.Debug("device info", "source", ctx.Source, "host", ctx.Host)

	respFn()

	g.QueryDeviceInfo(ctx)
	_ = g.QueryCatalog(dev.DeviceID)
	_ = g.QueryConfigDownloadBasic(dev.DeviceID)
}

func (g GB28181API) login(ctx *sip.Context, fn func(d *gb28181.Device)) {
	slog.Info("status change 设备上线", "device_id", ctx.DeviceID)
	g.svr.memoryStorer.Change(ctx.DeviceID, fn, func(d *Device) {
		d.conn = ctx.Request.GetConnection()
		d.source = ctx.Source
		d.to = ctx.To
	})
}

func (g GB28181API) logout(deviceID string, changeFn func(*gb28181.Device)) error {
	slog.Info("status change 设备离线", "device_id", deviceID)
	return g.svr.memoryStorer.Change(deviceID, changeFn, func(d *Device) {
		d.Expires = 0
		d.IsOnline = false
	})
}
