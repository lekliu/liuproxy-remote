// --- START OF MODIFIED FILE liuproxy_go/internal/agent/socks5/agent.go ---
package tunnel

import (
	"bufio"
	"liuproxy_remote/internal/types"
	"net"
)

type Agent struct {
	config *types.Config

	bufferSize     int
	remoteUdpRelay *RemoteUDPRelay
}

func NewAgent(cfg *types.Config, bufferSize int) types.Agent {
	return &Agent{
		config:     cfg,
		bufferSize: bufferSize,
	}
}
func (a *Agent) HandleConnection(conn net.Conn, reader *bufio.Reader) {
	defer conn.Close()
	// remote端只执行 remote 逻辑
	a.handleRemote(conn)
}

func (a *Agent) SetUDPRelay(relay *RemoteUDPRelay) {
	a.remoteUdpRelay = relay
}
