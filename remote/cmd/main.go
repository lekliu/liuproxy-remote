package main

import (
	"flag"
	"liuproxy_remote/remote/types"
	"log"
	"path/filepath"

	"liuproxy_remote/remote/config"
	"liuproxy_remote/remote/server"
)

func main() {
	defaultConfigPath := filepath.Join("remote", "ini", "remote.ini")
	configPath := flag.String("config", defaultConfigPath, "Path to remote config file")
	flag.Parse()

	// 1. 加载配置
	cfg := new(types.Config)
	if err := config.LoadIni(cfg, *configPath); err != nil {
		log.Fatalf("Failed to load config file '%s': %v", *configPath, err)
	}

	// 2. 创建并运行服务器
	appServer := server.New(cfg, *configPath)
	appServer.Run()
}
