package gbs

import (
	"fmt"
	"log/slog"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/gowvp/owl/internal/core/ipc"
	"github.com/gowvp/owl/pkg/gbs/m"
	"github.com/gowvp/owl/pkg/gbs/sip"
	"github.com/ixugo/goddd/pkg/conc"
)

// sip服务用户信息
var svr *sip.Server

type Device struct {
	Channels conc.Map[string, *Channel]

	registerWithKeepaliveMutex sync.Mutex
	// 播放互斥锁也可以移动到 channel 属性
	playMutex sync.Mutex

	IsOnline bool
	Address  string
	Password string
	// 设备注册时 From 头的 SIP 域名，用于构造 INVITE To URI
	region string

	conn   sip.Connection
	source net.Addr
	to     *sip.Address

	LastKeepaliveAt time.Time
	LastRegisterAt  time.Time
	Expires         int

	keepaliveInterval uint16
	keepaliveTimeout  uint16
}

func NewDevice(conn sip.Connection, d *ipc.Device) *Device {
	uri, err := sip.ParseURI(fmt.Sprintf("sip:%s@%s", d.GetGB28181DeviceID(), d.Address))
	if err != nil {
		slog.Error("parse uri", "err", err, "did", d.ID)
		return nil
	}

	addr, err := net.ResolveUDPAddr("udp", d.Address)
	if err != nil {
		slog.Error("resolve udp addr", "err", err, "did", d.ID)
		return nil
	}

	// DB 重建时用 DeviceID 前 10 位推导 SIP 域名，设备重新注册后会被实际值覆盖
	region := d.Address
	if did := d.GetGB28181DeviceID(); len(did) >= 10 {
		region = did[:10]
	}

	c := Device{
		conn:   conn,
		source: addr,
		to: &sip.Address{
			URI:    uri,
			Params: sip.NewParams(),
		},
		Address:         d.Address,
		region:          region,
		LastKeepaliveAt: d.KeepaliveAt.Time,
		LastRegisterAt:  d.RegisteredAt.Time,
		IsOnline:        d.IsOnline,
		Password:        d.Password,
	}

	return &c
}

// CheckConnection 检查 udp 设备能否通信
func (d *Device) CheckConnection() error {
	const timeout = 2 * time.Second

	if d.source.Network() == "tcp" {
		return nil
	}
	// 创建临时UDP连接进行检查
	tempConn, err := net.DialTimeout("udp", d.source.String(), timeout)
	if err != nil {
		return fmt.Errorf("UDP连接失败: %w", err)
	}
	defer tempConn.Close()
	return nil
}

// LoadChannels 从 DB 重建设备通道，domain 取设备注册域名，无则回退 IP:Port
func (d *Device) LoadChannels(channels ...*ipc.Channel) {
	domain := d.region
	if domain == "" {
		domain = d.Address
	}
	for _, channel := range channels {
		ch := Channel{
			ChannelID: channel.ChannelID,
			device:    d,
		}
		ch.init(domain)
		d.Channels.Store(channel.ChannelID, &ch)
	}
}

// Conn implements Targeter.
func (d *Device) Conn() sip.Connection {
	return d.conn
}

// Source implements Targeter.
func (d *Device) Source() net.Addr {
	return d.source
}

// To implements Targeter.
func (d *Device) To() *sip.Address {
	return d.to
}

var _ Targeter = &Device{}

type Channel struct {
	ChannelID string

	uriStr string
	to     *sip.Address

	device *Device
}

// Conn implements Targeter.
func (c *Channel) Conn() sip.Connection {
	return c.device.conn
}

// Source implements Targeter.
func (c *Channel) Source() net.Addr {
	return c.device.source
}

// To implements Targeter.
func (c *Channel) To() *sip.Address {
	return c.to
}

var _ Targeter = &Channel{}

