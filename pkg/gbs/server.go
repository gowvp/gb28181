package gbs

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"

	"github.com/gowvp/owl/internal/conf"
	"github.com/gowvp/owl/internal/core/ipc"
	"github.com/gowvp/owl/internal/core/sms"
	"github.com/gowvp/owl/pkg/gbs/m"
	"github.com/gowvp/owl/pkg/gbs/sip"
	"github.com/ixugo/goddd/pkg/conc"
	"github.com/ixugo/netpulse/ip"
)

type MemoryStorer interface {
	LoadOrStore(deviceID string, value *Device)
	LoadDeviceToMemory(conn sip.Connection)               // 加载设备到内存
	RangeDevices(fn func(key string, value *Device) bool) // 遍历设备

	Change(deviceID string, changeFn func(*ipc.Device) error, changeFn2 func(*Device)) error // 登出设备

	Load(deviceID string) (*Device, bool)
	Store(deviceID string, value *Device)
	DeleteDevice(deviceID string)
	GetChannel(deviceID, channelID string) (*Channel, bool)
}

type Server struct {
	*sip.Server
	gb           *GB28181API
	mediaService sms.Core

	fromAddress  sip.Address
	memoryStorer MemoryStorer
}

// resolveHost 将 host 统一为 IP：域名做 DNS 解析，IP 直接返回
func resolveHost(host string) string {
	if host == "" {
		return ""
	}
	if net.ParseIP(host) != nil {
		return host
	}
	addrs, err := net.LookupHost(host)
	if err != nil || len(addrs) == 0 {
		slog.Warn("resolveHost failed, fallback to raw host", "host", host, "err", err)
		return host
	}
	slog.Debug("resolveHost", "host", host, "resolved", addrs[0])
	return addrs[0]
}

func NewServer(cfg *conf.Bootstrap, store ipc.Adapter, sc sms.Core) (*Server, func()) {
	api := NewGB28181API(cfg, store, sc.NodeManager)

	// 优先使用配置的公网地址，空时回退内网探测；域名自动解析为 IP
	sipHost := resolveHost(cfg.Sip.Host)
	if sipHost == "" {
		sipHost = ip.InternalIP()
	}
	uri, _ := sip.ParseSipURI(fmt.Sprintf("sip:%s@%s:%d", cfg.Sip.ID, sipHost, cfg.Sip.Port))
	from := sip.Address{
		DisplayName: sip.String{Str: "gowvp/owl"},
		URI:         &uri,
		Params:      sip.NewParams(),
	}

	svr = sip.NewServer(&from)

	svr.Use(func(c *sip.Context) {
		if filterUnknowDevices(c.DeviceID) != nil {
			c.Abort()
			return
		}
		c.Next()
	})
	svr.Register(api.handlerRegister)
	msg := svr.Message()
	msg.Handle("Keepalive", api.sipMessageKeepalive)
	msg.Handle("Catalog", api.sipMessageCatalog)
	msg.Handle("DeviceInfo", api.sipMessageDeviceInfo)
	msg.Handle("ConfigDownload", api.sipMessageConfigDownload)
	msg.Handle("DeviceConfig", api.handleDeviceConfig)
	// msg.Handle("RecordInfo", api.handlerMessage)

	c := Server{
		Server:       svr,
		mediaService: sc,
		fromAddress:  from,
		gb:           api,
		memoryStorer: store.Store().(MemoryStorer),
	}
	api.svr = &c

	go svr.ListenUDPServer(fmt.Sprintf(":%d", cfg.Sip.Port))
	go svr.ListenTCPServer(fmt.Sprintf(":%d", cfg.Sip.Port))
	go c.startTickerCheck()
	// 等待 UDP 连接
	for {
		time.Sleep(50 * time.Millisecond)
		if svr.UDPConn() != nil {
			c.memoryStorer.LoadDeviceToMemory(svr.UDPConn())
			break
		}
	}
	return &c, c.Close
}

// SetConfig 热更新 SIP 配置，用于配置变更时更新 from 地址而无需重启服务
func (s *Server) SetConfig() {
	cfg := s.gb.cfg
	sipHost := resolveHost(cfg.Host)
	if sipHost == "" {
		sipHost = ip.InternalIP()
	}
	uri, _ := sip.ParseSipURI(fmt.Sprintf("sip:%s@%s:%d", cfg.ID, sipHost, cfg.Port))
	from := sip.Address{
		DisplayName: sip.String{Str: "gowvp/owl"},
		URI:         &uri,
		Params:      sip.NewParams(),
	}
	s.fromAddress = from
	s.Server.SetFrom(&from)
}

