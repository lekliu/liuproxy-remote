// --- START OF COMPLETE REPLACEMENT for server.go (REVERTED) ---
package server

import (
	"liuproxy_remote/internal/types"
	"log"
	"sync"
)

// AppServer 是应用的主结构体，持有配置和核心组件
type AppServer struct {
	cfg       *types.Config
	waitGroup sync.WaitGroup
}

// New 创建一个新的 AppServer 实例
func New(cfg *types.Config, configPath string) *AppServer {
	return &AppServer{
		cfg: cfg,
	}
}

// Run 是服务器的启动入口 (用于命令行)
func (s *AppServer) Run() {
	log.Printf("Starting server in '%s' mode...", s.cfg.Mode)
	if s.cfg.Mode == "remote" {
		s.runRemote()
	} else {
		log.Fatalf("This executable only supports 'remote' mode. Found mode: '%s'", s.cfg.Mode)
	}

	s.Wait()
}

// Wait 会阻塞直到所有服务器的 goroutine 都已退出。
func (s *AppServer) Wait() {
	s.waitGroup.Wait()
	log.Println("All server routines have finished.")
}
