package api

import (
	"github.com/gin-gonic/gin"
	"github.com/gowvp/owl/internal/core/proxy"
	"github.com/gowvp/owl/internal/core/proxy/stores/proxydb"
	"github.com/ixugo/goddd/domain/uniqueid"
	"github.com/ixugo/goddd/pkg/orm"
	"github.com/ixugo/goddd/pkg/web"
	"gorm.io/gorm"
)

type ProxyAPI struct {
	proxyCore *proxy.Core
}

func NewProxyCore(db *gorm.DB, uni uniqueid.Core) *proxy.Core {
	return proxy.NewCore(proxydb.NewDB(db).AutoMigrate(orm.GetEnabledAutoMigrate()), uni)
}

func NewProxyAPI(proxyCore *proxy.Core) ProxyAPI {
	return ProxyAPI{proxyCore: proxyCore}
}

// streamProxyIDInput 拉流代理 ID 路径参数
type streamProxyIDInput = proxy.GetStreamProxyInput

// updateStreamProxyInput 更新拉流代理的请求参数（路径 ID + 请求体）
type updateStreamProxyInput = proxy.UpdateStreamProxyInput

func registerProxy(g gin.IRouter, api ProxyAPI, handler ...gin.HandlerFunc) {
	group := g.Group("/stream_proxys", handler...)
	group.GET("", web.WrapH(api.listStreamProxys))
	group.GET("/:id", web.WrapH(api.getStreamProxy))
	group.PUT("/:id", web.WrapH(api.updateStreamProxy))
	group.POST("", web.WrapH(api.createStreamProxy))
	group.DELETE("/:id", web.WrapH(api.deleteStreamProxy))
}

// listStreamProxys 分页查询拉流代理列表
func (a ProxyAPI) listStreamProxys(c *gin.Context, in *proxy.ListStreamProxyInput) (any, error) {
	items, total, err := a.proxyCore.ListStreamProxys(c.Request.Context(), in)
	return gin.H{"items": items, "total": total}, err
}

// getStreamProxy 按 ID 查询拉流代理
func (a ProxyAPI) getStreamProxy(c *gin.Context, in *streamProxyIDInput) (any, error) {
	return a.proxyCore.GetStreamProxy(c.Request.Context(), in.ID)
}

// updateStreamProxy 更新拉流代理
func (a ProxyAPI) updateStreamProxy(c *gin.Context, in *updateStreamProxyInput) (any, error) {
	return a.proxyCore.UpdateStreamProxy(c.Request.Context(), in)
}

// createStreamProxy 创建拉流代理
func (a ProxyAPI) createStreamProxy(c *gin.Context, in *proxy.CreateStreamProxyInput) (any, error) {
	return a.proxyCore.CreateStreamProxy(c.Request.Context(), in)
}

// deleteStreamProxy 删除拉流代理
func (a ProxyAPI) deleteStreamProxy(c *gin.Context, in *streamProxyIDInput) (any, error) {
	return a.proxyCore.DeleteStreamProxy(c.Request.Context(), in.ID)
}
