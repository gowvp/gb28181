package api

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gowvp/owl/internal/conf"
	"github.com/gowvp/owl/internal/core/recording"
	"github.com/gowvp/owl/internal/core/recording/stores/recordingdb"
	"github.com/grafov/m3u8"
	"github.com/ixugo/goddd/pkg/orm"
	"github.com/ixugo/goddd/pkg/web"
	"gorm.io/gorm"
)

// RecordingAPI 为 http 提供业务方法
type RecordingAPI struct {
	recordingCore recording.Core
	conf          *conf.Bootstrap
}

// NewRecordingStore 创建录像存储层
func NewRecordingStore(db *gorm.DB) recording.Storer {
	return recordingdb.NewDB(db).AutoMigrate(orm.GetEnabledAutoMigrate())
}

// NewRecordingCore 创建录像管理核心服务
// 依赖 recording.SMSProvider 接口而非 sms.Core，避免循环依赖
func NewRecordingCore(
	store recording.Storer,
	cfg *conf.Bootstrap,
	provider recording.SMSProvider,
	ipcProvider recording.IPCProvider,
	playProvider recording.PlayProvider,
) recording.Core {
	core := recording.NewCore(store,
		recording.WithConfig(&cfg.Server.Recording),
		recording.WithSMSProvider(provider),
		recording.WithIPCProvider(ipcProvider),
		recording.WithPlayProvider(playProvider),
	)

	// 启动清理协程
	go core.StartCleanupWorker()

	// 启动录制同步协程（平台重启/流中断恢复）
	core.StartRecordingSyncLoop(context.Background())

	return core
}

func NewRecordingAPI(core recording.Core, conf *conf.Bootstrap) RecordingAPI {
	return RecordingAPI{recordingCore: core, conf: conf}
}

// recordingIDInput 录像 ID 路径参数
type recordingIDInput struct {
	ID int64 `uri:"id" binding:"required"`
}

// updateRecordingInput 更新录像的请求参数（路径 ID + 请求体）
type updateRecordingInput struct {
	ID int64 `uri:"id" binding:"required"`
	recording.EditRecordingInput
}

func RegisterRecording(g gin.IRouter, api RecordingAPI, handler ...gin.HandlerFunc) {
	{
		group := g.Group("/recordings", handler...)
		group.GET("", web.WrapH(api.listRecordings))
		group.GET("/timeline", web.WrapH(api.getTimeline))
		group.GET("/monthly", web.WrapH(api.getMonthlyStats))
		// HLS 播放列表（根据通道 ID 和时间范围生成 m3u8）
		group.GET("/channels/:cid/index.m3u8", api.channelPlaylist)
		group.GET("/:id", web.WrapH(api.getRecording))
		group.PUT("/:id", web.WrapH(api.updateRecording))
		group.DELETE("/:id", web.WrapH(api.deleteRecording))
		group.GET("/:id/download", api.downloadRecording)
	}

	// 静态文件服务，用于访问录像 MP4 文件
	// 路径格式: /static/recordings/xxx.mp4?token=xxx
	// Gin Static 支持 HTTP Range 请求，实现边下载边播放（秒播）
	if api.conf != nil && api.conf.Server.Recording.StorageDir != "" {
		slog.Info("注册录像静态文件服务", "path", "/static/recordings", "dir", api.conf.Server.Recording.StorageDir)
		g.Group("/static", handler...).Static("/recordings", api.conf.Server.Recording.StorageDir)
	}
}

// listRecordings 分页查询录像列表
func (a RecordingAPI) listRecordings(c *gin.Context, in *recording.FindRecordingInput) (any, error) {
	ctx := web.WithContext(c.Request)
	items, total, err := a.recordingCore.ListRecordings(ctx, in)
	return gin.H{"items": items, "total": total}, err
}

// getTimeline 获取时间轴数据
func (a RecordingAPI) getTimeline(c *gin.Context, in *recording.TimelineInput) (any, error) {
	items, err := a.recordingCore.GetTimeline(c.Request.Context(), in)
	return gin.H{"items": items}, err
}

func (a RecordingAPI) getRecording(c *gin.Context, in *recordingIDInput) (*recording.Recording, error) {
	return a.recordingCore.GetRecording(c.Request.Context(), in.ID)
}

func (a RecordingAPI) updateRecording(c *gin.Context, in *updateRecordingInput) (*recording.Recording, error) {
	return a.recordingCore.UpdateRecording(c.Request.Context(), &in.EditRecordingInput, in.ID)
}

func (a RecordingAPI) deleteRecording(c *gin.Context, in *recordingIDInput) (*recording.Recording, error) {
	return a.recordingCore.DeleteRecording(c.Request.Context(), in.ID)
}

// getMonthlyStats 获取月度录像统计
func (a RecordingAPI) getMonthlyStats(c *gin.Context, in *recording.MonthlyStatsInput) (*recording.MonthlyStatsOutput, error) {
	return a.recordingCore.GetMonthlyStats(c.Request.Context(), in)
}

