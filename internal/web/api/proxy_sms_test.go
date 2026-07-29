package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gowvp/owl/internal/conf"
	"github.com/ixugo/goddd/pkg/web"
)

const testPlaySecret = "unit-test-secret"

// makePlayToken 生成播放 token 用于测试
func makePlayToken(t *testing.T, app, stream string, expiresAt time.Time) string {
	t.Helper()
	secret := testPlaySecret + "_play"
	token, err := web.NewToken(
		map[string]any{"stream": stream, "app": app},
		secret,
		web.WithExpiresAt(expiresAt),
	)
	if err != nil {
		t.Fatalf("生成播放 token 失败: %v", err)
	}
	return token
}

// newTestUsecase 创建带最小配置的 Usecase 用于测试 verifyPlayToken
func newTestUsecase() *Usecase {
	return &Usecase{
		Conf: &conf.Bootstrap{
			Server: conf.Server{
				HTTP: conf.ServerHTTP{
					JwtSecret: testPlaySecret,
				},
			},
		},
	}
}

func TestVerifyPlayToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	uc := newTestUsecase()

	tests := []struct {
		name       string
		path       string
		query      string
		wantStatus int
	}{
		{
			name:       "正常 http_flv 请求",
			path:       "/rtp/34020000001320000001.live.flv",
			query:      "token=" + makePlayToken(t, "rtp", "34020000001320000001", time.Now().Add(42*time.Hour)),
			wantStatus: http.StatusOK,
		},
		{
			name:       "正常 hls 请求",
			path:       "/live/camera01/hls.fmp4.m3u8",
			query:      "token=" + makePlayToken(t, "live", "camera01", time.Now().Add(42*time.Hour)),
			wantStatus: http.StatusOK,
		},
		{
			name:       "webrtc 请求（stream 在 query 中）",
			path:       "/index/api/webrtc",
			query:      "app=rtp&stream=34020000001320000001&type=play&token=" + makePlayToken(t, "rtp", "34020000001320000001", time.Now().Add(42*time.Hour)),
			wantStatus: http.StatusOK,
		},
		{
			name:       "缺少 token",
			path:       "/rtp/34020000001320000001.live.flv",
			query:      "",
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "token 已过期",
			path:       "/rtp/34020000001320000001.live.flv",
			query:      "token=" + makePlayToken(t, "rtp", "34020000001320000001", time.Now().Add(-1*time.Hour)),
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "token 中 stream 与路径不匹配",
			path:       "/rtp/other_stream.live.flv",
			query:      "token=" + makePlayToken(t, "rtp", "34020000001320000001", time.Now().Add(42*time.Hour)),
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "错误密钥签发的 token",
			path:       "/rtp/34020000001320000001.live.flv",
			query:      "token=" + makeTokenWithWrongSecret(t),
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "HLS 分片 init.mp4 无 token 豁免",
			path:       "/rtp/34020000001320000001/init.mp4",
			query:      "",
			wantStatus: http.StatusOK,
		},
		{
			name:       "HLS 分片 ts 无 token 豁免",
			path:       "/rtp/34020000001320000001/2026-07-29/22/56-45_21.mp4",
			query:      "",
			wantStatus: http.StatusOK,
		},
		{
			name:       "HLS 分片 m4s 无 token 豁免",
			path:       "/rtp/34020000001320000001/seg-1.m4s",
			query:      "",
			wantStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := gin.New()
			r.GET("/proxy/sms/*path", func(c *gin.Context) {
				path := c.Param("path")
				if err := uc.verifyPlayToken(c, path); err != nil {
					c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"code": 403, "msg": err.Error()})
					return
				}
				c.String(http.StatusOK, "pass")
			})

			w := httptest.NewRecorder()
			reqURL := "/proxy/sms" + tt.path
			if tt.query != "" {
				reqURL += "?" + tt.query
			}
			req := httptest.NewRequest(http.MethodGet, reqURL, nil)
			r.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("want status %d, got %d, body: %s", tt.wantStatus, w.Code, w.Body.String())
			}
		})
	}
}

// makeTokenWithWrongSecret 用错误密钥生成 token
func makeTokenWithWrongSecret(t *testing.T) string {
	t.Helper()
	token, err := web.NewToken(
		map[string]any{"stream": "34020000001320000001", "app": "rtp"},
		"wrong-secret_play",
	)
	if err != nil {
		t.Fatalf("生成错误密钥 token 失败: %v", err)
	}
	return token
}
