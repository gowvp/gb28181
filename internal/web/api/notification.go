package api

import (
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/ixugo/goddd/pkg/web"
)

// NotificationType 通知类型
type NotificationType string

const (
	// NotifyDeviceOnline 设备上线
	NotifyDeviceOnline NotificationType = "device_online"
	// NotifyDeviceOffline 设备离线
	NotifyDeviceOffline NotificationType = "device_offline"
	// NotifyStreamStart 流开始
	NotifyStreamStart NotificationType = "stream_start"
	// NotifyStreamStop 流停止
	NotifyStreamStop NotificationType = "stream_stop"
	// NotifyRecordStart 录像开始
	NotifyRecordStart NotificationType = "record_start"
	// NotifyRecordStop 录像停止
	NotifyRecordStop NotificationType = "record_stop"
	// NotifyTypeError 错误通知
	NotifyTypeError NotificationType = "error"
)

// Notification 通知消息
type Notification struct {
	ID        string           `json:"id"`
	Type      NotificationType `json:"type"`
	Message   string           `json:"message"`
	Data      any              `json:"data,omitempty"`
	Timestamp int64            `json:"timestamp"`
}

// NotificationHub 通知中心，管理所有 SSE 连接
type NotificationHub struct {
	mu      sync.RWMutex
	clients map[string]*web.SSE
}

// NewNotificationHub 创建通知中心
func NewNotificationHub() *NotificationHub {
	return &NotificationHub{
		clients: make(map[string]*web.SSE),
	}
}

// globalNotificationHub 全局通知中心实例
var globalNotificationHub = NewNotificationHub()

// GetNotificationHub 获取全局通知中心
func GetNotificationHub() *NotificationHub {
	return globalNotificationHub
}

// AddClient 添加客户端
func (h *NotificationHub) AddClient(id string, sse *web.SSE) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.clients[id] = sse
}

// RemoveClient 移除客户端
func (h *NotificationHub) RemoveClient(id string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.clients, id)
}

// Broadcast 广播通知到所有客户端
func (h *NotificationHub) Broadcast(notification Notification) {
	h.mu.RLock()
	notification.ID = uuid.NewString()
	notification.Timestamp = time.Now().Unix()

	data, err := json.Marshal(notification)
	if err != nil {
		h.mu.RUnlock()
		slog.Error("marshal notification failed", "err", err)
		return
	}

	var nilClients []string
	for id, client := range h.clients {
		if client == nil {
			nilClients = append(nilClients, id)
			continue
		}
		client.Publish(web.Event{
			ID:    notification.ID,
			Event: string(notification.Type),
			Data:  data,
		})
		slog.Debug("notification sent", "client_id", id, "type", notification.Type)
	}
	h.mu.RUnlock()

	// 清理 nil 客户端
	if len(nilClients) > 0 {
		h.mu.Lock()
		for _, id := range nilClients {
			delete(h.clients, id)
		}
		h.mu.Unlock()
	}
}

// NotifyDeviceStatus 通知设备状态变化
func NotifyDeviceStatus(deviceID, deviceName string, online bool) {
	hub := GetNotificationHub()
	notifyType := NotifyDeviceOffline
	message := "设备离线"
	if online {
		notifyType = NotifyDeviceOnline
		message = "设备上线"
	}

	hub.Broadcast(Notification{
		Type:    notifyType,
		Message: message,
		Data: map[string]any{
			"device_id":   deviceID,
			"device_name": deviceName,
			"online":      online,
		},
	})
}

// NotifyStreamStatus 通知流状态变化
func NotifyStreamStatus(app, stream string, started bool) {
	hub := GetNotificationHub()
	notifyType := NotifyStreamStop
	message := "流已停止"
	if started {
		notifyType = NotifyStreamStart
		message = "流已开始"
	}

	hub.Broadcast(Notification{
		Type:    notifyType,
		Message: message,
		Data: map[string]any{
			"app":     app,
			"stream":  stream,
			"started": started,
		},
	})
}

// NotifyRecordStatus 通知录像状态变化
func NotifyRecordStatus(app, stream string, started bool) {
	hub := GetNotificationHub()
	notifyType := NotifyRecordStop
	message := "录像已停止"
	if started {
		notifyType = NotifyRecordStart
		message = "录像已开始"
	}

	hub.Broadcast(Notification{
		Type:    notifyType,
		Message: message,
		Data: map[string]any{
			"app":     app,
			"stream":  stream,
			"started": started,
		},
	})
}

// NotifyError 通知错误
func NotifyError(message string, data any) {
	hub := GetNotificationHub()
	hub.Broadcast(Notification{
		Type:    NotifyTypeError,
		Message: message,
		Data:    data,
	})
}

// registerNotificationAPI 注册通知 API
func registerNotificationAPI(g gin.IRouter, handler ...gin.HandlerFunc) {
	group := g.Group("/notifications", handler...)
	group.GET("/subscribe", subscribeNotifications)
}

// subscribeNotifications 订阅实时通知 (SSE)
func subscribeNotifications(c *gin.Context) {
	clientID := uuid.NewString()
	se := web.NewSSE(64, 30*time.Minute)

	hub := GetNotificationHub()
	hub.AddClient(clientID, se)

	// 发送连接成功通知
	se.Publish(web.Event{
		ID:    uuid.NewString(),
		Event: "connected",
		Data:  []byte(`{"message":"已连接到通知服务"}`),
	})

	slog.Info("client subscribed to notifications", "client_id", clientID)

	// 使用 defer 确保在连接关闭时移除客户端
	defer func() {
		hub.RemoveClient(clientID)
		se.Close()
		slog.Info("client unsubscribed from notifications", "client_id", clientID)
	}()

	se.ServeHTTP(c.Writer, c.Request)
}