// downloadRecording 下载录像文件
func (a RecordingAPI) downloadRecording(c *gin.Context) {
	recordingID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "msg": "invalid recording id"})
		return
	}

	rec, err := a.recordingCore.GetRecording(c.Request.Context(), recordingID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 1, "msg": err.Error()})
		return
	}

	// 构建文件完整路径
	filePath := a.recordingCore.GetFullPath(rec.Path)
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, gin.H{"code": 1, "msg": "recording file not found"})
		return
	}

	// 设置下载文件名
	fileName := filepath.Base(filePath)
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", fileName))
	c.File(filePath)
}

// channelPlaylist 生成 HLS m3u8 播放列表
// 根据通道 ID 和时间范围，动态生成包含多个 MP4 片段的 m3u8 文件
// 路径: /recordings/channels/:cid/index.m3u8?start_ms=xxx&end_ms=xxx&token=xxx
func (a RecordingAPI) channelPlaylist(c *gin.Context) {
	cid := c.Param("cid")
	if cid == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "msg": "cid is required"})
		return
	}

	startMs, _ := strconv.ParseInt(c.Query("start_ms"), 10, 64)
	endMs, _ := strconv.ParseInt(c.Query("end_ms"), 10, 64)
	token := c.Query("token")

	if startMs <= 0 || endMs <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "msg": "start_ms and end_ms are required"})
		return
	}

	// 获取时间范围内的录像列表（需要完整路径信息）
	ctx := web.WithContext(c.Request)
	recordings, _, err := a.recordingCore.ListRecordings(ctx, &recording.FindRecordingInput{
		CID:  cid,
		Page: 1, Size: 10000,
		StartMs: startMs, EndMs: endMs,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "msg": err.Error()})
		return
	}

	if len(recordings) == 0 {
		c.JSON(http.StatusNotFound, gin.H{"code": 1, "msg": "no recordings found in time range"})
		return
	}

	// 构建请求的 base URL
	scheme := "http"
	if c.Request.TLS != nil {
		scheme = "https"
	}
	baseURL := fmt.Sprintf("%s://%s", scheme, c.Request.Host)

	// 生成 m3u8 内容（带 token）
	m3u8Content := a.generateM3U8WithToken(recordings, baseURL, token)

	c.Header("Content-Type", "application/vnd.apple.mpegurl")
	c.Header("Cache-Control", "no-cache")
	c.String(http.StatusOK, m3u8Content)
}

// generateM3U8WithToken 根据录像列表生成 m3u8 播放列表（每个 MP4 URL 带 token）
func (a RecordingAPI) generateM3U8WithToken(recordings []*recording.Recording, baseURL, token string) string {
	count := len(recordings)
	if count == 0 {
		return ""
	}

	// 创建媒体播放列表 (winSize=0 表示 VOD，不使用滑动窗口)
	pl, err := m3u8.NewMediaPlaylist(0, uint(count))
	if err != nil {
		return ""
	}

	// 设置为 VOD 类型
	pl.MediaType = m3u8.VOD

	// 录像按时间升序排列
	sortedRecs := make([]*recording.Recording, len(recordings))
	copy(sortedRecs, recordings)
	// 按开始时间升序排序
	for i := 0; i < len(sortedRecs)-1; i++ {
		for j := i + 1; j < len(sortedRecs); j++ {
			if sortedRecs[i].StartedAt.After(sortedRecs[j].StartedAt.Time) {
				sortedRecs[i], sortedRecs[j] = sortedRecs[j], sortedRecs[i]
			}
		}
	}

	// 添加每个录像片段
	// URL 格式: /static/recordings/{path}?token=xxx
	// 使用相对路径（以 / 开头），让浏览器相对于当前域名访问
	// 这样无论通过代理还是直接访问都能正常工作
	// ZLM 录制的 fMP4 每个文件 DTS 都从 0 开始，必须在每个片段间添加 DISCONTINUITY
	// 告诉 HLS.js 重置解码器，避免 DTS 不连续导致的解析错误
	for i, rec := range sortedRecs {
		// 每个片段之间都添加 EXT-X-DISCONTINUITY 标签
		// ZLM 每个录像文件都是独立的 fMP4，DTS 从 0 开始，必须重置解码器
		if i > 0 {
			pl.SetDiscontinuity()
		}

		// 构建相对路径，去掉前导斜杠
		relativePath := strings.TrimPrefix(rec.Path, "/")

		// 使用相对路径（不带域名），让浏览器根据当前页面域名访问
		// 这样开发时通过 Vite 代理、生产时通过后端都能正常访问
		var uri string
		if token != "" {
			uri = fmt.Sprintf("/static/recordings/%s?token=%s", relativePath, token)
		} else {
			uri = fmt.Sprintf("/static/recordings/%s", relativePath)
		}
		_ = pl.Append(uri, rec.Duration, "")
	}

	// 关闭播放列表，添加 #EXT-X-ENDLIST 标签
	pl.Close()

	// 编码为字符串
	return pl.String()
}
