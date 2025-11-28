package api

import (
	"expvar"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"sort"
	"strings"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
	"github.com/gowvp/gb28181/plugin/stat"
	"github.com/gowvp/gb28181/plugin/stat/statapi"
	"github.com/ixugo/goddd/domain/version/versionapi"
	"github.com/ixugo/goddd/pkg/system"
	"github.com/ixugo/goddd/pkg/web"
)

var startRuntime = time.Now()

func setupRouter(r *gin.Engine, uc *Usecase) {
	uc.GB28181API.uc = uc
	uc.SMSAPI.uc = uc
	uc.WebHookAPI.uc = uc
	const staticPrefix = "/web"

	go stat.LoadTop(system.Getwd(), func(m map[string]any) {
		_ = m
	})
	r.Use(
		// 格式化输出到控制台，然后记录到日志
		// 此处不做 recover，底层 http.server 也会 recover，但不会输出方便查看的格式
		gin.CustomRecovery(func(c *gin.Context, err any) {
			slog.ErrorContext(c.Request.Context(), "panic", "err", err, "stack", string(debug.Stack()))
			c.AbortWithStatus(http.StatusInternalServerError)
		}),
		web.Metrics(),
		web.Logger(web.IgnorePrefix(staticPrefix),
			web.IgnoreMethod(http.MethodOptions),
		),
		web.LoggerWithBody(web.DefaultBodyLimit,
			web.IgnoreBool(uc.Conf.Debug),
			web.IgnoreMethod(http.MethodOptions),
		),
	)
	go web.CountGoroutines(10*time.Minute, 20)

	r.Use(cors.New(cors.Config{
		AllowMethods: []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"},
		AllowHeaders: []string{
			"Accept", "Content-Length", "Content-Type", "Range", "Accept-Language",
			"Origin", "Authorization",
			"Accept-Encoding",
			"Cache-Control", "Pragma", "X-Requested-With",
			"Sec-Fetch-Mode", "Sec-Fetch-Site", "Sec-Fetch-Dest",
			"Dnt", "X-Forwarded-For", "X-Forwarded-Proto", "X-Forwarded-Host",
			"X-Real-IP", "X-Request-ID", "X-Request-Start", "X-Request-Time",
		},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
		AllowOriginFunc: func(_ string) bool {
			return true
		},
	}))

	const staticDir = "www"
	admin := r.Group(staticPrefix, gzip.Gzip(gzip.DefaultCompression))
	admin.Static("/", filepath.Join(system.Getwd(), staticDir))
	r.NoRoute(func(c *gin.Context) {
		// react-router 路由指向前端资源
		if strings.HasPrefix(c.Request.URL.Path, staticPrefix) {
			c.File(filepath.Join(system.Getwd(), staticDir, "index.html"))
			return
		}
		c.JSON(404, gin.H{"msg": "来到了无人的荒漠"})
	})
	// 访问根路径时重定向到前端资源
	r.GET("/", func(ctx *gin.Context) {
		ctx.Redirect(http.StatusPermanentRedirect, staticPrefix+"/"+"index.html")
	})

	auth := web.AuthMiddleware(uc.Conf.Server.HTTP.JwtSecret)
	r.GET("/health", web.WrapH(uc.getHealth))
	r.GET("/app/metrics/api", web.WrapH(uc.getMetricsAPI))

	versionapi.Register(r, uc.Version, auth)
	statapi.Register(r)
	registerZLMWebhookAPI(r, uc.WebHookAPI)

	registerPushAPI(r, uc.MediaAPI, auth)
	registerGB28181(r, uc.GB28181API, auth)
	registerProxy(r, uc.ProxyAPI, auth)
	registerConfig(r, uc.ConfigAPI, auth)
	registerSms(r, uc.SMSAPI, auth)
	RegisterUser(r, uc.UserAPI, auth)
	registerRecordAPI(r, uc.SMSAPI, auth)       // 录像 API
	registerNotificationAPI(r, auth)            // 实时通知 API
	registerPTZAPI(r, uc.GB28181API, auth)      // 云台控制 API
	registerPlaybackAPI(r, uc.GB28181API, auth) // 录像回放 API
	registerAlarmAPI(r, uc.GB28181API, auth)    // 报警事件 API

	// 反向代理流媒体数据
	r.Any("/proxy/sms/*path", uc.proxySMS)
}

type playOutput struct {
	App       string           `json:"app"`
	Stream    string           `json:"stream"`
	Items     []streamAddrItem `json:"items"`
	ExpiresAt int64            `json:"expires_at,omitempty"` // 播放链接过期时间戳(秒)
}
type streamAddrItem struct {
	Label   string `json:"label"`
	WSFLV   string `json:"ws_flv"`
	HTTPFLV string `json:"http_flv"`
	RTMP    string `json:"rtmp"`
	RTSP    string `json:"rtsp"`
	WebRTC  string `json:"webrtc"`
	HLS     string `json:"hls"`
}

