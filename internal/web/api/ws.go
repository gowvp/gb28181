package api

import (
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/gin-gonic/gin"
	"github.com/gowvp/owl/internal/notify"
	"github.com/ixugo/goddd/pkg/web"
	"github.com/ixugo/goddd/pkg/ws"
)

// wsHub 全局 WebSocket Hub 实例，供各模块广播告警
var wsHub ws.Huber

// broadcastWarn 向所有已认证的 WebSocket 客户端广播一条通用警告通知
func broadcastWarn(msg string) {
	if wsHub == nil {
		return
	}
	wsHub.Broadcast(ws.NewMessage("warn", map[string]string{"msg": msg}))
}

// broadcastIPCWarn 向所有已认证的 WebSocket 客户端广播一条设备相关警告通知
func broadcastIPCWarn(msg, deviceID, name string) {
	if wsHub == nil {
		return
	}
	wsHub.Broadcast(ws.NewMessage("ipc_warn", map[string]string{"msg": msg, "device_id": deviceID, "name": name}))
}

// broadcastIPCInfo 向所有已认证的 WebSocket 客户端广播一条设备相关信息通知
func broadcastIPCInfo(msg, deviceID, name string) {
	if wsHub == nil {
		return
	}
	wsHub.Broadcast(ws.NewMessage("ipc_info", map[string]string{"msg": msg, "device_id": deviceID, "name": name}))
}

// registerWS 注册 WebSocket 路由并初始化 Hub
func registerWS(r *gin.Engine, jwtSecret string) {
	hub := ws.NewHub(func(c *ws.Config) {
		c.MaxConnections = 128
	})

	hub.SetAuthHandler(func(message ws.Message) (string, error) {
		var data struct {
			Token string `json:"token"`
		}
		if err := json.Unmarshal(message.Data(), &data); err != nil {
			return "", fmt.Errorf("invalid auth message")
		}
		if data.Token == "" {
			return "", fmt.Errorf("token is empty")
		}
		claims, err := web.ParseToken(data.Token, jwtSecret)
		if err != nil {
			return "", fmt.Errorf("invalid token")
		}
		if err := claims.Valid(); err != nil {
			return "", fmt.Errorf("token expired")
		}
		userID, _ := claims.Data["user_id"].(string)
		return userID, nil
	})

	hub.SetConnectHandler(func(client *ws.Client) error {
		return client.Send(client.Request().Context(), ws.NewMessage("auth", nil))
	})

	hub.SetErrorHandler(func(client *ws.Client, err error) {
		slog.Debug("ws client error", "client_id", client.ID(), "err", err)
	})

	r.GET("/ws", func(c *gin.Context) {
		hub.ServeHTTP(c.Writer, c.Request)
	})

	wsHub = hub
	notify.SetWarnFunc(broadcastWarn)
	notify.SetIPCWarnFunc(broadcastIPCWarn)
	notify.SetIPCInfoFunc(broadcastIPCInfo)
}
