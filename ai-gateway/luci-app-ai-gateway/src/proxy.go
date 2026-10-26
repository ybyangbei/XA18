package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

// Gateway 网关运行时状态。
type Gateway struct {
	cfg    *Config
	logOut io.Writer
}

// chatRequest 仅解析转发所需的 OpenAI 请求字段，其余字段原样透传。
type chatRequest struct {
	Model    string `json:"model"`
	Stream   bool   `json:"stream"`
	Messages []any  `json:"messages"`
}

// providersForModel 按模型返回所有可服务的 provider（保持配置顺序）。
func (c *Config) providersForModel(model string) []*Provider {
	var out []*Provider
	for i := range c.Providers {
		p := &c.Providers[i]
		if len(p.Models) == 0 || contains(p.Models, model) {
			out = append(out, p)
		}
	}
	// 模型未命中任何 provider 时，回退到第一个 provider
	if len(out) == 0 {
		out = append(out, &c.Providers[0])
	}
	return out
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

func (g *Gateway) handleChat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":{"message":"method not allowed"}}`, http.StatusMethodNotAllowed)
		return
	}
	if !g.checkAuth(r) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":{"message":"invalid gateway key","type":"auth_error"}}`))
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, `{"error":{"message":"read body failed"}}`, http.StatusBadRequest)
		return
	}
	var req chatRequest
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, `{"error":{"message":"invalid body","type":"invalid_request"}}`, http.StatusBadRequest)
		return
	}
	if req.Model == "" {
		http.Error(w, `{"error":{"message":"model is required"}}`, http.StatusBadRequest)
		return
	}

	providers := g.cfg.providersForModel(req.Model)
	var lastErr error
	for _, p := range providers {
		err := g.forward(w, r, p, body)
		if err == nil {
			g.logRequest(req.Model, p.Name, true, "")
			return
		}
		lastErr = err
		g.logRequest(req.Model, p.Name, false, err.Error())
		log.Printf("[ai-gateway] provider %s failed: %v", p.Name, err)
	}

	g.logRequest(req.Model, "", false, "all providers failed")
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadGateway)
	fmt.Fprintf(w, `{"error":{"message":"all providers failed: %s","type":"upstream_error"}}`, lastErr)
}

// forward 将请求原样转发到上游。仅在尚未写回任何响应时（连接失败 / 上游 5xx）
// 返回错误，由调用方触发 fallback。
func (g *Gateway) forward(w http.ResponseWriter, src *http.Request, p *Provider, body []byte) error {
	upstream := strings.TrimSuffix(p.BaseURL, "/") + "/chat/completions"
	req, err := http.NewRequestWithContext(src.Context(), http.MethodPost, upstream, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.APIKey)
	if a := src.Header.Get("Accept"); a != "" {
		req.Header.Set("Accept", a)
	}

	client := &http.Client{Timeout: 10 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// 上游业务错误（4xx，如限流/参数错误）直接透传，不触发 fallback
	if resp.StatusCode >= 500 {
		return fmt.Errorf("upstream %s status %d", p.Name, resp.StatusCode)
	}

	for k, vv := range resp.Header {
		for _, v := range vv {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(resp.StatusCode)
	// SSE 流式响应通过 io.Copy 天然逐块透传
	_, err = io.Copy(w, resp.Body)
	return err
}

func (g *Gateway) checkAuth(r *http.Request) bool {
	if g.cfg.GatewayKey == "" {
		return true // 未配置统一 key 时不鉴权（生产环境应配置）
	}
	auth := r.Header.Get("Authorization")
	if !strings.HasPrefix(auth, "Bearer ") {
		return false
	}
	return strings.TrimSpace(strings.TrimPrefix(auth, "Bearer ")) == g.cfg.GatewayKey
}

type reqLog struct {
	Time     string `json:"time"`
	Model    string `json:"model"`
	Provider string `json:"provider"`
	OK       bool   `json:"ok"`
	Error    string `json:"error,omitempty"`
}

func (g *Gateway) logRequest(model, provider string, ok bool, errMsg string) {
	rec := reqLog{
		Time:     time.Now().Format(time.RFC3339),
		Model:    model,
		Provider: provider,
		OK:       ok,
		Error:    errMsg,
	}
	b, err := json.Marshal(rec)
	if err != nil {
		return
	}
	out := g.logOut
	if out == nil {
		out = log.Writer()
	}
	fmt.Fprintln(out, string(b))
}
