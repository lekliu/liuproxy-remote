// --- START OF COMPLETE REPLACEMENT for remote_server.go ---
package server

import (
	"fmt"
	"liuproxy_remote/internal/shared"
	"liuproxy_remote/internal/tunnel"
	"log"
	"net"
	"net/http"
)

// runRemote 负责初始化和运行 remote 模式的所有服务
func (s *AppServer) runRemote() {
	log.Println("Initializing remote listeners...")

	// 1. 创建一个 Agent 实例，用于处理 SOCKS5 逻辑
	socksAgent := tunnel.NewAgent(s.cfg, s.cfg.CommonConf.BufferSize).(*tunnel.Agent)

	// 2. 创建 UDP 中继器并关联到 Agent
	udpRelay, err := tunnel.NewRemoteUDPRelay(*s.cfg, s.cfg.CommonConf.BufferSize)
	if err != nil {
		log.Fatalf("Failed to create Remote UDP Relay: %v", err)
	}
	socksAgent.SetUDPRelay(udpRelay)

	// 3. 确定监听端口 (PaaS 适配逻辑保持不变)
	wsPort := s.cfg.RemoteConf.PortWsSvr
	if wsPort <= 0 {
		log.Fatalln("Remote WebSocket port (port_ws_svr) is not configured.")
		return
	}

	// 4. 设置 HTTP Mux 和处理器
	mux := http.NewServeMux()

	// 健康检查端点
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("LiuProxy Remote is healthy."))
	})

	// 隧道处理器
	mux.HandleFunc("/tunnel", func(w http.ResponseWriter, r *http.Request) {
		// a. 将 HTTP 连接升级为 WebSocket 连接
		conn, err := shared.NewWebSocketConnAdapterServer(w, r)
		if err != nil {
			// 升级失败时，库会自动返回HTTP错误，我们只需记录
			log.Printf("[Remote] WebSocket upgrade failed: %v", err)
			return
		}

		// b. 直接将这个连接交给 Agent 处理
		//    Agent 内部会创建一个 RemoteTunnel 来管理这个连接
		s.waitGroup.Add(1)
		go func() {
			defer s.waitGroup.Done()
			defer func() {
				if r := recover(); r != nil {
					log.Printf("!!! PANIC recovered in remote handler for %s: %v", conn.RemoteAddr(), r)
				}
				conn.Close()
			}()
			// 直接调用 Agent 的 HandleConnection，它会进入 remote 分支
			socksAgent.HandleConnection(conn, nil)
		}()
	})

	addr := fmt.Sprintf("0.0.0.0:%d", wsPort)
	log.Printf(">>> SUCCESS: Unified tunnel server listening on ws://%s", addr)
	logLocalIPs(wsPort)

	// 5. 启动 HTTP 服务器
	httpServer := &http.Server{Addr: addr, Handler: mux}
	s.waitGroup.Add(1)
	go func() {
		defer s.waitGroup.Done()
		if err := httpServer.ListenAndServe(); err != http.ErrServerClosed {
			log.Fatalf("ListenAndServe for WebSocket failed: %v", err)
		}
	}()
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
			// We only care about IPv4 for simplicity
			ip = ip.To4()
			if ip != nil {
				log.Printf("  -> %s:%d", ip.String(), port)
			}
		}
	}
}
