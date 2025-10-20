package conf

import "time"

type Bootstrap struct {
	Debug        bool   `toml:"-" json:"-"`
	BuildVersion string `toml:"-" json:"-"`
	ConfigDir    string `toml:"-" json:"-"`
	ConfigPath   string `toml:"-" json:"-"`

	Server Server // 服务器
	Data   Data   // 数据
	Log    Log    // 日志
	Sip    SIP
	Media  Media // 媒体
}

type Server struct {
	Debug      bool
	RTMPSecret string `comment:"rtmp 推流秘钥"`

	Username string `comment:"登录用户名"`
	Password string `comment:"登录密码"`

	HTTP ServerHTTP `comment:"对外提供的服务，建议由 nginx 代理"` // HTTP服务器
}

type ServerHTTP struct {
	Port      int         `comment:"http 端口"`                // 服务器端口号
	Timeout   Duration    `comment:"请求超时时间"`                 // 请求超时时间
	JwtSecret string      `comment:"jwt 秘钥，空串时，每次启动程序将随机赋值"` // JWT密钥
	PProf     ServerPPROF // Pprof配置
}

// ServerPPROF 结构体，包含 Enabled 和 AccessIps 两个字段
type ServerPPROF struct {
	Enabled   bool     `comment:"是否启用 pprof, 建议设置为 true"`  // 是否启用
	AccessIps []string `comment:"访问白名单" json:"access_ips"` // 允许访问的IP地址列表
}

// Data 结构体，包含 Database 和 Redis 两个字段
type Data struct {
	// Database 数据库
	Database Database `comment:"数据库支持 sqlite/postgres/mysql, 使用 sqlite 时 dsn 应当填写文件存储路径"`
	// Redis Redis数据库
	// Redis DataRedis
}

// Database 结构体，包含 Dsn、MaxIdleConns、MaxOpenConns、ConnMaxLifetime 和 SlowThreshold 五个字段
type Database struct {
	Dsn             string   // 数据源名称
	MaxIdleConns    int32    // 最大空闲连接数
	MaxOpenConns    int32    // 最大打开连接数
	ConnMaxLifetime Duration // 连接最大生命周期
	SlowThreshold   Duration // 慢查询阈值
}

// Log 结构体，包含 Dir、Level、MaxAge、RotationTime 和 RotationSize 五个字段
type Log struct {
	Dir          string   `comment:"日志存储目录，不能使用特殊符号"`
	Level        string   `comment:"记录级别 debug/info/warn/error"`
	MaxAge       Duration `comment:"保留日志多久，超过时间自动删除"`
	RotationTime Duration `comment:"多久时间，分割一个新的日志文件"`
	RotationSize int64    `comment:"多大文件，分割一个新的日志文件(MB)"`
}

type SIP struct {
	Port     int    `comment:"服务监听的 tcp/udp 端口号" json:"port"`
	ID       string `comment:"gb/t28181 20 位国标 ID" json:"id"`
	Domain   string `comment:"域" json:"domain"`
	Password string `comment:"注册密码" json:"password"`
}

type Media struct {
	IP           string `comment:"媒体服务器 IP"`
	HTTPPort     int    `comment:"媒体服务器 HTTP 端口"`
	Secret       string `comment:"媒体服务器密钥"`
	WebHookIP    string `comment:"用于流媒体 webhook 回调"`
	RTPPortRange string `comment:"媒体服务器 RTP 端口范围"`
	SDPIP        string `comment:"媒体服务器 SDP IP"`
}

type Duration time.Duration

func (d *Duration) UnmarshalText(b []byte) error {
	x, err := time.ParseDuration(string(b))
	if err != nil {
		return err
	}
	*d = Duration(x)
	return nil
}

func (d Duration) MarshalText() ([]byte, error) {
	return []byte(d.Duration().String()), nil
}

func (d *Duration) Duration() time.Duration {
	return time.Duration(*d)
}