type getHealthOutput struct {
	Version   string    `json:"version"`
	StartAt   time.Time `json:"start_at"`
	GitBranch string    `json:"git_branch"`
	GitHash   string    `json:"git_hash"`
}

func (uc *Usecase) getHealth(_ *gin.Context, _ *struct{}) (getHealthOutput, error) {
	return getHealthOutput{
		Version:   uc.Conf.BuildVersion,
		GitBranch: strings.Trim(expvar.Get("git_branch").String(), `"`),
		GitHash:   strings.Trim(expvar.Get("git_hash").String(), `"`),
		StartAt:   startRuntime,
	}, nil
}

type getMetricsAPIOutput struct {
	RealTimeRequests int64  `json:"real_time_requests"` // 实时请求数
	TotalRequests    int64  `json:"total_requests"`     // 总请求数
	TotalResponses   int64  `json:"total_responses"`    // 总响应数
	RequestTop10     []KV   `json:"request_top10"`      // 请求TOP10
	StatusCodeTop10  []KV   `json:"status_code_top10"`  // 状态码TOP10
	Goroutines       any    `json:"goroutines"`         // 协程数量
	NumGC            uint32 `json:"num_gc"`             // gc 次数
	SysAlloc         uint64 `json:"sys_alloc"`          // 内存占用
	StartAt          string `json:"start_at"`           // 运行时间
}

func (uc *Usecase) getMetricsAPI(_ *gin.Context, _ *struct{}) (*getMetricsAPIOutput, error) {
	req := expvar.Get("request").(*expvar.Int).Value()
	reqs := expvar.Get("requests").(*expvar.Int).Value()
	resps := expvar.Get("responses").(*expvar.Int).Value()
	urls := expvar.Get(`requestURLs`).(*expvar.Map)
	status := expvar.Get(`statusCodes`).(*expvar.Map)
	u := sortExpvarMap(urls, 10)
	s := sortExpvarMap(status, 10)
	g := expvar.Get("goroutine_num").(expvar.Func)

	var stats runtime.MemStats
	runtime.ReadMemStats(&stats)

	return &getMetricsAPIOutput{
		RealTimeRequests: req,
		TotalRequests:    reqs,
		TotalResponses:   resps,
		RequestTop10:     u,
		StatusCodeTop10:  s,
		Goroutines:       g(),
		NumGC:            stats.NumGC,
		SysAlloc:         stats.Sys,
		StartAt:          startRuntime.Format(time.DateTime),
	}, nil
}

type KV struct {
	Key   string
	Value int64
}

func sortExpvarMap(data *expvar.Map, top int) []KV {
	kvs := make([]KV, 0, 8)
	data.Do(func(kv expvar.KeyValue) {
		kvs = append(kvs, KV{
			Key:   kv.Key,
			Value: kv.Value.(*expvar.Int).Value(),
		})
	})

	sort.Slice(kvs, func(i, j int) bool {
		return kvs[i].Value > kvs[j].Value
	})

	idx := top
	if l := len(kvs); l < top {
		idx = len(kvs)
	}
	return kvs[:idx]
}

func (uc *Usecase) proxySMS(c *gin.Context) {
	defer func() {
		_ = recover()
	}()

	rc := http.NewResponseController(c.Writer)
	exp := time.Now().AddDate(99, 0, 0)
	_ = rc.SetReadDeadline(exp)
	_ = rc.SetWriteDeadline(exp)

	path := c.Param("path")
	addr, err := url.JoinPath(fmt.Sprintf("http://%s:%d", uc.Conf.Media.IP, uc.Conf.Media.HTTPPort), path)
	if err != nil {
		web.Fail(c, err)
		return
	}
	fullAddr, _ := url.Parse(addr)
	c.Request.URL.Path = ""
	proxy := httputil.NewSingleHostReverseProxy(fullAddr)
	proxy.Director = func(req *http.Request) {
		// 设置请求的URL
		req.URL.Scheme = "http"
		req.URL.Host = fmt.Sprintf("%s:%d", uc.Conf.Media.IP, uc.Conf.Media.HTTPPort)
		req.URL.Path = path
	}
	proxy.ModifyResponse = func(r *http.Response) error {
		r.Header.Del("Access-Control-Allow-Credentials")
		r.Header.Del("Access-Control-Allow-Origin")
		if r.StatusCode >= 300 && r.StatusCode < 400 {
			if l := r.Header.Get("Location"); l != "" {
				if !strings.HasPrefix(l, "http") {
					r.Header.Set("Location", "/proxy/sms/"+strings.TrimPrefix(l, "/"))
				}
			}
		}
		return nil
	}
	proxy.ServeHTTP(c.Writer, c.Request)
}
