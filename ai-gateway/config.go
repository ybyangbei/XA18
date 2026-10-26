package main

import (
	"encoding/json"
	"fmt"
	"os"
)

// Provider 定义上游 LLM 服务商配置。
type Provider struct {
	Name    string   `json:"name"`     // 服务商名称（用于日志 / fallback 追踪）
	BaseURL string   `json:"base_url"` // 上游 OpenAI 兼容根地址，如 https://api.groq.com/openai/v1
	APIKey  string   `json:"api_key"`  // 该服务商的 API key
	Models  []string `json:"models"`   // 该服务商可服务的模型列表；为空表示可服务任意模型
}

// Config 网关整体配置。
type Config struct {
	Listen     string     `json:"listen"`      // 监听地址，如 ":3001"
	GatewayKey string     `json:"gateway_key"` // 统一 API key；为空则不启用鉴权
	LogFile    string     `json:"log_file"`    // JSONL 记账文件；为空输出到 stdout
	Providers  []Provider `json:"providers"`
}

func loadConfig(path string) (*Config, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取配置失败: %w", err)
	}
	cfg := &Config{Listen: ":3001"}
	if err := json.Unmarshal(raw, cfg); err != nil {
		return nil, fmt.Errorf("解析配置失败: %w", err)
	}
	if len(cfg.Providers) == 0 {
		return nil, fmt.Errorf("配置中至少需要定义一个 provider")
	}
	return cfg, nil
}
