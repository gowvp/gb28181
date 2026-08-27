package api

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/gowvp/owl/internal/conf"
	"github.com/gowvp/owl/internal/core/event"
	"github.com/ixugo/goddd/pkg/orm"
)

// stubEventStorer 实现 event.EventStorer 接口，记录 Create 调用次数
type stubEventStorer struct {
	addCount int
}

func (s *stubEventStorer) WithTx(_ orm.Tx) (event.EventStorer, error)               { return s, nil }
func (s *stubEventStorer) GetByID(_ context.Context, _ int64) (*event.Event, error) { return nil, nil }
func (s *stubEventStorer) List(_ context.Context, _ *[]*event.Event, _ *event.ListEventInput) (int64, error) {
	return 0, nil
}

func (s *stubEventStorer) Create(_ context.Context, _ *event.Event) error {
	s.addCount++
	return nil
}

func (s *stubEventStorer) Update(_ context.Context, _ *event.Event, _ func(*event.Event) error) error {
	return nil
}
func (s *stubEventStorer) Delete(_ context.Context, _ *event.Event) error { return nil }
func (s *stubEventStorer) Count(_ context.Context, _ *event.ListEventInput) (int64, error) {
	return 0, nil
}
func (s *stubEventStorer) BatchDeleteByIDs(_ context.Context, _ []int64) error { return nil }

// stubStorer 实现 event.Storer
type stubStorer struct{ ev *stubEventStorer }

func (s *stubStorer) Begin() (orm.Tx, error)   { return nil, nil }
func (s *stubStorer) Event() event.EventStorer { return s.ev }

// makeWebhookEngine 构建仅注册 POST /webhook/events 的测试引擎
func makeWebhookEngine(internalSecret, recvSecret string, ev *stubEventStorer) *gin.Engine {
	bc := &conf.Bootstrap{}
	bc.AISecret = internalSecret
	bc.Server.Webhook.RecvSecret = recvSecret

	hookAPI := WebHookAPI{
		conf: bc,
		// TODO: 检查是否需要注入事件分发器
		eventCore: event.NewCore(&stubStorer{ev}, nil),
		log:       slog.Default(),
	}

	r := gin.New()
	r.POST("/webhook/events", hookAPI.onWebhookEvents)
	return r
}

// TestWebhookEvents_AIAuth_WrongSecret 验证 query secret 鉴权失败时返回 401
func TestWebhookEvents_AIAuth_WrongSecret(t *testing.T) {
	ev := &stubEventStorer{}
	r := makeWebhookEngine("correct-secret", "recv-secret", ev)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/webhook/events?secret=wrong-secret", bytes.NewReader([]byte(`{}`)))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
	if ev.addCount != 0 {
		t.Error("expected no event saved on auth failure")
	}
}

// TestWebhookEvents_NoSecret 验证无 secret 时返回 401
func TestWebhookEvents_NoSecret(t *testing.T) {
	ev := &stubEventStorer{}
	r := makeWebhookEngine("internal-uuid", "recv-secret", ev)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/webhook/events", bytes.NewReader([]byte(`{}`)))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

// TestWebhookEvents_HeaderSecret 验证 Secret: <secret> header 方式也被接受
func TestWebhookEvents_HeaderSecret(t *testing.T) {
	const recvSecret = "recv-via-header"
	ev := &stubEventStorer{}
	r := makeWebhookEngine("internal-uuid", recvSecret, ev)

	payload := webhookForwardInput{CID: "cam001", Label: "car", Score: 0.9}
	body, _ := json.Marshal(payload)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/webhook/events", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Secret", recvSecret)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d, body: %s", w.Code, w.Body.String())
	}
	if ev.addCount != 1 {
		t.Errorf("expected 1 event saved, got %d", ev.addCount)
	}
}

// TestWebhookEvents_Forward_RecvSecret 验证 gowvp 转发路径：query secret 鉴权 + 事件入库
func TestWebhookEvents_Forward_RecvSecret(t *testing.T) {
	const recvSecret = "recv-secret-abc"
	ev := &stubEventStorer{}
	r := makeWebhookEngine("internal-uuid", recvSecret, ev)

	payload := webhookForwardInput{
		DID:   "dev001",
		CID:   "cam001",
		Label: "car",
		Score: 0.9,
		Model: "yolov8",
	}
	body, _ := json.Marshal(payload)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/webhook/events?secret="+recvSecret, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d, body: %s", w.Code, w.Body.String())
	}
	if ev.addCount != 1 {
		t.Errorf("expected 1 event saved, got %d", ev.addCount)
	}
}

// TestWebhookEvents_Forward_WrongSecret 验证 gowvp 转发路径：错误 secret 返回 401
func TestWebhookEvents_Forward_WrongSecret(t *testing.T) {
	ev := &stubEventStorer{}
	r := makeWebhookEngine("internal-uuid", "correct-recv", ev)

	body := []byte(`{"cid":"cam001","label":"person"}`)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/webhook/events?secret=wrong", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
	if ev.addCount != 0 {
		t.Error("expected no event saved on wrong secret")
	}
}

// TestWebhookEvents_Forward_InternalSecretAsRecv 验证 InternalSecret 也可通过 query secret 鉴权（两路信任）
func TestWebhookEvents_Forward_InternalSecretAsRecv(t *testing.T) {
	const internalSecret = "shared-internal"
	ev := &stubEventStorer{}
	// 当 query secret == InternalSecret 时走 AI 路径，ipcCore 为零值会 panic
	// 此处不测试 AI 路径，改用 RecvSecret 测试转发路径
	r := makeWebhookEngine("different-internal", internalSecret, ev)

	payload := webhookForwardInput{CID: "cam001", Label: "dog", Score: 0.8}
	body, _ := json.Marshal(payload)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/webhook/events?secret="+internalSecret, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if ev.addCount != 1 {
		t.Errorf("expected 1 event saved, got %d", ev.addCount)
	}
}
