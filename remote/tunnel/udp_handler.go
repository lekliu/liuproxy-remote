// --- START OF NEW FILE remote/tunnel/udp_handler.go ---
package tunnel

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"net"
	"strconv"
	"sync"
	"time"

	"liuproxy_remote/remote/core/securecrypt"
	"liuproxy_remote/remote/types"
)

const udpSessionTimeout = 60 * time.Second

// udpSession 存储了一个UDP会话所需的信息
type udpSession struct {
	// 连接到最终目标的UDP "连接"
	targetConn net.PacketConn
	// 会话的过期时间
	expiry time.Time
}

// UDPHandler 负责管理所有的UDP会话
type UDPHandler struct {
	cfg            *types.Config
	listener       *net.UDPConn
	sessions       sync.Map // map[string]*udpSession, key是 gateway_addr:port
	sessionCleanup *time.Ticker
	cipher         *securecrypt.Cipher
}

// NewUDPHandler 创建并初始化一个新的UDPHandler
func NewUDPHandler(cfg *types.Config, listener *net.UDPConn) *UDPHandler {
	cipher, err := securecrypt.NewCipher(cfg.Crypt)
	if err != nil {
		log.Fatalf("[REMOTE-UDP] Failed to create cipher for UDP handler: %v", err)
	}

	handler := &UDPHandler{
		cfg:            cfg,
		listener:       listener,
		sessionCleanup: time.NewTicker(30 * time.Second),
		cipher:         cipher,
	}

	go handler.cleanupLoop()
	return handler
}

// Listen 开始监听并处理传入的UDP包
func (h *UDPHandler) Listen() {
	buf := make([]byte, h.cfg.BufferSize)
	for {
		n, gatewayAddr, err := h.listener.ReadFromUDP(buf)
		if err != nil {
			log.Printf("[REMOTE-UDP] Error reading from UDP listener: %v", err)
			return // 监听器关闭时会出错，循环自然结束
		}

		encryptedPayload := make([]byte, n)
		copy(encryptedPayload, buf[:n])

		// 每个包都在独立的goroutine中处理，以实现高并发
		go h.handlePacket(encryptedPayload, gatewayAddr)
	}
}

func (h *UDPHandler) handlePacket(encryptedPayload []byte, gatewayAddr *net.UDPAddr) {
	// 1. 解密
	payload, err := h.cipher.Decrypt(encryptedPayload)
	if err != nil {
		log.Printf("[REMOTE-UDP] Failed to decrypt UDP packet from %s: %v", gatewayAddr, err)
		return
	}

	// 2. 解析SOCKS5 UDP头部
	targetAddr, data, err := parseSocks5UDPHeader(payload)
	if err != nil {
		log.Printf("[REMOTE-UDP] Failed to parse SOCKS5 UDP header from %s: %v", gatewayAddr, err)
		return
	}
	log.Printf("[REMOTE-UDP-DIAG] Received packet from %s, forwarding to %s", gatewayAddr, targetAddr)

	// 3. 获取或创建会话
	sessionKey := gatewayAddr.String()
	session, err := h.getOrCreateSession(sessionKey, gatewayAddr)
	if err != nil {
		log.Printf("[REMOTE-UDP] Failed to get or create session for %s: %v", gatewayAddr, err)
		return
	}

	// 4. 发送数据到最终目标
	_, err = session.targetConn.WriteTo(data, targetAddr)
	if err != nil {
		log.Printf("[REMOTE-UDP] Failed to write to target %s: %v", targetAddr, err)
	}
}

func (h *UDPHandler) getOrCreateSession(sessionKey string, gatewayAddr *net.UDPAddr) (*udpSession, error) {
	// 尝试加载现有会话
	if s, ok := h.sessions.Load(sessionKey); ok {
		session := s.(*udpSession)
		session.expiry = time.Now().Add(udpSessionTimeout) // 续期
		return session, nil
	}

	// 创建新会话
	log.Printf("[REMOTE-UDP-DIAG] Creating new UDP session for %s", sessionKey)
	targetConn, err := net.ListenPacket("udp", "0.0.0.0:0")
	if err != nil {
		return nil, fmt.Errorf("failed to create outbound UDP socket: %w", err)
	}

	newSession := &udpSession{
		targetConn: targetConn,
		expiry:     time.Now().Add(udpSessionTimeout),
	}

	h.sessions.Store(sessionKey, newSession)

	// 为这个新会话启动一个专门的“回复”goroutine
	go h.replyLoop(newSession, gatewayAddr)

	return newSession, nil
}

