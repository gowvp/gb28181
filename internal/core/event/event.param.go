package event

import (
	"time"

	"github.com/ixugo/goddd/pkg/orm"
	"github.com/ixugo/goddd/pkg/web"
)

// ListEventInput 分页查询参数
type ListEventInput struct {
	web.PagerFilter
	web.DateFilter
	DID      string    `form:"did"`   // 设备 ID
	CID      string    `form:"cid"`   // 通道 ID
	Label    string    `form:"label"` // 检测标签
	BeforeAt time.Time `form:"-"`     // 内部清理用：started_at < BeforeAt
}

// GetEventInput 按 ID 查询参数
type GetEventInput struct {
	ID int64 `uri:"id" binding:"required"`
}

// CreateEventInput 新增事件参数
type CreateEventInput struct {
	DID       string   `json:"-"`          // 设备 ID (API 层填充)
	CID       string   `json:"-"`          // 通道 ID (API 层填充)
	StartedAt orm.Time `json:"started_at"` // 事件开始时间 (毫秒时间戳)
	EndedAt   orm.Time `json:"ended_at"`   // 事件结束时间 (毫秒时间戳)
	Label     string   `json:"label"`      // 检测标签
	Score     float32  `json:"score"`      // 置信度
	Zones     string   `json:"zones"`      // 检测区域 JSON
	ImagePath string   `json:"image_path"` // 图片相对路径
	Model     string   `json:"model"`      // 分析模型名称
}

// UpdateEventInput 编辑事件参数
type UpdateEventInput struct {
	ID      int64    `uri:"id" binding:"required"`
	EndedAt orm.Time `json:"ended_at"` // 事件结束时间 (毫秒时间戳)
}

// DeleteEventInput 删除事件参数
type DeleteEventInput struct {
	ID int64 `uri:"id" binding:"required"`
}
