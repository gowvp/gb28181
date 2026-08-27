package metadata

import "github.com/ixugo/goddd/pkg/orm"

// Storer 持久层聚合入口
type Storer interface {
	Begin() (orm.Tx, error)
	Metadata() MetadataStorer
}

// Core 通用数据持久化领域核心
type Core struct {
	store Storer
}

// NewCore 创建领域核心
func NewCore(store Storer) Core {
	return Core{store: store}
}
