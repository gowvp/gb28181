package push

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"log/slog"
	"math"
	"math/rand/v2"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/gowvp/owl/internal/core/event"
	"github.com/ixugo/goddd/pkg/orm"
	"github.com/ixugo/goddd/pkg/system"
)

const (
	defaultMaxRetry   = 3
	defaultBufferSize = 64
	baseDelay         = time.Second
	maxDelay          = 10 * time.Second
)

// Dispatcher 告警事件分发接口，供 event.Core 依赖注入使用
type Dispatcher interface {
	Dispatch(ctx context.Context, ev *event.Event)
}

// Notifier 将告警事件异步推送到多个 webhook 目标
// 每个目标独立维护一个 buffered channel 和一个 worker goroutine，互不阻塞
type Notifier struct {
	workers []*worker
}

type worker struct {
	url      string // 已剥离 secret query 参数的干净 URL
	secret   string // 从原始 URL ?secret= 中提取，发请求时放入 Secret header
	ch       chan *event.Event
	client   *http.Client
	maxRetry int
}

// NewNotifier 创建 Notifier，为每个 target 启动独立的 worker goroutine
// target URL 中的 ?secret=xxx 会被提取放入 Secret header 发送，URL 本身不携带 secret
func NewNotifier(targets []string, maxRetry, bufferSize int) *Notifier {
	if maxRetry <= 0 {
		maxRetry = defaultMaxRetry
	}
	if bufferSize <= 0 {
		bufferSize = defaultBufferSize
	}

	client := http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			MaxIdleConnsPerHost: 50,
		},
	}

	n := &Notifier{}
	for _, rawURL := range targets {
		cleanURL, secret := extractSecret(rawURL)
		w := &worker{
			url:      cleanURL,
			secret:   secret,
			ch:       make(chan *event.Event, bufferSize),
			client:   &client,
			maxRetry: maxRetry,
		}
		n.workers = append(n.workers, w)
		go w.run()
	}
	return n
}

// extractSecret 从 URL query 中取出 secret 参数，返回去掉 secret 的干净 URL 和 secret 值
// secret 不落日志、不出现在请求 URL 中，仅通过 Secret header 传递
func extractSecret(rawURL string) (cleanURL, secret string) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL, ""
	}
	q := u.Query()
	secret = q.Get("secret")
	q.Del("secret")
	u.RawQuery = q.Encode()
	return u.String(), secret
}

// Dispatch 非阻塞地将事件投递到每个 target 的缓冲队列
// 若队列已满则丢弃并记录警告，不阻塞调用方
func (n *Notifier) Dispatch(ctx context.Context, ev *event.Event) {
	for _, w := range n.workers {
		select {
		case w.ch <- ev:
		default:
			slog.WarnContext(ctx, "webhook buffer full, event dropped", "url", w.url)
		}
	}
}

// Close 关闭所有 worker channel，等待剩余事件处理后退出
func (n *Notifier) Close() {
	for _, w := range n.workers {
		close(w.ch)
	}
}

func (w *worker) run() {
	for ev := range w.ch {
		w.sendWithRetry(ev)
	}
}

// webhookPayload 推送给外部 gowvp 实例的事件 payload
// ImageBase64 携带图片 base64 编码内容，接收方解码后落盘，避免文件路径跨实例失效
type webhookPayload struct {
	DID         string   `json:"did"`
	CID         string   `json:"cid"`
	StartedAt   orm.Time `json:"started_at"`
	EndedAt     orm.Time `json:"ended_at"`
	Label       string   `json:"label"`
	Score       float32  `json:"score"`
	Zones       string   `json:"zones"`
	Model       string   `json:"model"`
	ImageBase64 string   `json:"image_base64,omitempty"` // base64 编码图片，为空则无图片
}

// buildPayload 从 Event 构建推送 payload，若有图片则读取并 base64 编码
func buildPayload(ev *event.Event) *webhookPayload {
	p := &webhookPayload{
		DID:       ev.DID,
		CID:       ev.CID,
		StartedAt: ev.StartedAt,
		EndedAt:   ev.EndedAt,
		Label:     ev.Label,
		Score:     ev.Score,
		Zones:     ev.Zones,
		Model:     ev.Model,
	}
	if ev.ImagePath != "" {
		fullPath := filepath.Join(system.Getwd(), "configs", "events", ev.ImagePath)
		data, err := os.ReadFile(fullPath)
		if err != nil {
			slog.Warn("webhook: read image failed, sending without image", "path", fullPath, "err", err)
		} else {
			p.ImageBase64 = base64.StdEncoding.EncodeToString(data)
		}
	}
	return p
}

// sendWithRetry 使用指数退避重试推送事件
// 延迟公式: min(baseDelay * 2^attempt, maxDelay) * (0.75 + rand*0.5)  加 ±25% jitter
func (w *worker) sendWithRetry(ev *event.Event) {
	body, err := json.Marshal(buildPayload(ev))
	if err != nil {
		slog.Error("webhook: marshal event failed", "url", w.url, "err", err)
		return
	}

	for attempt := 0; attempt <= w.maxRetry; attempt++ {
		if attempt > 0 {
			delay := calcDelay(attempt - 1)
			time.Sleep(delay)
		}

		statusCode, err := w.doPost(body)
		if err == nil && statusCode >= 200 && statusCode < 300 {
			return
		}

		// 4xx（除 429、408）不重试，属于永久性错误
		if err == nil && statusCode >= 400 && statusCode < 500 &&
			statusCode != http.StatusTooManyRequests && statusCode != http.StatusRequestTimeout {
			slog.Error("webhook push permanent failure", "url", w.url, "status_code", statusCode)
			return
		}

		slog.Warn("webhook push retry",
			"url", w.url,
			"attempt", attempt+1,
			"max_retry", w.maxRetry,
			"status_code", statusCode,
			"err", err,
		)
	}

	slog.Error("webhook push exhausted retries", "url", w.url, "max_retry", w.maxRetry)
}

func (w *worker) doPost(body []byte) (int, error) {
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, w.url, bytes.NewReader(body))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	if w.secret != "" {
		req.Header.Set("Secret", w.secret)
	}

	resp, err := w.client.Do(req)
	if err != nil {
		return 0, err
	}
	_ = resp.Body.Close()
	return resp.StatusCode, nil
}

// calcDelay 指数退避 + ±25% jitter，上限 maxDelay
func calcDelay(attempt int) time.Duration {
	exp := math.Pow(2, float64(attempt))
	d := min(time.Duration(float64(baseDelay)*exp), maxDelay)
	// jitter: [0.75, 1.25)
	jitter := 0.75 + rand.Float64()*0.5
	return time.Duration(float64(d) * jitter)
}
