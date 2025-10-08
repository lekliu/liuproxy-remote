package tunnel

import (
	"bufio"
	"liuproxy_remote/remote/types"
	"net"
)

// HandleTCPConnection 是 remote 端处理新连接的唯一入口。
func HandleTCPConnection(inboundConn net.Conn, reader *bufio.Reader, cfg *types.Config) {
	defer inboundConn.Close()
	//log.Printf("[REMOTE-TCP-DIAG] Accepted new connection from %s", inboundConn.RemoteAddr())

	// 将连接和 reader 传递给 tcp_handler.go 中的处理器
	handleTCPStream(inboundConn, reader, cfg)
}
