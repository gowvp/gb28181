package api

import (
	"bytes"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/ixugo/goddd/pkg/reason"
	"github.com/ixugo/goddd/pkg/web"
)

// AuthMiddleware 鉴权
// handler 可以拦截请求，返回 true 则跳过默认鉴权行为，可以通过此参数自定义鉴权方案
// 开启第三方鉴权时，两者任何一个通过，则鉴权通过!
// 优先尝试 JWT（开销小），失败后再尝试 authURL（开销大）
func AuthMiddleware(secret string, authURL string, handler ...web.HandlerOption) gin.HandlerFunc {
	client := http.Client{Timeout: 10 * time.Second}
	return func(c *gin.Context) {
		for _, h := range handler {
			if h(c) {
				c.Next()
				return
			}
		}

		auth := c.Request.Header.Get("Authorization")
		// header 中没有时，尝试从 query 参数中取
		if auth == "" {
			auth = c.Query("token")
		}

		const prefix = "Bearer "
		if len(auth) > len(prefix) && strings.EqualFold(auth[:len(prefix)], prefix) {
			tokenStr := auth[len(prefix):]
			claims, err := web.ParseToken(tokenStr, secret)
			if err != nil && errors.Is(err, jwt.ErrSignatureInvalid) {
				web.AbortWithStatusJSON(c, reason.ErrUnauthorizedToken.WithMsg("无效的 token"))
				return
			}
			if err == nil {
				// JWT 鉴权通过
				c.Set(web.KeyTokenString, auth)
				for k, v := range claims.Data {
					c.Set(k, v)
				}
				c.Next()
				return
			}
			// JWT 解析失败，继续尝试 authURL
		}

		// 无有效 token，检查是否配置了 authURL
		if authURL == "" {
			web.AbortWithStatusJSON(c, reason.ErrUnauthorizedToken.WithMsg("身份验证失败"))
			return
		}

		// 尝试第三方鉴权，失败时响应已直接写给客户端
		code := forwardToAuthURL(&client, c, authURL)
		if code == http.StatusOK {
			c.Set(web.KeyTokenString, auth)
			c.Next()
			return
		}
		c.Abort()
	}
}

// forwardToAuthURL 将原始请求透传给第三方鉴权服务
// 为什么: 需要将客户端的全部请求信息（header、body）原样转发，让第三方服务自行判断鉴权
// 非 200 时直接流式写回客户端，避免缓冲响应体；返回状态码供调用方判断
func forwardToAuthURL(client *http.Client, c *gin.Context, authURL string) int {
	// 读取原始请求 body，用于透传
	var bodyBytes []byte
	if c.Request.Body != nil {
		bodyBytes, _ = io.ReadAll(c.Request.Body)
		// 恢复 body，让后续 handler 仍能读取
		c.Request.Body = io.NopCloser(bytes.NewReader(bodyBytes))
	}

	req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodPost, authURL, bytes.NewReader(bodyBytes))
	if err != nil {
		slog.ErrorContext(c.Request.Context(), "构建 authURL 请求失败", "err", err, "authURL", authURL)
		web.Fail(c, reason.ErrServer.WithHTTPStatus(http.StatusInternalServerError).WithMsg("鉴权服务请求失败"))
		return http.StatusInternalServerError
	}

	// 透传原始请求的所有 header
	for key, values := range c.Request.Header {
		for _, v := range values {
			req.Header.Add(key, v)
		}
	}

	resp, err := client.Do(req)
	if err != nil {
		slog.ErrorContext(c.Request.Context(), "请求 authURL 失败", "err", err, "authURL", authURL)
		web.Fail(c, reason.ErrServiceUnavailable.WithHTTPStatus(http.StatusBadGateway).WithMsg("鉴权服务不可达: "+err.Error()))
		return http.StatusBadGateway
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		return http.StatusOK
	}

	// 非 200，流式写回客户端，不缓冲响应体
	c.Status(resp.StatusCode)
	c.Header("Content-Type", resp.Header.Get("Content-Type"))
	io.Copy(c.Writer, resp.Body)
	return resp.StatusCode
}
