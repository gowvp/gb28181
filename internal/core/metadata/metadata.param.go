package metadata

import "github.com/ixugo/goddd/pkg/web"

// ListMetadataInput 分页查询参数
type ListMetadataInput struct {
	web.PagerFilter
}

// GetMetadataInput 按 ID 查询参数
type GetMetadataInput struct {
	ID string `uri:"id" binding:"required"`
}

// CreateMetadataInput 新增数据参数
type CreateMetadataInput struct {
	ID  string `json:"id" binding:"required,max=64"`      // 调用方指定的唯一标识
	Ext string `json:"ext" binding:"required,max=131072"` // 序列化的 JSON 字符串

	CreatedBy     string `json:"-"` // 创建者（API层填充）
	LastUpdatedBy string `json:"-"` // 最后更新者（API层填充）
}

// UpdateMetadataInput 编辑数据参数
type UpdateMetadataInput struct {
	ID  string `uri:"id"`
	Ext string `json:"ext" binding:"required,max=131072"` // 序列化的 JSON 字符串

	LastUpdatedBy string `json:"-"` // 最后更新者（API层填充）
}

// DeleteMetadataInput 删除数据参数
type DeleteMetadataInput struct {
	ID string `uri:"id" binding:"required"`
}

// SaveMetadataInput 保存数据参数，合并创建与更新为一个幂等操作
type SaveMetadataInput struct {
	Ext string `json:"ext" binding:"required,max=131072"` // 序列化的 JSON 字符串

	CreatedBy     string `json:"-"` // 创建者（API层填充）
	LastUpdatedBy string `json:"-"` // 最后更新者（API层填充）
}
