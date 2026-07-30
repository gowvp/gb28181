package notify

import "sync/atomic"

// warnFunc 存储广播函数引用，由 api 层在初始化 Hub 时注入
var warnFunc atomic.Value

// ipcWarnFunc 存储设备相关警告广播函数引用
var ipcWarnFunc atomic.Value

// ipcInfoFunc 存储设备相关信息广播函数引用
var ipcInfoFunc atomic.Value

// SetWarnFunc 注册全局警告广播函数，在 WebSocket Hub 初始化后由 api 层调用
func SetWarnFunc(fn func(string)) {
	warnFunc.Store(fn)
}

// SetIPCWarnFunc 注册设备相关警告广播函数
func SetIPCWarnFunc(fn func(msg, deviceID, name string)) {
	ipcWarnFunc.Store(fn)
}

// SetIPCInfoFunc 注册设备相关信息广播函数
func SetIPCInfoFunc(fn func(msg, deviceID, name string)) {
	ipcInfoFunc.Store(fn)
}

// Warn 向所有已连接的 WebSocket 客户端广播一条通用警告通知
func Warn(msg string) {
	if fn, ok := warnFunc.Load().(func(string)); ok && fn != nil {
		fn(msg)
	}
}

// IPCWarn 向所有已连接的 WebSocket 客户端广播一条设备相关警告通知
func IPCWarn(msg, deviceID, name string) {
	if fn, ok := ipcWarnFunc.Load().(func(string, string, string)); ok && fn != nil {
		fn(msg, deviceID, name)
	}
}

// IPCInfo 向所有已连接的 WebSocket 客户端广播一条设备相关信息通知
func IPCInfo(msg, deviceID, name string) {
	if fn, ok := ipcInfoFunc.Load().(func(string, string, string)); ok && fn != nil {
		fn(msg, deviceID, name)
	}
}
