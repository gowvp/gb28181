package bz

import "strings"

const (
	IDPrefixGB           = "gb" // 国标设备
	IDPrefixGBChannel    = "ch" // 国标通道 id 前缀，channel
	IDPrefixOnvif        = "on" // onvif 设备 id 前缀
	IDPrefixOnvifChannel = "pr" // onvif 通道 id 前缀，profile
	IDPrefixRTMP         = "mp" // rtmp ID 前缀，取 rtmp 后缀的 mp，不好记但是清晰
	IDPrefixRTSP         = "sp" // rtsp ID 前缀，取 rtsp 后缀的 sp，不好记但是清晰
)

func IsGB28181(stream string) bool {
	return strings.HasPrefix(stream, IDPrefixGB) || strings.HasPrefix(stream, IDPrefixGBChannel)
}

func IsOnvif(stream string) bool {
	return strings.HasPrefix(stream, IDPrefixOnvif) || strings.HasPrefix(stream, IDPrefixOnvifChannel)
}

func IsRTMP(stream string) bool {
	return strings.HasPrefix(stream, IDPrefixRTMP)
}

func IsRTSP(stream string) bool {
	return strings.HasPrefix(stream, IDPrefixRTSP)
}
