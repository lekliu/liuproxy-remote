package tunnel

import (
	"bufio"
	"github.com/gorilla/websocket"
	"io"
	"liuproxy_remote/remote/shared"
	"liuproxy_remote/remote/types"
	"log"
	"net"
	"net/http"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	CheckOrigin:     func(r *http.Request) bool { return true }, // 允许所有来源
}

// HandleWebSocketConnection 处理一个疑似 WebSocket 的连接
func HandleWebSocketConnection(conn net.Conn, reader *bufio.Reader, cfg *types.Config) {
	// 将 conn 和 reader 组合成一个可以被 http.ReadRequest 读取的 io.Reader
	combinedReader := bufio.NewReader(io.MultiReader(reader, conn))

	// 1. 解析 HTTP 请求
	req, err := http.ReadRequest(combinedReader)
	if err != nil {
		log.Printf("[REMOTE-WS] Failed to read HTTP request: %v", err)
		conn.Close()
		return
	}

	// 2. 检查路径是否为 /tunnel
	//if strings.ToLower(req.URL.Path) != "/tunnel" {
	//	log.Printf("[REMOTE-WS] Rejected WS connection with incorrect path: %s", req.URL.Path)
	//	// 返回一个标准的 HTTP 404 响应
	//	resp := "HTTP/1.1 404 Not Found\r\n\r\nPage not found."
	//	conn.Write([]byte(resp))
	//	conn.Close()
	//	return
	//}

	// 3. 升级为 WebSocket 连接
	wsConn, err := upgrader.Upgrade(hijack(conn, reader), req, nil)
	if err != nil {
		log.Printf("[REMOTE-WS] Failed to upgrade to WebSocket: %v", err)
		// Upgrade 会自动处理错误响应, 我们只需确保连接被关闭
		return
	}
	defer wsConn.Close()

	log.Printf("[REMOTE-WS] WebSocket connection established from %s", wsConn.RemoteAddr())

	// 4. 将 WebSocket 连接适配为 net.Conn
	adaptedConn := shared.NewWebSocketConnAdapter(wsConn)

	// 5. 【约定】WebSocket 传输必须使用 Mux 模式。直接交给 Mux 处理器。
	// 注意：这里我们传入 nil 作为 reader，因为 wsConn 内部自己处理帧，是干净的。
	HandleMuxSession(adaptedConn, nil, cfg)
}

// hijack 是一个辅助结构，用于将 net.Conn 包装起来以满足 http.ResponseWriter 接口
// 这是 `websocket.Upgrader` 所必需的技巧
type hijackedConn struct {
	net.Conn
	io.Reader
}

func (hc *hijackedConn) Header() http.Header {
	return http.Header{}
}
func (hc *hijackedConn) Write(b []byte) (int, error) {
	return hc.Conn.Write(b)
}
func (hc *hijackedConn) WriteHeader(int) {}

func (hc *hijackedConn) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return hc.Conn, bufio.NewReadWriter(bufio.NewReader(hc.Reader), bufio.NewWriter(hc.Conn)), nil
}

func hijack(conn net.Conn, reader io.Reader) http.ResponseWriter {
	return &hijackedConn{Conn: conn, Reader: reader}
}