// replyLoop 持续从目标UDP连接读取数据，并将其转发回对应的gateway
func (h *UDPHandler) replyLoop(session *udpSession, gatewayAddr *net.UDPAddr) {
	buf := make([]byte, h.cfg.BufferSize)
	for {
		session.targetConn.SetReadDeadline(time.Now().Add(udpSessionTimeout + 5*time.Second))
		n, remoteAddr, err := session.targetConn.ReadFrom(buf)
		if err != nil {
			// 超时或其他错误，意味着此会话可以关闭了
			log.Printf("[REMOTE-UDP-DIAG] Reply loop for %s terminating: %v", gatewayAddr, err)
			session.targetConn.Close()
			h.sessions.Delete(gatewayAddr.String())
			return
		}

		log.Printf("[REMOTE-UDP-DIAG] Received reply from %s for %s", remoteAddr, gatewayAddr)

		// 封装成SOCKS5 UDP包
		var replyBuf bytes.Buffer
		// RSV, FRAG, ATYP
		replyBuf.Write([]byte{0x00, 0x00, 0x00})
		if udpRemoteAddr, ok := remoteAddr.(*net.UDPAddr); ok {
			if ipv4 := udpRemoteAddr.IP.To4(); ipv4 != nil {
				replyBuf.WriteByte(0x01) // IPv4
				replyBuf.Write(ipv4)
			} else {
				// 暂不支持IPv6回复
				continue
			}
			binary.Write(&replyBuf, binary.BigEndian, uint16(udpRemoteAddr.Port))
		} else {
			continue // 不支持非UDP地址
		}
		replyBuf.Write(buf[:n])

		// 加密
		encryptedReply, err := h.cipher.Encrypt(replyBuf.Bytes())
		if err != nil {
			log.Printf("[REMOTE-UDP] Failed to encrypt reply for %s: %v", gatewayAddr, err)
			continue
		}

		// 发送回gateway
		_, err = h.listener.WriteTo(encryptedReply, gatewayAddr)
		if err != nil {
			log.Printf("[REMOTE-UDP] Failed to send reply to gateway %s: %v", gatewayAddr, err)
		}
	}
}

// cleanupLoop 定期清理过期的UDP会话
func (h *UDPHandler) cleanupLoop() {
	for range h.sessionCleanup.C {
		now := time.Now()
		h.sessions.Range(func(key, value interface{}) bool {
			session := value.(*udpSession)
			if now.After(session.expiry) {
				log.Printf("[REMOTE-UDP-DIAG] Cleaning up expired session for %s", key.(string))
				session.targetConn.Close()
				h.sessions.Delete(key)
			}
			return true
		})
	}
}

// parseSocks5UDPHeader 解析 SOCKS5 UDP 请求的头部
func parseSocks5UDPHeader(data []byte) (*net.UDPAddr, []byte, error) {
	if len(data) < 4 {
		return nil, nil, io.ErrShortBuffer
	}
	// RSV(2), FRAG(1)
	offset := 3
	addrType := data[offset]
	offset++

	var host string
	switch addrType {
	case 0x01: // IPv4
		if len(data) < offset+4+2 {
			return nil, nil, io.ErrShortBuffer
		}
		host = net.IP(data[offset : offset+4]).String()
		offset += 4
	case 0x03: // Domain
		if len(data) < offset+1 {
			return nil, nil, io.ErrShortBuffer
		}
		domainLen := int(data[offset])
		offset++
		if len(data) < offset+domainLen+2 {
			return nil, nil, io.ErrShortBuffer
		}
		host = string(data[offset : offset+domainLen])
		offset += domainLen
	default:
		return nil, nil, fmt.Errorf("unsupported address type: %d", addrType)
	}

	port := binary.BigEndian.Uint16(data[offset : offset+2])
	offset += 2

	addr, err := net.ResolveUDPAddr("udp", net.JoinHostPort(host, strconv.Itoa(int(port))))
	if err != nil {
		return nil, nil, err
	}

	return addr, data[offset:], nil
}
