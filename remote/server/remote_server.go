package server

import (
	"bufio"
	"fmt"
	"liuproxy_remote/remote/tunnel"
	"log"
	"net"
	"time"
)

// runRemote 负责初始化和运行 remote 模式的所有服务
func (s *AppServer) runRemote() {
	log.Println("Initializing remote listeners...")

	// 1. 确定监听端口
	listenPort := s.cfg.RemoteConf.PortWsSvr
	if listenPort <= 0 {
		log.Fatalln("Remote port (port_ws_svr) is not configured.")
		return
	}
	addr := fmt.Sprintf("0.0.0.0:%d", listenPort)

	// 2. 启动 TCP 监听器
	tcpListener, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("Failed to listen on TCP port %s: %v", addr, err)
	}
	defer tcpListener.Close()
	log.Printf(">>> SUCCESS: GoRemote v3 (TCP) server listening on %s", addr)

	// --- 新增: UDP 监听 ---
	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		log.Fatalf("Failed to resolve UDP address %s: %v", addr, err)
	}
	udpListener, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		log.Fatalf("Failed to listen on UDP port %s: %v", addr, err)
	}
	defer udpListener.Close()
	log.Printf(">>> SUCCESS: GoRemote v3 (UDP) server listening on %s", addr)

	logLocalIPs(listenPort)

	// --- 新增: 启动 UDP 包处理循环 ---
	udpHandler := tunnel.NewUDPHandler(s.cfg, udpListener)
	s.waitGroup.Add(1)
	go func() {
		defer s.waitGroup.Done()
		udpHandler.Listen()
	}()

	// 3. 接受连接并处理
	for {
		conn, err := tcpListener.Accept()
		if err != nil {
			log.Printf("Failed to accept connection: %v", err)
			continue
		}
		s.waitGroup.Add(1)
		go func() {
			defer s.waitGroup.Done()
			//tunnel.HandleTCPConnection(conn, s.cfg)
			// 在这里进行模式判断
			s.dispatchTCPConnection(conn)
		}()
	}
}

// dispatchTCPConnection 根据第一个字节判断连接模式
func (s *AppServer) dispatchTCPConnection(conn net.Conn) {
	defer func() {
		if r := recover(); r != nil {
			//log.Printf("[REMOTE-DISPATCH] Panic recovered: %v", r)
			conn.Close()
		}
	}()

	reader := bufio.NewReader(conn)

	// 设置一个短暂的读取超时，以应对不发送任何数据的客户端
	conn.SetReadDeadline(time.Now().Add(3 * time.Second))

	// 预读2个字节
	header, err := reader.Peek(2)
	// 判断后立即清除超时
	conn.SetReadDeadline(time.Time{})

	if err != nil {
		log.Printf("[REMOTE-DISPATCH] Failed to peek header for mode detection: %v. Closing connection.", err)
		conn.Close()
		return
	}

	switch {
	// 情况一: WebSocket 模式 (检查 "GE" from "GET /...")
	case header[0] == 'G' && header[1] == 'E':
		log.Printf("[REMOTE-DISPATCH] WebSocket mode detected from %s.", conn.RemoteAddr())
		tunnel.HandleWebSocketConnection(conn, reader, s.cfg)

	// 情况二: Mux 模式 (检查 smux v1 或 v2 或 v3 头部)
	case (header[0] == 1 || header[0] == 2 || header[0] == 3) && header[1] == 0:
		//log.Printf("[REMOTE-DISPATCH] MUX mode detected (version %d) from %s.", header[0], conn.RemoteAddr())
		tunnel.HandleMuxSession(conn, reader, s.cfg)

	// 情况三: Multi-Conn 模式
	default:
		//log.Printf("[REMOTE-DISPATCH] Multi-Conn mode detected from %s.", conn.RemoteAddr())
		tunnel.HandleTCPConnection(conn, reader, s.cfg)
	}
}

// logLocalIPs finds and prints available non-loopback IPv4 addresses.
func logLocalIPs(port int) {
	interfaces, err := net.Interfaces()
	if err != nil {
		log.Printf("Could not get network interfaces: %v", err)
		return
	}

	log.Println("--- Available server addresses for client configuration ---")
	for _, i := range interfaces {
		addrs, err := i.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip == nil || ip.IsLoopback() {
				continue
			}
			ip = ip.To4()
			if ip != nil {
				log.Printf("  -> %s:%d", ip.String(), port)
			}
		}
	}
}
