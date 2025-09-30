// --- START OF COMPLETE REPLACEMENT for liuproxy-remote/internal/config/config.go ---
package config

import (
	"os"
	"strconv"

	"gopkg.in/ini.v1"
	"liuproxy_remote/internal/types"
)

// LoadIni 从指定的 fileName 加载配置到 types.Config 结构体中。
// 这个版本被大幅简化，只处理 remote 端需要的配置。
func LoadIni(cfg *types.Config, fileName string) error {
	iniFile, err := ini.Load(fileName)
	if err != nil {
		return err
	}

	// 自动映射 [common] 和 [remote] 节
	if err := iniFile.MapTo(cfg); err != nil {
		return err
	}

	// 优先处理 PaaS 平台注入的 PORT 环境变量
	envPort := os.Getenv("PORT")
	if envPort != "" {
		if intValue, err := strconv.Atoi(envPort); err == nil {
			// 如果 PORT 存在且有效，它将覆盖 .ini 文件中的 port_ws_svr
			cfg.RemoteConf.PortWsSvr = intValue
		}
	}

	// 环境变量覆盖逻辑保持不变
	overrideFromEnvInt(&cfg.CommonConf.Crypt, "CRYPT_KEY")
	// REMOTE_PORT 优先级高于 PORT 定义的端口
	overrideFromEnvInt(&cfg.RemoteConf.PortWsSvr, "REMOTE_PORT")

	return nil
}

// overrideFromEnvInt 是一个私有辅助函数
func overrideFromEnvInt(target *int, envName string) {
	envValue := os.Getenv(envName)
	if envValue != "" {
		if intValue, err := strconv.Atoi(envValue); err == nil {
			*target = intValue
		}
	}
}