// startTickerCheck 定时检查离线，通过心跳超时判断设备是否离线
func (s *Server) startTickerCheck() {
	conc.Timer(context.Background(), 60*time.Second, time.Second, func() {
		now := time.Now()
		s.memoryStorer.RangeDevices(func(key string, dev *Device) bool {
			if !dev.IsOnline {
				return true
			}
			if len(key) < 18 {
				return true
			}

			// 计算超时时间：心跳间隔 * 超时次数
			// 默认心跳间隔 60s，超时次数 3 次，即 3 分钟无心跳判定离线
			interval := dev.keepaliveInterval
			if interval == 0 {
				interval = 60
			}
			timeoutCount := dev.keepaliveTimeout
			if timeoutCount == 0 {
				timeoutCount = 3
			}
			timeout := time.Duration(interval) * time.Duration(timeoutCount) * time.Second

			// 跳过未收到过心跳的设备（LastKeepaliveAt 为零值），这类设备依赖注册超时处理
			if dev.LastKeepaliveAt.IsZero() {
				// 如果注册时间也超过了超时时间，则判定离线
				if !dev.LastRegisterAt.IsZero() && now.Sub(dev.LastRegisterAt) >= timeout {
					if err := s.gb.logout(key, func(d *ipc.Device) error {
						d.IsOnline = false
						return nil
					}); err != nil {
						slog.Error("logout device failed", "device_id", key, "err", err)
					}
				}
				return true
			}

			// 心跳超时或连接丢失，判定设备离线
			if sub := now.Sub(dev.LastKeepaliveAt); sub >= timeout || dev.conn == nil {
				slog.Info("device offline detected",
					"device_id", key,
					"last_keepalive", dev.LastKeepaliveAt,
					"timeout", timeout,
					"elapsed", sub,
					"conn_nil", dev.conn == nil,
				)
				if err := s.gb.logout(key, func(d *ipc.Device) error {
					d.IsOnline = false
					return nil
				}); err != nil {
					slog.Error("logout device failed", "device_id", key, "err", err)
				}
			}
			return true
		})
	})
}

// MODDEBUG MODDEBUG
var MODDEBUG = "DEBUG"

// ActiveDevices 记录当前活跃设备，请求播放时设备必须处于活跃状态
type ActiveDevices struct {
	sync.Map
}

// Get Get
func (a *ActiveDevices) Get(key string) (Devices, bool) {
	if v, ok := a.Load(key); ok {
		return v.(Devices), ok
	}
	return Devices{}, false
}

var _activeDevices ActiveDevices

// 系统运行信息
var (
	_sysinfo *m.SysInfo
	config   *m.Config
)

func LoadSYSInfo() {
	config = m.MConfig
	_activeDevices = ActiveDevices{sync.Map{}}

	StreamList = streamsList{&sync.Map{}, &sync.Map{}, 0}
	_recordList = &sync.Map{}
	RecordList = apiRecordList{items: map[string]*apiRecordItem{}, l: sync.RWMutex{}}

	// init sysinfo
	// _sysinfo = &m.SysInfo{}
	// if err := db.Get(db.DBClient, _sysinfo); err != nil {
	// 	if db.RecordNotFound(err) {
	// 		//  初始不存在
	// 		_sysinfo = m.DefaultInfo()

	// 		if err = db.Create(db.DBClient, _sysinfo); err != nil {
	// 			// logrus.Fatalf("1 init sysinfo err:%v", err)
	// 		}
	// 	} else {
	// 		// logrus.Fatalf("2 init sysinfo err:%v", err)
	// 	}
	// }
	m.MConfig.GB28181 = _sysinfo

	// uri, _ := sip.ParseSipURI(fmt.Sprintf("sip:%s@%s", _sysinfo.LID, _sysinfo.Region))
	_serverDevices = Devices{
		DeviceID: _sysinfo.LID,
		// Region:   _sysinfo.Region,
		addr: &sip.Address{
			DisplayName: sip.String{Str: "sipserver"},
			// URI:         &uri,
			Params: sip.NewParams(),
		},
	}

	// init media
	url, err := url.Parse(config.Media.RTP)
	if err != nil {
		// logrus.Fatalf("media rtp url error,url:%s,err:%v", config.Media.RTP, err)
	}
	ipaddr, err := net.ResolveIPAddr("ip", url.Hostname())
	if err != nil {
		// logrus.Fatalf("media rtp url error,url:%s,err:%v", config.Media.RTP, err)
	}
	_sysinfo.MediaServerRtpIP = ipaddr.IP
	_sysinfo.MediaServerRtpPort, _ = strconv.Atoi(url.Port())
}

// zlm接收到的ssrc为16进制。发起请求的ssrc为10进制
func ssrc2stream(ssrc string) string {
	if ssrc[0:1] == "0" {
		ssrc = ssrc[1:]
	}
	num, _ := strconv.Atoi(ssrc)
	return fmt.Sprintf("%08X", num)
}

func sipResponse(tx *sip.Transaction) (*sip.Response, error) {
	response := tx.GetResponse()
	if response == nil {
		return nil, sip.NewError(nil, "response timeout", "tx key:", tx.Key())
	}
	if response.StatusCode() != http.StatusOK {
		return response, sip.NewError(nil, "device: ", response.StatusCode(), " ", response.Reason())
	}
	return response, nil
}

// QueryCatalog 查询 catalog
func (s *Server) QueryCatalog(deviceID string) error {
	return s.gb.QueryCatalog(deviceID)
}

func (s *Server) Play(in *PlayInput) error {
	return s.gb.Play(in)
}

func (s *Server) StopPlay(ctx context.Context, in *StopPlayInput) error {
	return s.gb.StopPlay(ctx, in)
}

// QuerySnapshot 厂商实现抓图的少，sip 层已实现，先搁置
func (s *Server) QuerySnapshot(deviceID, channelID string) error {
	return s.gb.QuerySnapshot(deviceID, channelID)
}

// PTZControl 云台控制
func (s *Server) PTZControl(deviceID, channelID, ptzCmd string) error {
	return s.SendPTZCommand(deviceID, channelID, ptzCmd)
}
