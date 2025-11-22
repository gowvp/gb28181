package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/wire"
	"github.com/gowvp/gb28181/internal/adapter/gbadapter"
	"github.com/gowvp/gb28181/internal/adapter/onvifadapter"
	"github.com/gowvp/gb28181/internal/adapter/rtspadapter"
	"github.com/gowvp/gb28181/internal/conf"
	"github.com/gowvp/gb28181/internal/core/ipc"
	"github.com/gowvp/gb28181/internal/core/ipc/store/ipccache"
	"github.com/gowvp/gb28181/internal/core/ipc/store/ipcdb"
	"github.com/gowvp/gb28181/internal/core/proxy"
	"github.com/gowvp/gb28181/internal/core/push"
	"github.com/gowvp/gb28181/internal/core/push/store/pushdb"
	"github.com/gowvp/gb28181/internal/core/sms"
	"github.com/gowvp/gb28181/pkg/gbs"
	"github.com/ixugo/goddd/domain/uniqueid"
	"github.com/ixugo/goddd/domain/uniqueid/store/uniqueiddb"
	"github.com/ixugo/goddd/domain/version/versionapi"
	"github.com/ixugo/goddd/pkg/orm"
	"github.com/ixugo/goddd/pkg/web"
	"gorm.io/gorm"
)

var (
	ProviderVersionSet = wire.NewSet(versionapi.NewVersionCore)
	ProviderSet        = wire.NewSet(
		wire.Struct(new(Usecase), "*"),
		NewHTTPHandler,
		versionapi.New,
		NewSMSCore, NewSmsAPI,
		NewWebHookAPI,
		NewUniqueID,
		NewPushCore, NewPushAPI,
		gbs.NewServer,
		NewIPCStore, NewProtocols, NewIPCCore, NewIPCAPI, NewGBAdapter,
		NewProxyAPI, NewProxyCore,
		NewConfigAPI,
		NewUserAPI,
	)
)

type Usecase struct {
	Conf       *conf.Bootstrap
	DB         *gorm.DB
	Version    versionapi.API
	SMSAPI     SmsAPI
	WebHookAPI WebHookAPI
	UniqueID   uniqueid.Core
	MediaAPI   PushAPI
	GB28181API IPCAPI
	ProxyAPI   ProxyAPI
	ConfigAPI  ConfigAPI

	SipServer *gbs.Server
	UserAPI   UserAPI
}

// NewHTTPHandler 生成Gin框架路由内容
func NewHTTPHandler(uc *Usecase) http.Handler {
	cfg := uc.Conf.Server
	// 检查是否设置了 JWT 密钥，如果未设置，则生成一个长度为 32 的随机字符串作为密钥
	if cfg.HTTP.JwtSecret == "" {
		uc.Conf.Server.HTTP.JwtSecret = orm.GenerateRandomString(32) // 生成一个长度为 32 的随机字符串作为密钥
	}
	// 如果不处于调试模式，将 Gin 设置为发布模式
	if !cfg.Debug {
		gin.SetMode(gin.ReleaseMode) // 将 Gin 设置为发布模式
	}
	g := gin.New() // 创建一个新的 Gin 实例
	// 处理未找到路由的情况，返回 JSON 格式的 404 错误信息
	g.NoRoute(func(c *gin.Context) {
		c.JSON(404, "来到了无人的荒漠") // 返回 JSON 格式的 404 错误信息
	})
	// 如果启用了 Pprof，设置 Pprof 监控
	if cfg.HTTP.PProf.Enabled {
		web.SetupPProf(g, &cfg.HTTP.PProf.AccessIps) // 设置 Pprof 监控
	}

	setupRouter(g, uc) // 设置路由处理函数
	uc.Version.RecordVersion()
	return g // 返回配置好的 Gin 实例作为 http.Handler
}

// NewUniqueID 唯一 id 生成器
func NewUniqueID(db *gorm.DB) uniqueid.Core {
	return uniqueid.NewCore(uniqueiddb.NewDB(db).AutoMigrate(orm.GetEnabledAutoMigrate()), 5)
}

func NewPushCore(db *gorm.DB, uni uniqueid.Core) push.Core {
	return push.NewCore(pushdb.NewDB(db).AutoMigrate(orm.GetEnabledAutoMigrate()), uni)
}

func NewIPCStore(db *gorm.DB) ipc.Storer {
	return ipccache.NewCache(ipcdb.NewDB(db).AutoMigrate(orm.GetEnabledAutoMigrate()))
}

func NewGBAdapter(store ipc.Storer, uni uniqueid.Core) ipc.Adapter {
	return ipc.NewAdapter(
		store,
		uni,
	)
}

// NewProtocols 创建协议适配器映射
func NewProtocols(adapter ipc.Adapter, sms sms.Core, proxyCore *proxy.Core, gbs *gbs.Server) map[string]ipc.Protocoler {
	protocols := make(map[string]ipc.Protocoler)
	protocols[ipc.TypeOnvif] = onvifadapter.NewAdapter(adapter, sms)
	protocols[ipc.TypeRTSP] = rtspadapter.NewAdapter(proxyCore, sms)
	protocols[ipc.TypeGB28181] = gbadapter.NewAdapter(adapter, gbs, sms)
	return protocols
}
