// ai-gateway 轻量 AI LLM API 网关
//
// 纯标准库实现，零第三方依赖：
//   - OpenAI 兼容 /v1/chat/completions（含 SSE 流式透传）
//   - 多 provider 聚合与 fallback 链
//   - 统一 Bearer 鉴权
//   - JSONL 轻量记账
//
// 交叉编译（目标：IPQ60xx aarch64）：
//   GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o ai-gateway .
package main

import (
	"flag"
	"log"
	"net/http"
	"os"
)

func main() {
	cfgPath := flag.String("config", "/etc/ai-gateway/config.json", "配置文件路径")
	flag.Parse()

	cfg, err := loadConfig(*cfgPath)
	if err != nil {
		log.Fatalf("[ai-gateway] %v", err)
	}

	g := &Gateway{cfg: cfg}

	if cfg.LogFile != "" {
		f, err := os.OpenFile(cfg.LogFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			log.Fatalf("[ai-gateway] 打开日志文件失败: %v", err)
		}
		g.logOut = f
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/v1/chat/completions", g.handleChat)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte("ok\n"))
	})

	log.Printf("[ai-gateway] listening on %s", cfg.Listen)
	if err := http.ListenAndServe(cfg.Listen, mux); err != nil {
		log.Fatalf("[ai-gateway] %v", err)
	}
}
