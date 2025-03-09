// Code generated by gowebx, DO AVOID EDIT.
package gb28181

import (
	"github.com/gowvp/gb28181/internal/core/uniqueid"
)

// Storer data persistence
type Storer interface {
	Device() DeviceStorer
	Channel() ChannelStorer
}

// Core business domain
type Core struct {
	store    Storer
	uniqueID uniqueid.Core
}

// NewCore create business domain
func NewCore(store Storer, uni uniqueid.Core) Core {
	return Core{store: store, uniqueID: uni}
}
