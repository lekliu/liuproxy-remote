package shared

import (
	"fmt"
	"net"
	"time"

	"github.com/gorilla/websocket"
)

type WebSocketConnAdapter struct {
	*websocket.Conn
	readBuffer *ThreadSafeBuffer
}

func NewWebSocketConnAdapter(ws *websocket.Conn) net.Conn {
	return &WebSocketConnAdapter{
		Conn:       ws,
		readBuffer: NewThreadSafeBuffer(),
	}
}
func (wsc *WebSocketConnAdapter) Read(b []byte) (int, error) {
	if wsc.readBuffer.Len() == 0 {
		msgType, msg, err := wsc.Conn.ReadMessage()
		if err != nil {
			return 0, err
		}
		if msgType != websocket.BinaryMessage {
			return 0, fmt.Errorf("received non-binary message")
		}
		if _, err := wsc.readBuffer.Write(msg); err != nil {
			return 0, err
		}
	}
	return wsc.readBuffer.Read(b)
}
func (wsc *WebSocketConnAdapter) Write(b []byte) (int, error) {
	err := wsc.Conn.WriteMessage(websocket.BinaryMessage, b)
	if err != nil {
		return 0, err
	}
	return len(b), nil
}
func (wsc *WebSocketConnAdapter) Close() error         { return wsc.Conn.Close() }
func (wsc *WebSocketConnAdapter) LocalAddr() net.Addr  { return wsc.Conn.LocalAddr() }
func (wsc *WebSocketConnAdapter) RemoteAddr() net.Addr { return wsc.Conn.RemoteAddr() }
func (wsc *WebSocketConnAdapter) SetDeadline(t time.Time) error {
	_ = wsc.Conn.SetReadDeadline(t)
	return wsc.Conn.SetWriteDeadline(t)
}
func (wsc *WebSocketConnAdapter) SetReadDeadline(t time.Time) error {
	return wsc.Conn.SetReadDeadline(t)
}
func (wsc *WebSocketConnAdapter) SetWriteDeadline(t time.Time) error {
	return wsc.Conn.SetWriteDeadline(t)
}
