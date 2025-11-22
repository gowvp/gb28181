package onvifadapter

import (
	"strings"

	"github.com/gowvp/onvif"
)

type DiscoverResponse struct {
	Addr string `json:"addr"`
}

func toDiscoverResponse(dev *onvif.Device) *DiscoverResponse {
	addr := dev.GetDeviceParams().Xaddr
	if !strings.Contains(addr, ":") {
		addr += ":80"
	}
	return &DiscoverResponse{
		Addr: addr,
	}
}
