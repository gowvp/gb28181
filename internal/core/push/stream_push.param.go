package push

import "github.com/ixugo/goddd/pkg/web"

// ListStreamPushInput 分页查询参数
type ListStreamPushInput struct {
	web.PagerFilter
	Status string `form:"status"` // 推流状态(PUSHING)
	Key    string `form:"key"`    // 搜索关键字(ID/App/Stream)
}

// GetStreamPushInput 按 ID 查询参数
type GetStreamPushInput struct {
	ID string `uri:"id" binding:"required"`
}

// CreateStreamPushInput 新增推流参数
type CreateStreamPushInput struct {
	Name           string `json:"name"`             // 推流名称
	App            string `json:"app,required"`     // 应用名
	Stream         string `json:"stream,required"`  // 流 ID
	IsAuthDisabled bool   `json:"is_auth_disabled"` // 是否禁用推流鉴权
}

// UpdateStreamPushInput 编辑推流参数
type UpdateStreamPushInput struct {
	ID             string `uri:"id"`
	App            string `json:"app"`              // 应用名
	Stream         string `json:"stream"`           // 流 ID
	IsAuthDisabled bool   `json:"is_auth_disabled"` // 是否禁用推流鉴权
}

// DeleteStreamPushInput 删除推流参数
type DeleteStreamPushInput struct {
	ID string `uri:"id" binding:"required"`
}

// ListStreamPushOutputItem 列表响应行，附带推流地址
type ListStreamPushOutputItem struct {
	StreamPush
	PushAddrs []string `json:"push_addrs"`
}