func (c *Channel) init(domain string) {
	c.uriStr = fmt.Sprintf("sip:%s@%s", c.ChannelID, domain)
	uri, _ := sip.ParseURI(c.uriStr)
	c.to = &sip.Address{
		URI:    uri,
		Params: sip.NewParams(),
	}
}

// func NewClient() *Client {
// 	return &Client{
// 		devices: conc.Map[string, *Device]{},
// 	}
// }

// func (c *Client) Store(deviceID string, in *Device) {
// 	v, ok := c.devices.LoadOrStore(deviceID, in)
// 	if ok {
// 		v.conn = in.conn
// 		v.source = in.source
// 		v.to = in.to
// 		v.lastKeepaliveAt = in.lastKeepaliveAt
// 		v.lastRegisterAt = in.lastRegisterAt
// 	}
// }

// func (c *Client) Load(deviceID string) (*Device, bool) {
// 	return c.devices.Load(deviceID)
// }

func (c *Device) GetChannel(channelID string) (*Channel, bool) {
	return c.Channels.Load(channelID)
}

// func (c *Client) Delete(deviceID string) {
// 	c.devices.Delete(deviceID)
// }

// Devices NVR  设备信息
type Devices struct {
	// db.DBModel
	// Name 设备名称
	Name string `json:"name" gorm:"column:name" `
	// DeviceID 设备id
	DeviceID string `json:"deviceid" gorm:"column:deviceid"`
	// Region 设备域
	Region string `json:"region" gorm:"column:region"`
	// Host Via 地址
	Host string `json:"host" gorm:"column:host"`
	// Port via 端口
	Port string `json:"port" gorm:"column:port"`
	// TransPort via transport
	TransPort string `json:"transport" gorm:"column:transport"`
	// Proto 协议
	Proto string `json:"proto" gorm:"column:proto"`
	// Rport via rport
	Rport string `json:"report" gorm:"column:report"`
	// RAddr via recevied
	RAddr string `json:"raddr"  gorm:"column:raddr"`
	// Manufacturer 制造厂商
	Manufacturer string `xml:"Manufacturer"  json:"manufacturer"  gorm:"column:manufacturer"`
	// 设备类型DVR，NVR
	DeviceType string `xml:"DeviceType"  json:"devicetype"  gorm:"column:devicetype"`
	// Firmware 固件版本
	Firmware string ` json:"firmware"  gorm:"column:firmware"`
	// Model 型号
	Model  string `json:"model"  gorm:"column:model"`
	URIStr string `json:"uri"  gorm:"column:uri"`
	// ActiveAt 最后心跳检测时间
	ActiveAt int64 `json:"active" gorm:"column:active"`
	// Regist 是否注册
	Regist bool `json:"regist"  gorm:"column:regist"`
	// PWD 密码
	PWD string `json:"pwd" gorm:"column:pwd"`
	// Source
	Source string `json:"source"  gorm:"column:source"`

	Sys m.SysInfo `json:"sysinfo" gorm:"-"`

	//----
	addr   *sip.Address `gorm:"-"`
	source net.Addr     `gorm:"-"`

	Expire string `json:"-"`
}

