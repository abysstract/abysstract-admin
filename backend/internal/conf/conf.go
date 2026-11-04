package conf

import "github.com/abysstract/abysstract-admin/backend/adminconfig"

// Config 是公共配置类型的别名，保持现有内部引用。
type Config = adminconfig.Config

// 运行模式。
const (
	EnvDev  = adminconfig.EnvDev
	EnvProd = adminconfig.EnvProd
)

// Load 从环境变量读取配置，提供合理默认值。
func Load() *Config {
	return adminconfig.Load()
}
