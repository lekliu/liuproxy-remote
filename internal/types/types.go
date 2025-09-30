// --- START OF COMPLETE REPLACEMENT for types.go ---
package types

import (
	"bufio"
	"net"
)

// Agent 接口定义了所有代理处理器的通用行为。
// 如果 reader 为 nil，处理器应从 conn 创建自己的 reader。
// 如果 reader 不为 nil，处理器必须使用这个 reader 来读取初始数据。
type Agent interface {
	HandleConnection(conn net.Conn, reader *bufio.Reader)
}

// --- 1/1 MODIFICATION START ---
// 将所有配置结构体统一到这里，并添加 ini 标签

// CommonConf 包含 local 和 remote 模式共有的配置
type CommonConf struct {
	Mode           string `ini:"mode"`
	MaxConnections int    `ini:"maxConnections"`
	BufferSize     int    `ini:"bufferSize"`
	Crypt          int    `ini:"crypt"`
}

// RemoteConf 包含 remote 模式特有的配置
type RemoteConf struct {
	PortWsSvr int `ini:"port_ws_svr"`
}

// Config 是整个应用程序的统一配置结构体
type Config struct {
	CommonConf `ini:"common"`
	RemoteConf `ini:"remote"`
}