// Channels 摄像头通道信息
type Channels struct {
	// db.DBModel
	// ChannelID 通道编码
	ChannelID string `xml:"DeviceID" json:"channelid" gorm:"column:channelid"`
	// DeviceID 设备编号
	DeviceID string `xml:"-" json:"deviceid"  gorm:"column:deviceid"`
	// Memo 备注（用来标示通道信息）
	MeMo string `json:"memo"  gorm:"column:memo"`
	// Name 通道名称（设备端设置名称）
	Name         string `xml:"Name" json:"name"  gorm:"column:name"`
	Manufacturer string `xml:"Manufacturer" json:"manufacturer"  gorm:"column:manufacturer"`
	Model        string `xml:"Model" json:"model"  gorm:"column:model"`
	Owner        string `xml:"Owner"  json:"owner"  gorm:"column:owner"`
	CivilCode    string `xml:"CivilCode" json:"civilcode"  gorm:"column:civilcode"`
	// Address ip地址
	Address     string `xml:"Address"  json:"address"  gorm:"column:address"`
	Parental    int    `xml:"Parental"  json:"parental"  gorm:"column:parental"`
	SafetyWay   int    `xml:"SafetyWay"  json:"safetyway"  gorm:"column:safetyway"`
	RegisterWay int    `xml:"RegisterWay"  json:"registerway"  gorm:"column:registerway"`
	Secrecy     int    `xml:"Secrecy" json:"secrecy"  gorm:"column:secrecy"`
	// Status 状态  on 在线
	Status string `xml:"Status"  json:"status"  gorm:"column:status"`
	// PTZType 云台类型 (厂商扩展字段，非 GB28181 标准必选字段)
	// 取值: 0=无云台, >0=有云台 (具体值含义依赖厂商实现)
	PTZType int `xml:"PTZType" json:"ptztype" gorm:"column:ptztype"`
	// CameraType 摄像机类型 (厂商扩展字段)
	// 取值: 1=球机, 2=半球, 3=固定枪机, 4=遥控枪机
	CameraType int `xml:"CameraType" json:"camera_type" gorm:"-"`
	// Active 最后活跃时间
	Active int64  `json:"active"  gorm:"column:active"`
	URIStr string ` json:"uri"  gorm:"column:uri"`

	// 视频编码格式
	VF string ` json:"vf"  gorm:"column:vf"`
	// 视频高
	Height int `json:"height"  gorm:"column:height"`
	// 视频宽
	Width int `json:"width"  gorm:"column:width"`
	// 视频FPS
	FPS int `json:"fps"  gorm:"column:fps"`
	//  pull 媒体服务器主动拉流，push 监控设备主动推流
	StreamType string `json:"streamtype"  gorm:"column:streamtype"`
	// streamtype=pull时，拉流地址
	URL string `json:"url"  gorm:"column:url"`

	addr *sip.Address `gorm:"-"`
}

// 从请求中解析出设备信息
func parserDevicesFromReqeust(req *sip.Request) (Devices, bool) {
	u := Devices{}
	header, ok := req.From()
	if !ok {
		// logrus.Warningln("not found from header from request", req.String())
		return u, false
	}
	if header.Address == nil {
		// logrus.Warningln("not found from user from request", req.String())
		return u, false
	}
	if header.Address.User() == nil {
		// logrus.Warningln("not found from user from request", req.String())
		return u, false
	}
	u.DeviceID = header.Address.User().String()
	u.Region = header.Address.Host()
	via, ok := req.ViaHop()
	if !ok {
		// logrus.Info("not found ViaHop from request", req.String())
		return u, false
	}
	u.Host = via.Host
	u.Port = via.Port.String()
	report, ok := via.Params.Get("rport")
	if ok && report != nil {
		u.Rport = report.String()
	}
	raddr, ok := via.Params.Get("received")
	if ok && raddr != nil {
		u.RAddr = raddr.String()
	}

	u.TransPort = via.Transport
	u.URIStr = header.Address.String()
	u.addr = sip.NewAddressFromFromHeader(header)
	u.Source = req.Source().String()
	u.source = req.Source()

	headers := req.GetHeaders("Expires")
	if len(headers) != 0 {
		header := headers[0]
		splits := strings.Split(header.String(), ":")
		if len(splits) == 2 {
			u.Expire = splits[1][1:]
		}
	}

	return u, true
}

var deviceStatusMap = map[string]string{
	"ON":     m.DeviceStatusON,
	"OK":     m.DeviceStatusON,
	"ONLINE": m.DeviceStatusON,
	"OFFILE": m.DeviceStatusOFF,
	"OFF":    m.DeviceStatusOFF,
}

func transDeviceStatus(status string) string {
	if v, ok := deviceStatusMap[status]; ok {
		return v
	}
	return status
}
