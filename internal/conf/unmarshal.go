package conf

import (
	"os"

	"github.com/pelletier/go-toml/v2"
)

// SetupConfig 从文件读取内容初始化配置
func SetupConfig(v any, path string) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return toml.Unmarshal(b, v)
}

// WriteConfig 将配置写回文件
func WriteConfig(v any, path string) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	return toml.NewEncoder(f).SetIndentTables(true).Encode(v)
}
