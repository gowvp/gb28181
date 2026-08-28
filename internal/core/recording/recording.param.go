package recording

import (
	"time"

	"github.com/ixugo/goddd/pkg/orm"
	"github.com/ixugo/goddd/pkg/web"
)

type FindRecordingInput struct {
	web.PagerFilter
	web.DateFilter
	CID    string `form:"cid"`    // 通道 ID (channel.ID)
	App    string `form:"app"`    // ZLM 应用名
	Stream string `form:"stream"` // ZLM 流 ID

	// 以下为内部字段，API 层不暴露，供 cleanup/timeline 使用
	StartedAtBefore *time.Time `form:"-"` // started_at < value
	StartedAtAfter  *time.Time `form:"-"` // started_at >= value
	EndedAtAfter    *time.Time `form:"-"` // ended_at > value（timeline 重叠查询）
	DeleteFlagEq    *bool      `form:"-"` // delete_flag = value
	OrderBy         string     `form:"-"` // 覆盖默认排序，如 "started_at ASC"
}

type EditRecordingInput struct {
	ObjectCount int `json:"object_count"` // AI检测对象数量（从event表统计）
}

type AddRecordingInput struct {
	CID       string   `json:"-"`          // 通道 ID（由 API 层填充）
	App       string   `json:"app"`        // ZLM 应用名
	Stream    string   `json:"stream"`     // ZLM 流 ID
	StartedAt orm.Time `json:"started_at"` // 录像开始时间
	EndedAt   orm.Time `json:"ended_at"`   // 录像结束时间
	Duration  float64  `json:"duration"`   // 持续时长（秒）
	Path      string   `json:"path"`       // 文件相对路径
	Size      int64    `json:"size"`       // 文件大小（字节）
}

// TimelineInput 时间轴查询参数
type TimelineInput struct {
	web.DateFilter
	CID string `form:"cid"` // 通道 ID
}

// MonthlyStatsInput 月度统计查询参数
type MonthlyStatsInput struct {
	CID   string `form:"cid"`   // 通道 ID（可选，不传则查所有通道）
	Year  int    `form:"year"`  // 年份，如 2024
	Month int    `form:"month"` // 月份，1-12
}

// MonthlyStatsOutput 月度统计输出
type MonthlyStatsOutput struct {
	Year     int    `json:"year"`      // 年份
	Month    int    `json:"month"`     // 月份
	Days     int    `json:"days"`      // 该月总天数
	HasVideo string `json:"has_video"` // 位图字符串，如 "10101010..." 第 1 天有录像则第 1 位为 1
}
