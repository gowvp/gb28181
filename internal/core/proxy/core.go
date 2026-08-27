package proxy

import (
	"github.com/ixugo/goddd/domain/uniqueid"
	"github.com/ixugo/goddd/pkg/orm"
)

// Storer 持久层聚合入口
type Storer interface {
	Begin() (orm.Tx, error)
	StreamProxy() StreamProxyStorer
}

// Core 拉流代理领域核心
type Core struct {
	store    Storer
	uniqueID uniqueid.Core
}

// NewCore 创建领域核心
func NewCore(store Storer, uni uniqueid.Core) *Core {
	return &Core{
		store:    store,
		uniqueID: uni,
	}
}
