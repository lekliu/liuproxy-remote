// --- START OF COMPLETE REPLACEMENT for liuproxy-remote/internal/shared/conn_adapter.go ---
package shared

import (
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

// upgrader 是一个全局的 WebSocket 升级器实例
var upgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

// WebSocketConnAdapter 实现了 net.Conn 接口，将 websocket.Conn 包装起来
type WebSocketConnAdapter struct {
	*websocket.Conn
	readBuffer *ThreadSafeBuffer
}

// NewWebSocketConnAdapterServer 端使用此函数来升级一个 HTTP 请求为 WebSocket 连接。
func NewWebSocketConnAdapterServer(w http.ResponseWriter, r *http.Request) (*WebSocketConnAdapter, error) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return nil, err
	}
	return &WebSocketConnAdapter{
		Conn:       ws,
		readBuffer: NewThreadSafeBuffer(),
	}, nil
}

// Read 方法实现了 io.Reader 接口。
func (wsc *WebSocketConnAdapter) Read(b []byte) (int, error) {
	if wsc.readBuffer.Len() == 0 {
		msgType, msg, err := wsc.Conn.ReadMessage()
		if err != nil {
			return 0, err
		}

		if msgType != websocket.BinaryMessage {
			return 0, fmt.Errorf("received non-binary message from websocket")
		}
		if _, err := wsc.readBuffer.Write(msg); err != nil {
			return 0, err
		}
	}
	return wsc.readBuffer.Read(b)
}

// Write 方法实现了 io.Writer 接口。
func (wsc *WebSocketConnAdapter) Write(b []byte) (int, error) {
	dataCopy := make([]byte, len(b))
	copy(dataCopy, b)

	err := wsc.Conn.WriteMessage(websocket.BinaryMessage, dataCopy)
	if err != nil {
		return 0, err
	}
	return len(b), nil
}

// Close 实现了 io.Closer 接口。
func (wsc *WebSocketConnAdapter) Close() error {
	return wsc.Conn.Close()
}

// LocalAddr 实现了 net.Conn 接口。
func (wsc *WebSocketConnAdapter) LocalAddr() net.Addr {
	return wsc.Conn.LocalAddr()
}

// RemoteAddr 实现了 net.Conn 接口。
func (wsc *WebSocketConnAdapter) RemoteAddr() net.Addr {
	return wsc.Conn.RemoteAddr()
}

// SetDeadline 实现了 net.Conn 接口。
func (wsc *WebSocketConnAdapter) SetDeadline(t time.Time) error {
	_ = wsc.Conn.SetReadDeadline(t)
	return wsc.Conn.SetWriteDeadline(t)
}

// SetReadDeadline 实现了 net.Conn 接口。
func (wsc *WebSocketConnAdapter) SetReadDeadline(t time.Time) error {
	return wsc.Conn.SetReadDeadline(t)
}

// SetWriteDeadline 实现了 net.Conn 接口。
func (wsc *WebSocketConnAdapter) SetWriteDeadline(t time.Time) error {
	return wsc.Conn.SetWriteDeadline(t)
}
