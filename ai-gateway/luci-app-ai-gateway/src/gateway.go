// Package main implements a lightweight multi-provider AI LLM API gateway.
//
// Pure Go standard library, zero third-party dependencies.
// OpenAI-compatible /v1/chat/completions, /v1/models, /health endpoints.
//
// Supported built-in platforms:
//   - OpenAI-compatible: Groq, Cerebras, NVIDIA NIM, Mistral, OpenRouter,
//     Ollama Cloud, Kilo Gateway, Cloudflare, Cohere, ModelScope,

//     HuggingFace Router, Zhipu (Z.ai), Radeon Cloud, Volcengine Ark,
//     Baidu Qianfan, LongCat, iFlytek Spark, OpenCode Zen, OVHcloud,
//     Agnes AI, Reka, SiliconFlow, Routeway, BazaarLink, AINative Studio,
//     Aion Labs, Requesty, NavyAI, NaraRouter, SEA-LION, OrcaRouter,
//     UnoRouter, xKiro, LLM7, AnyAPI

//   - Google: Gemini API (v1beta)
//
// Cross-compile for IPQ60xx aarch64:
//   GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o ai-gateway .

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ──────────────────────────────────────────────
// Config types
// ──────────────────────────────────────────────

// Provider declares one upstream LLM service.
type Provider struct {
	Name    string   `json:"name"`
	BaseURL string   `json:"base_url"`
	APIKey  string   `json:"api_key"`
	Models  []string `json:"models"`

	// Platform identifier for built-in providers (empty = custom):
	// google, groq, cerebras, nvidia, mistral, openrouter, ollama, kilo,
	// cloudflare, cohere, modelscope, huggingface, zhipu, radeon, volcengine,
	// qianfan, longcat, xfyun, opencode, ovh, agnes, reka, siliconflow,
	// routeway, bazaarlink, ainative, aion, requesty, navy, nara, sealion,
	// orcarouter, unorouter, xkiro, llm7, anyapi
	Platform string `json:"platform,omitempty"`

	// Per-provider overrides
	TimeoutMs       int               `json:"timeout_ms,omitempty"`
	ExtraHeaders    map[string]string `json:"extra_headers,omitempty"`
	ForceSingleTool bool              `json:"force_single_tool,omitempty"`
	Keyless         bool              `json:"keyless,omitempty"`
}

// Config is the top-level gateway configuration.
type Config struct {
	Listen     string     `json:"listen"`
	GatewayKey string     `json:"gateway_key"`
	LogFile    string     `json:"log_file"`
	Providers  []Provider `json:"providers"`
}

// ──────────────────────────────────────────────
// OpenAI-compatible request / response types
// ──────────────────────────────────────────────

// ChatMessage represents a single chat message.
type ChatMessage struct {
	Role      string     `json:"role"`
	Content   any        `json:"content,omitempty"`
	Name      string     `json:"name,omitempty"`
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string    `json:"tool_call_id,omitempty"`
}

// ToolCall represents a function call from the model.
type ToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function FunctionCall `json:"function"`
}

// FunctionCall wraps a function name and arguments.
type FunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// ToolDefinition describes a tool/function available to the model.
type ToolDefinition struct {
	Type     string             `json:"type"`
	Function FunctionDefinition `json:"function"`
}

// FunctionDefinition wraps a JSON Schema for a function.
type FunctionDefinition struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Parameters  any    `json:"parameters,omitempty"`
}

// ChatCompletionRequest is the incoming request body.
type ChatCompletionRequest struct {
	Model            string           `json:"model"`
	Messages         []ChatMessage    `json:"messages"`
	Stream           bool             `json:"stream,omitempty"`
	Temperature      *float64         `json:"temperature,omitempty"`
	TopP             *float64         `json:"top_p,omitempty"`
	MaxTokens        *int             `json:"max_tokens,omitempty"`
	Stop             any              `json:"stop,omitempty"`
	Tools            []ToolDefinition `json:"tools,omitempty"`
	ToolChoice       any              `json:"tool_choice,omitempty"`
	ResponseFormat   *ResponseFormat  `json:"response_format,omitempty"`
	ReasoningEffort  string           `json:"reasoning_effort,omitempty"`
	ParallelToolCalls *bool           `json:"parallel_tool_calls,omitempty"`
	StreamOptions    *StreamOptions   `json:"stream_options,omitempty"`
}

// ResponseFormat controls output mode.
type ResponseFormat struct {
	Type       string      `json:"type"`
	JSONSchema *JSONSchema `json:"json_schema,omitempty"`
}

// JSONSchema wraps a JSON Schema object.
type JSONSchema struct {
	Name   string `json:"name"`
	Schema any    `json:"schema"`
}

// StreamOptions controls streaming behavior.
type StreamOptions struct {
	IncludeUsage bool `json:"include_usage,omitempty"`
}

// ChatCompletionResponse is the non-streaming response.
type ChatCompletionResponse struct {
	ID      string        `json:"id"`
	Object  string        `json:"object"`
	Created int64         `json:"created"`
	Model   string        `json:"model"`
	Choices []ChatChoice  `json:"choices"`
	Usage   *TokenUsage   `json:"usage,omitempty"`
}

// ChatChoice represents a single completion choice.
type ChatChoice struct {
	Index        int         `json:"index"`
	Message      ChatMessage `json:"message"`
	FinishReason string      `json:"finish_reason"`
}

// TokenUsage tracks token consumption.
type TokenUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// ChatCompletionChunk represents one SSE chunk.
type ChatCompletionChunk struct {
	ID      string       `json:"id"`
	Object  string       `json:"object"`
	Created int64        `json:"created"`
	Model   string       `json:"model"`
	Choices []ChunkChoice `json:"choices"`
}

// ChunkChoice represents a single delta choice.
type ChunkChoice struct {
	Index        int         `json:"index"`
	Delta        ChatDelta   `json:"delta"`
	FinishReason *string     `json:"finish_reason"`
}

// ChatDelta represents content delta in SSE streaming.
type ChatDelta struct {
	Role      string     `json:"role,omitempty"`
	Content   string     `json:"content,omitempty"`
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
}

// ──────────────────────────────────────────────
// Model catalog types
// ──────────────────────────────────────────────

// ModelInfo describes a single model.
type ModelInfo struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	OwnedBy string `json:"owned_by"`
}

// ModelListResponse is the /v1/models response.
type ModelListResponse struct {
	Object string      `json:"object"`
	Data   []ModelInfo `json:"data"`
}

// ──────────────────────────────────────────────
// Health status types
// ──────────────────────────────────────────────

// ProviderHealth reports health status for one provider.
type ProviderHealth struct {
	Name      string `json:"name"`
	Platform  string `json:"platform"`
	Status    string `json:"status"`
	LastCheck string `json:"last_check,omitempty"`
	Error     string `json:"error,omitempty"`
}

// HealthResponse is the /health response.
type HealthResponse struct {
	Status    string           `json:"status"`
	Uptime    string           `json:"uptime"`
	Version   string           `json:"version"`
	Models    int              `json:"models"`
	Providers []ProviderHealth `json:"providers"`
}

// ──────────────────────────────────────────────
// Request logging
// ──────────────────────────────────────────────

// ReqLog is one line in the JSONL log.
type ReqLog struct {
	Time      string `json:"time"`
	Model     string `json:"model"`
	Provider  string `json:"provider"`
	OK        bool   `json:"ok"`
	Error     string `json:"error,omitempty"`
	LatencyMs int64  `json:"latency_ms,omitempty"`
}

// ──────────────────────────────────────────────
// Gemini types
// ──────────────────────────────────────────────

// GeminiPart represents a part of Gemini message content.
type GeminiPart struct {
	Text         string                    `json:"text,omitempty"`
	Thought      bool                      `json:"thought,omitempty"`
	InlineData   *GeminiInlineData         `json:"inlineData,omitempty"`
	FunctionCall *GeminiFunctionCall       `json:"functionCall,omitempty"`
	FunctionResponse *GeminiFunctionResponse `json:"functionResponse,omitempty"`
}

// GeminiInlineData wraps base64-encoded data.
type GeminiInlineData struct {
	MimeType string `json:"mimeType"`
	Data     string `json:"data"`
}

// GeminiFunctionCall represents a function call in Gemini format.
type GeminiFunctionCall struct {
	ID   string `json:"id,omitempty"`
	Name string `json:"name"`
	Args any    `json:"args"`
}

// GeminiFunctionResponse represents a function response in Gemini format.
type GeminiFunctionResponse struct {
	ID       string `json:"id,omitempty"`
	Name     string `json:"name"`
	Response any    `json:"response"`
}

// GeminiContent is a message in Gemini format.
type GeminiContent struct {
	Role  string       `json:"role"`
	Parts []GeminiPart `json:"parts"`
}

// GeminiRequest is the body for Gemini API.
type GeminiRequest struct {
	Contents          []GeminiContent  `json:"contents"`
	SystemInstruction *GeminiContent   `json:"systemInstruction,omitempty"`
	GenerationConfig  *GeminiGenConfig `json:"generationConfig,omitempty"`
	Tools             any              `json:"tools,omitempty"`
}

// GeminiGenConfig wraps generation parameters.
type GeminiGenConfig struct {
	Temperature     *float64 `json:"temperature,omitempty"`
	MaxOutputTokens *int     `json:"maxOutputTokens,omitempty"`
	TopP            *float64 `json:"topP,omitempty"`
	StopSequences   []string `json:"stopSequences,omitempty"`
}

// GeminiResponse is the Gemini API response.
type GeminiResponse struct {
	Candidates    []GeminiCandidate `json:"candidates"`
	UsageMetadata *GeminiUsage      `json:"usageMetadata,omitempty"`
}

// GeminiCandidate represents one completion candidate.
type GeminiCandidate struct {
	Content      *GeminiContent `json:"content"`
	FinishReason string         `json:"finishReason"`
}

// GeminiUsage tracks token usage.
type GeminiUsage struct {
	PromptTokenCount     int `json:"promptTokenCount"`
	CandidatesTokenCount int `json:"candidatesTokenCount"`
	TotalTokenCount      int `json:"totalTokenCount"`
}

// ──────────────────────────────────────────────
// Error types
// ──────────────────────────────────────────────

// APIError represents a structured API error.
type APIError struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Code    string `json:"code,omitempty"`
}

// ──────────────────────────────────────────────
// Global state
// ──────────────────────────────────────────────

var (
	buildVersion = "0.3.0"
	startTime    = time.Now()
)

// ──────────────────────────────────────────────
// Config loading
// ──────────────────────────────────────────────

// loadConfig reads and validates config from a JSON file.
func loadConfig(path string) (*Config, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	cfg := &Config{Listen: ":3001"}
	if err := json.Unmarshal(raw, cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	if len(cfg.Providers) == 0 {
		return nil, fmt.Errorf("at least one provider required")
	}
	for i := range cfg.Providers {
		applyBuiltinDefaults(&cfg.Providers[i])
	}
	return cfg, nil
}

// ──────────────────────────────────────────────
// Built-in provider registry
// ──────────────────────────────────────────────

// builtinDefaults holds known platform presets.
var builtinDefaults = map[string]Provider{
	"groq":        {Name: "Groq", BaseURL: "https://api.groq.com/openai/v1", Platform: "groq"},
	"cerebras":    {Name: "Cerebras", BaseURL: "https://api.cerebras.ai/v1", Platform: "cerebras"},
	"nvidia":      {Name: "NVIDIA NIM", BaseURL: "https://integrate.api.nvidia.com/v1", Platform: "nvidia", ForceSingleTool: true, TimeoutMs: 180000},
	"mistral":     {Name: "Mistral", BaseURL: "https://api.mistral.ai/v1", Platform: "mistral"},
	"openrouter":  {Name: "OpenRouter", BaseURL: "https://openrouter.ai/api/v1", Platform: "openrouter", ExtraHeaders: map[string]string{"HTTP-Referer": "http://localhost:3001", "X-Title": "AI-Gateway"}},
	"ollama":      {Name: "Ollama Cloud", BaseURL: "https://ollama.com/v1", Platform: "ollama", TimeoutMs: 120000},
	"kilo":        {Name: "Kilo Gateway", BaseURL: "https://api.kilo.ai/api/gateway/v1", Platform: "kilo", Keyless: true},
	"cloudflare":  {Name: "Cloudflare Workers AI", BaseURL: "https://api.cloudflare.com/client/v4/accounts/PLACEHOLDER/ai/v1", Platform: "cloudflare"},
	"cohere":      {Name: "Cohere", BaseURL: "https://api.cohere.com/v2", Platform: "cohere"},
	"modelscope":  {Name: "ModelScope", BaseURL: "https://api-inference.modelscope.cn/v1", Platform: "modelscope"},
	"huggingface": {Name: "HuggingFace Router", BaseURL: "https://router.huggingface.co/v1", Platform: "huggingface"},
	"zhipu":       {Name: "Z.ai", BaseURL: "https://open.bigmodel.cn/api/paas/v4", Platform: "zhipu", TimeoutMs: 60000},
	"radeon":      {Name: "AMD Radeon Cloud", BaseURL: "https://developer.amd.com.cn/radeon/api/v1", Platform: "radeon", ForceSingleTool: true, TimeoutMs: 600000},
	"volcengine":  {Name: "Volcengine Ark", BaseURL: "https://ark.cn-beijing.volces.com/api/v3", Platform: "volcengine"},
	"qianfan":     {Name: "Baidu Qianfan", BaseURL: "https://qianfan.baidubce.com/v2", Platform: "qianfan"},
	"longcat":     {Name: "LongCat", BaseURL: "https://api.longcat.chat/openai/v1", Platform: "longcat"},
	"xfyun":       {Name: "iFlytek Spark", BaseURL: "https://spark-api-open.xf-yun.com/v1", Platform: "xfyun"},
	"opencode":    {Name: "OpenCode Zen", BaseURL: "https://opencode.ai/zen/v1", Platform: "opencode"},
	"ovh":         {Name: "OVH AI Endpoints", BaseURL: "https://oai.endpoints.kepler.ai.cloud.ovh.net/v1", Platform: "ovh", Keyless: true},
	"agnes":       {Name: "Agnes AI", BaseURL: "https://apihub.agnes-ai.com/v1", Platform: "agnes", TimeoutMs: 60000},
	"reka":        {Name: "Reka", BaseURL: "https://api.reka.ai/v1", Platform: "reka"},
	"siliconflow": {Name: "SiliconFlow", BaseURL: "https://api.siliconflow.com/v1", Platform: "siliconflow"},
	"routeway":    {Name: "Routeway", BaseURL: "https://api.routeway.ai/v1", Platform: "routeway", ExtraHeaders: map[string]string{"User-Agent": "Mozilla/5.0 AI-Gateway/1.0"}},
	"bazaarlink":  {Name: "BazaarLink", BaseURL: "https://bazaarlink.ai/api/v1", Platform: "bazaarlink"},
	"ainative":    {Name: "AINative Studio", BaseURL: "https://api.ainative.studio/api/v1", Platform: "ainative"},
	"aion":        {Name: "Aion Labs", BaseURL: "https://api.aionlabs.ai/v1", Platform: "aion"},
	"requesty":    {Name: "Requesty", BaseURL: "https://router.requesty.ai/v1", Platform: "requesty"},
	"navy":        {Name: "NavyAI", BaseURL: "https://api.navy/v1", Platform: "navy", ExtraHeaders: map[string]string{"User-Agent": "AI-Gateway/1.0"}},
	"nara":        {Name: "NaraRouter", BaseURL: "https://router.bynara.id/v1", Platform: "nara"},
	"sealion":     {Name: "SEA-LION", BaseURL: "https://api.sea-lion.ai/v1", Platform: "sealion"},
	"orcarouter":  {Name: "OrcaRouter", BaseURL: "https://api.orcarouter.ai/v1", Platform: "orcarouter"},
	"unorouter":   {Name: "UnoRouter", BaseURL: "https://api.unorouter.com/v1", Platform: "unorouter"},
	"xkiro":       {Name: "xKiro", BaseURL: "https://api.xkiro.com/v1", Platform: "xkiro"},
	"llm7":        {Name: "LLM7", BaseURL: "https://api.llm7.io/v1", Platform: "llm7"},
	"anyapi":      {Name: "AnyAPI", BaseURL: "https://api.anyapi.ai/v1", Platform: "anyapi"},
	"google":      {Name: "Google AI Studio", BaseURL: "https://generativelanguage.googleapis.com/v1beta", Platform: "google"},
}

// applyBuiltinDefaults fills in preset values when platform is recognized.
func applyBuiltinDefaults(p *Provider) {
	if p.Platform == "" {
		p.Platform = "custom"
		return
	}
	def, ok := builtinDefaults[p.Platform]
	if !ok {
		return
	}
	if p.Name == "" {
		p.Name = def.Name
	}
	if p.BaseURL == "" {
		p.BaseURL = def.BaseURL
	}
	if p.TimeoutMs == 0 {
		p.TimeoutMs = def.TimeoutMs
	}
	if p.ExtraHeaders == nil && def.ExtraHeaders != nil {
		p.ExtraHeaders = map[string]string{}
		for k, v := range def.ExtraHeaders {
			p.ExtraHeaders[k] = v
		}
	}
	if !p.ForceSingleTool && def.ForceSingleTool {
		p.ForceSingleTool = true
	}
	if !p.Keyless && def.Keyless {
		p.Keyless = true
	}
}

// ──────────────────────────────────────────────
// Gateway
// ──────────────────────────────────────────────

// Gateway holds runtime state.
type Gateway struct {
	cfg    *Config
	logOut io.Writer
	mu     sync.RWMutex
	health map[string]*ProviderHealth
}

// ──────────────────────────────────────────────
// Provider interface
// ──────────────────────────────────────────────

// ProviderImpl abstracts upstream API interactions.
type ProviderImpl interface {
	ChatCompletion(p *Provider, body []byte, req *ChatCompletionRequest) (*ChatCompletionResponse, error)
	StreamChatCompletion(p *Provider, body []byte, req *ChatCompletionRequest) (<-chan SSEChunk, error)
	ValidateKey(p *Provider) (bool, error)
	ListModels(p *Provider) ([]ModelInfo, error)
}

// SSEChunk is one SSE event.
type SSEChunk struct {
	Data string
	Err  error
}

// ──────────────────────────────────────────────
// OpenAI-compatible provider
// ──────────────────────────────────────────────

// openAIProvider implements ProviderImpl for standard OpenAI-compatible APIs.
type openAIProvider struct{}

func (op *openAIProvider) ChatCompletion(p *Provider, body []byte, req *ChatCompletionRequest) (*ChatCompletionResponse, error) {
	url := strings.TrimSuffix(p.BaseURL, "/") + "/chat/completions"
	httpReq, err := op.buildRequest(p, url, body)
	if err != nil {
		return nil, err
	}

	client := &http.Client{Timeout: op.timeout(p)}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("http: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	if resp.StatusCode >= 500 {
		return nil, fmt.Errorf("upstream %d: %s", resp.StatusCode, string(respBody[:min(len(respBody), 500)]))
	}
	if resp.StatusCode >= 400 {
		return nil, &httpError{status: resp.StatusCode, body: respBody, msg: fmt.Sprintf("HTTP %d", resp.StatusCode)}
	}

	var result ChatCompletionResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}
	return &result, nil
}

func (op *openAIProvider) StreamChatCompletion(p *Provider, body []byte, req *ChatCompletionRequest) (<-chan SSEChunk, error) {
	url := strings.TrimSuffix(p.BaseURL, "/") + "/chat/completions"
	var m map[string]any
	if err := json.Unmarshal(body, &m); err != nil {
		return nil, fmt.Errorf("parse body: %w", err)
	}
	m["stream"] = true
	body2, err := json.Marshal(m)
	if err != nil {
		return nil, err
	}

	httpReq, err := op.buildRequest(p, url, body2)
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Accept", "text/event-stream")

	client := &http.Client{Timeout: time.Duration(op.timeout(p)) * time.Millisecond}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("http: %w", err)
	}
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("upstream %d: %s", resp.StatusCode, string(body[:min(len(body), 200)]))
	}

	ch := make(chan SSEChunk, 64)
	go op.readSSE(resp, ch)
	return ch, nil
}

func (op *openAIProvider) buildRequest(p *Provider, url string, body []byte) (*http.Request, error) {
	req, err := http.NewRequest(http.MethodPost, url, io.NopCloser(strings.NewReader(string(body))))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if !p.Keyless {
		req.Header.Set("Authorization", "Bearer "+p.APIKey)
	}
	for k, v := range p.ExtraHeaders {
		req.Header.Set(k, v)
	}
	return req, nil
}

func (op *openAIProvider) timeout(p *Provider) time.Duration {
	if p.TimeoutMs > 0 {
		return time.Duration(p.TimeoutMs) * time.Millisecond
	}
	return 30 * time.Second
}

func (op *openAIProvider) readSSE(resp *http.Response, ch chan<- SSEChunk) {
	defer resp.Body.Close()
	defer close(ch)

	buf := make([]byte, 4096)
	var acc string
	for {
		n, err := resp.Body.Read(buf)
		if n > 0 {
			acc += string(buf[:n])
			for {
				idx := strings.Index(acc, "\n\n")
				if idx < 0 {
					break
				}
				line := acc[:idx]
				acc = acc[idx+2:]
				data := extractSSEData(line)
				if data != "" {
					ch <- SSEChunk{Data: data}
				}
			}
		}
		if err != nil {
			if err != io.EOF {
				ch <- SSEChunk{Err: err}
			}
			return
		}
	}
}

func (op *openAIProvider) ValidateKey(p *Provider) (bool, error) {
	url := strings.TrimSuffix(p.BaseURL, "/") + "/models"
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return false, err
	}
	if !p.Keyless {
		req.Header.Set("Authorization", "Bearer "+p.APIKey)
	}
	for k, v := range p.ExtraHeaders {
		req.Header.Set(k, v)
	}

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	return resp.StatusCode < 400, nil
}

func (op *openAIProvider) ListModels(p *Provider) ([]ModelInfo, error) {
	url := strings.TrimSuffix(p.BaseURL, "/") + "/models"
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	if !p.Keyless {
		req.Header.Set("Authorization", "Bearer "+p.APIKey)
	}
	for k, v := range p.ExtraHeaders {
		req.Header.Set(k, v)
	}

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	var result ModelListResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("parse models: %w", err)
	}
	return result.Data, nil
}

// ──────────────────────────────────────────────
// Google Gemini provider
// ──────────────────────────────────────────────

// googleProvider implements ProviderImpl for Google Gemini.
type googleProvider struct{}

func (gp *googleProvider) ChatCompletion(p *Provider, body []byte, req *ChatCompletionRequest) (*ChatCompletionResponse, error) {
	gemReq, err := gp.toGeminiRequest(req)
	if err != nil {
		return nil, err
	}

	modelID := gp.stripPrefix(req.Model, "models/")
	url := fmt.Sprintf("%s/models/%s:generateContent", strings.TrimSuffix(p.BaseURL, "/"), modelID)

	reqBody, _ := json.Marshal(gemReq)
	httpReq, err := http.NewRequest(http.MethodPost, url, io.NopCloser(strings.NewReader(string(reqBody))))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-goog-api-key", p.APIKey)

	client := &http.Client{Timeout: gp.timeout(p)}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("http: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("Gemini %d: %s", resp.StatusCode, string(respBody[:min(len(respBody), 300)]))
	}

	var gemResp GeminiResponse
	if err := json.Unmarshal(respBody, &gemResp); err != nil {
		return nil, fmt.Errorf("parse Gemini response: %w", err)
	}

	return gp.fromGeminiResponse(modelID, &gemResp), nil
}

func (gp *googleProvider) StreamChatCompletion(p *Provider, body []byte, req *ChatCompletionRequest) (<-chan SSEChunk, error) {
	gemReq, err := gp.toGeminiRequest(req)
	if err != nil {
		return nil, err
	}

	modelID := gp.stripPrefix(req.Model, "models/")
	url := fmt.Sprintf("%s/models/%s:streamGenerateContent?alt=sse", strings.TrimSuffix(p.BaseURL, "/"), modelID)

	reqBody, _ := json.Marshal(gemReq)
	httpReq, err := http.NewRequest(http.MethodPost, url, io.NopCloser(strings.NewReader(string(reqBody))))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-goog-api-key", p.APIKey)

	client := &http.Client{Timeout: time.Duration(gp.timeout(p)) * time.Millisecond}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("http: %w", err)
	}
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("Gemini %d: %s", resp.StatusCode, string(body[:min(len(body), 200)]))
	}

	ch := make(chan SSEChunk, 64)
	go func() {
		defer resp.Body.Close()
		defer close(ch)
		buf := make([]byte, 4096)
		var acc, id string
		idx := 0
		for {
			n, err := resp.Body.Read(buf)
			if n > 0 {
				acc += string(buf[:n])
				for {
					lineIdx := strings.Index(acc, "\n\n")
					if lineIdx < 0 {
						break
					}
					line := acc[:lineIdx]
					acc = acc[lineIdx+2:]

					data := extractSSEData(line)
					if data == "" || data == "[DONE]" {
						continue
					}

					var gemChunk GeminiResponse
					if err := json.Unmarshal([]byte(data), &gemChunk); err != nil {
						continue
					}
					candidate := firstGeminiCandidate(&gemChunk)
					if candidate == nil {
						continue
					}
					parts := candidate.Content.Parts
					text := geminiExtractText(parts)
					toolCalls := geminiExtractToolCalls(parts, idx)
					idx += len(toolCalls)

					if text == "" && len(toolCalls) == 0 && candidate.FinishReason == "" {
						continue
					}

					if id == "" {
						id = fmt.Sprintf("chatcmpl-%d", time.Now().UnixNano())
					}

					chunk := ChatCompletionChunk{
						ID:      id,
						Object:  "chat.completion.chunk",
						Created: time.Now().Unix(),
						Model:   modelID,
						Choices: []ChunkChoice{{
							Index: 0,
							Delta: ChatDelta{
								Content:   text,
								ToolCalls: toolCalls,
							},
						}},
					}
					if candidate.FinishReason != "" {
						fr := mapGeminiFinishReason(candidate.FinishReason, len(toolCalls) > 0)
						chunk.Choices[0].FinishReason = &fr
					}
					chunkJSON, _ := json.Marshal(chunk)
					ch <- SSEChunk{Data: string(chunkJSON)}
				}
			}
			if err != nil {
				if err != io.EOF {
					ch <- SSEChunk{Err: err}
				}
				return
			}
		}
	}()
	return ch, nil
}

func (gp *googleProvider) ValidateKey(p *Provider) (bool, error) {
	url := strings.TrimSuffix(p.BaseURL, "/") + "/models"
	req, _ := http.NewRequest(http.MethodGet, url, nil)
	req.Header.Set("x-goog-api-key", p.APIKey)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return false, nil
	}
	body, _ := io.ReadAll(resp.Body)
	var v any
	return json.Unmarshal(body, &v) == nil, nil
}

func (gp *googleProvider) ListModels(p *Provider) ([]ModelInfo, error) {
	url := strings.TrimSuffix(p.BaseURL, "/") + "/models"
	req, _ := http.NewRequest(http.MethodGet, url, nil)
	req.Header.Set("x-goog-api-key", p.APIKey)

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	var result struct {
		Models []struct {
			Name        string `json:"name"`
			DisplayName string `json:"displayName"`
		} `json:"models"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("parse: %w", err)
	}
	var out []ModelInfo
	for _, m := range result.Models {
		id := m.Name
		if strings.Contains(id, "/") {
			id = id[strings.LastIndex(id, "/")+1:]
		}
		out = append(out, ModelInfo{ID: id, Object: "model", Created: 0, OwnedBy: "google"})
	}
	return out, nil
}

func (gp *googleProvider) timeout(p *Provider) time.Duration {
	if p.TimeoutMs > 0 {
		return time.Duration(p.TimeoutMs) * time.Millisecond
	}
	return 60 * time.Second
}

func (gp *googleProvider) stripPrefix(s, prefix string) string {
	if strings.HasPrefix(s, prefix) {
		return s[len(prefix):]
	}
	return s
}

// Gemini format conversion helpers

func (gp *googleProvider) toGeminiRequest(req *ChatCompletionRequest) (*GeminiRequest, error) {
	contents, sysParts := convertMessagesToGemini(req.Messages)

	gr := &GeminiRequest{
		Contents: contents,
		GenerationConfig: &GeminiGenConfig{
			Temperature:     req.Temperature,
			TopP:            req.TopP,
			MaxOutputTokens: req.MaxTokens,
		},
	}
	if len(sysParts) > 0 {
		gr.SystemInstruction = &GeminiContent{Role: "user", Parts: sysParts}
	}
	if len(req.Tools) > 0 {
		var funcDecls []map[string]any
		for _, t := range req.Tools {
			funcDecls = append(funcDecls, map[string]any{
				"name":        t.Function.Name,
				"description": t.Function.Description,
				"parameters":  t.Function.Parameters,
			})
		}
		gr.Tools = []map[string]any{{"functionDeclarations": funcDecls}}
	}
	return gr, nil
}

func convertMessagesToGemini(msgs []ChatMessage) (contents []GeminiContent, sysParts []GeminiPart) {
	for _, msg := range msgs {
		if msg.Role == "system" {
			text := contentAsString(msg.Content)
			if text != "" {
				sysParts = append(sysParts, GeminiPart{Text: text})
			}
			continue
		}
		if msg.Role == "assistant" {
			var parts []GeminiPart
			text := contentAsString(msg.Content)
			if text != "" {
				parts = append(parts, GeminiPart{Text: text})
			}
			for _, tc := range msg.ToolCalls {
				parts = append(parts, GeminiPart{
					FunctionCall: &GeminiFunctionCall{
						ID:   tc.ID,
						Name: tc.Function.Name,
						Args: parseJSONString(tc.Function.Arguments),
					},
				})
			}
			if len(parts) > 0 {
				contents = append(contents, GeminiContent{Role: "model", Parts: parts})
			}
			continue
		}
		if msg.Role == "tool" {
			var resp any
			if err := json.Unmarshal([]byte(contentAsString(msg.Content)), &resp); err != nil {
				resp = map[string]any{"value": contentAsString(msg.Content)}
			}
			contents = append(contents, GeminiContent{
				Role: "user",
				Parts: []GeminiPart{{
					FunctionResponse: &GeminiFunctionResponse{
						ID:       msg.ToolCallID,
						Name:     msg.Name,
						Response: resp,
					},
				}},
			})
			continue
		}
		parts := []GeminiPart{{Text: contentAsString(msg.Content)}}
		contents = append(contents, GeminiContent{Role: "user", Parts: parts})
	}
	return
}

func (gp *googleProvider) fromGeminiResponse(modelID string, gr *GeminiResponse) *ChatCompletionResponse {
	candidate := firstGeminiCandidate(gr)
	if candidate == nil {
		return &ChatCompletionResponse{
			ID:      fmt.Sprintf("chatcmpl-%d", time.Now().UnixNano()),
			Object:  "chat.completion",
			Created: time.Now().Unix(),
			Model:   modelID,
			Choices: []ChatChoice{{Index: 0, Message: ChatMessage{Role: "assistant", Content: ""}, FinishReason: "stop"}},
		}
	}

	parts := candidate.Content.Parts
	text := geminiExtractText(parts)
	toolCalls := geminiExtractToolCalls(parts, 0)
	fr := mapGeminiFinishReason(candidate.FinishReason, len(toolCalls) > 0)

	usage := &TokenUsage{}
	if gr.UsageMetadata != nil {
		usage.PromptTokens = gr.UsageMetadata.PromptTokenCount
		usage.CompletionTokens = gr.UsageMetadata.CandidatesTokenCount
		usage.TotalTokens = gr.UsageMetadata.TotalTokenCount
	}

	msg := ChatMessage{Role: "assistant", Content: text}
	if len(toolCalls) > 0 {
		msg.Content = nil
		msg.ToolCalls = toolCalls
	}

	return &ChatCompletionResponse{
		ID:      fmt.Sprintf("chatcmpl-%d", time.Now().UnixNano()),
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   modelID,
		Choices: []ChatChoice{{Index: 0, Message: msg, FinishReason: fr}},
		Usage:   usage,
	}
}

func firstGeminiCandidate(gr *GeminiResponse) *GeminiCandidate {
	if len(gr.Candidates) == 0 {
		return nil
	}
	return &gr.Candidates[0]
}

func geminiExtractText(parts []GeminiPart) string {
	var texts []string
	for _, p := range parts {
		if !p.Thought && p.Text != "" {
			texts = append(texts, p.Text)
		}
	}
	return strings.Join(texts, "")
}

func geminiExtractToolCalls(parts []GeminiPart, startIdx int) []ToolCall {
	var calls []ToolCall
	for _, p := range parts {
		if p.FunctionCall == nil || p.FunctionCall.Name == "" {
			continue
		}
		args := "{}"
		if p.FunctionCall.Args != nil {
			b, _ := json.Marshal(p.FunctionCall.Args)
			args = string(b)
		}
		id := p.FunctionCall.ID
		if id == "" {
			id = fmt.Sprintf("call_%d", startIdx+len(calls))
		}
		calls = append(calls, ToolCall{
			ID:   id,
			Type: "function",
			Function: FunctionCall{
				Name:      p.FunctionCall.Name,
				Arguments: args,
			},
		})
	}
	return calls
}

func mapGeminiFinishReason(reason string, hasToolCalls bool) string {
	if hasToolCalls {
		return "tool_calls"
	}
	switch reason {
	case "STOP":
		return "stop"
	case "MAX_TOKENS":
		return "length"
	case "SAFETY", "RECITATION", "BLOCKLIST":
		return "content_filter"
	default:
		return "stop"
	}
}

// ──────────────────────────────────────────────
// Provider factory
// ──────────────────────────────────────────────

func getProviderImpl(p *Provider) ProviderImpl {
	if p.Platform == "google" {
		return &googleProvider{}
	}
	return &openAIProvider{}
}

// ──────────────────────────────────────────────
// Utility helpers
// ──────────────────────────────────────────────

func contentAsString(content any) string {
	switch v := content.(type) {
	case string:
		return v
	case nil:
		return ""
	default:
		b, _ := json.Marshal(v)
		return string(b)
	}
}

func extractSSEData(line string) string {
	for _, l := range strings.Split(line, "\n") {
		l = strings.TrimSpace(l)
		if strings.HasPrefix(l, "data:") {
			d := strings.TrimPrefix(l, "data:")
			d = strings.TrimSpace(d)
			if d == "[DONE]" {
				return ""
			}
			return d
		}
	}
	return ""
}

func parseJSONString(s string) any {
	var v any
	if err := json.Unmarshal([]byte(s), &v); err == nil {
		return v
	}
	return map[string]any{"value": s}
}

type httpError struct {
	status int
	body   []byte
	msg    string
}

func (e *httpError) Error() string { return e.msg }

// ──────────────────────────────────────────────
// Main entry point
// ──────────────────────────────────────────────

func main() {
	cfgPath := flag.String("config", "/etc/ai-gateway/config.json", "config file path")
	flag.Parse()

	cfg, err := loadConfig(*cfgPath)
	if err != nil {
		log.Fatalf("[ai-gateway] %v", err)
	}

	gw := &Gateway{
		cfg:    cfg,
		health: map[string]*ProviderHealth{},
	}

	if cfg.LogFile != "" {
		f, err := os.OpenFile(cfg.LogFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			log.Fatalf("[ai-gateway] open log file: %v", err)
		}
		gw.logOut = f
	}

	for i := range cfg.Providers {
		p := &cfg.Providers[i]
		gw.health[p.Name] = &ProviderHealth{
			Name:     p.Name,
			Platform: p.Platform,
			Status:   "unknown",
		}
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/v1/chat/completions", gw.handleChatCompletion)
	mux.HandleFunc("/v1/models", gw.handleModels)
	mux.HandleFunc("/health", gw.handleHealth)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte("ok\n"))
	})

	addr := cfg.Listen
	if addr == "" {
		addr = ":3001"
	}
	log.Printf("[ai-gateway] v%s listening on %s", buildVersion, addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("[ai-gateway] %v", err)
	}
}

// ──────────────────────────────────────────────
// HTTP Handlers
// ──────────────────────────────────────────────

func (gw *Gateway) handleChatCompletion(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, APIError{Message: "method not allowed", Type: "invalid_request_error"})
		return
	}

	if gw.cfg.GatewayKey != "" {
		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "Bearer ") || strings.TrimSpace(strings.TrimPrefix(auth, "Bearer ")) != gw.cfg.GatewayKey {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"error":{"message":"invalid gateway key","type":"auth_error"}}`))
			return
		}
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, APIError{Message: "read body failed", Type: "invalid_request_error"})
		return
	}

	var req ChatCompletionRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, APIError{Message: "invalid JSON body", Type: "invalid_request_error"})
		return
	}
	if req.Model == "" {
		writeJSON(w, http.StatusBadRequest, APIError{Message: "model is required", Type: "invalid_request_error"})
		return
	}

	candidates := gw.providersForModel(req.Model)
	if len(candidates) == 0 {
		writeJSON(w, http.StatusNotFound, APIError{Message: fmt.Sprintf("no provider for model '%s'", req.Model), Type: "invalid_request_error"})
		return
	}

	start := time.Now()
	var lastErr error

	for _, prov := range candidates {
		impl := getProviderImpl(prov)

		if req.Stream {
			if err := gw.handleStreamResponse(w, r, prov, impl, body, &req, start); err != nil {
				lastErr = err
				gw.logRequest(req.Model, prov.Name, false, err.Error(), start)
				log.Printf("[ai-gateway] provider %s stream failed: %v", prov.Name, err)
				continue
			}
			return
		}

		resp, err := impl.ChatCompletion(prov, body, &req)
		if err != nil {
			lastErr = err
			gw.logRequest(req.Model, prov.Name, false, err.Error(), start)
			log.Printf("[ai-gateway] provider %s failed: %v", prov.Name, err)
			continue
		}

		gw.logRequest(req.Model, prov.Name, true, "", start)
		writeJSON(w, http.StatusOK, resp)
		return
	}

	gw.logRequest(req.Model, "", false, "all providers failed", start)
	errMsg := "all providers failed"
	if lastErr != nil {
		errMsg = lastErr.Error()
	}
	writeJSON(w, http.StatusBadGateway, APIError{Message: errMsg, Type: "upstream_error"})
}

func (gw *Gateway) providersForModel(model string) []*Provider {
	var candidates []*Provider
	for i := range gw.cfg.Providers {
		p := &gw.cfg.Providers[i]
		if len(p.Models) == 0 {
			candidates = append(candidates, p)
		} else {
			for _, m := range p.Models {
				if modelMatches(m, model) {
					candidates = append(candidates, p)
					break
				}
			}
		}
	}
	// 白名单严格模式（防扣费）：模型不匹配任何 provider 白名单时直接拒绝，
	// 不再兜底转发到第一个 provider——避免按量付费模型被意外消费。
	return candidates
}

// modelMatches 支持通配符匹配：
//   - "*" 匹配一切
//   - "xxx*" 前缀匹配（如 "glm-4-flash*"）
//   - "*:free" 后缀匹配（OpenRouter 免费模型统一带 :free 后缀，付费模型无此后缀）
//   - 其余按精确字符串匹配
func modelMatches(pattern, model string) bool {
	if pattern == "*" {
		return true
	}
	if strings.HasSuffix(pattern, "*") {
		return strings.HasPrefix(model, strings.TrimSuffix(pattern, "*"))
	}
	if strings.HasPrefix(pattern, "*") {
		return strings.HasSuffix(model, strings.TrimPrefix(pattern, "*"))
	}
	return pattern == model
}

func (gw *Gateway) handleStreamResponse(w http.ResponseWriter, r *http.Request, p *Provider, impl ProviderImpl, body []byte, req *ChatCompletionRequest, start time.Time) error {
	ch, err := impl.StreamChatCompletion(p, body, req)
	if err != nil {
		return err
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return nil
	}
	w.WriteHeader(http.StatusOK)

	first := true
	for chunk := range ch {
		if chunk.Err != nil {
			if first {
				log.Printf("[ai-gateway] stream error from %s: %v", p.Name, chunk.Err)
			}
			return nil
		}
		if chunk.Data != "" {
			fmt.Fprintf(w, "data: %s\n\n", chunk.Data)
			flusher.Flush()
			first = false
		}
	}
	if !first {
		gw.logRequest(req.Model, p.Name, true, "", start)
	} else {
		gw.logRequest(req.Model, p.Name, false, "empty stream", start)
	}
	fmt.Fprintf(w, "data: [DONE]\n\n")
	flusher.Flush()
	return nil
}

func (gw *Gateway) handleModels(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, APIError{Message: "method not allowed"})
		return
	}

	var allModels []ModelInfo
	seen := map[string]bool{}

	for i := range gw.cfg.Providers {
		p := &gw.cfg.Providers[i]
		impl := getProviderImpl(p)

		if len(p.Models) > 0 {
			for _, m := range p.Models {
				if !seen[m] {
					seen[m] = true
					allModels = append(allModels, ModelInfo{ID: m, Object: "model", OwnedBy: p.Name, Created: 0})
				}
			}
			continue
		}

		models, err := impl.ListModels(p)
		if err != nil {
			log.Printf("[ai-gateway] fetch models from %s: %v", p.Name, err)
			continue
		}
		for _, m := range models {
			if !seen[m.ID] {
				seen[m.ID] = true
				allModels = append(allModels, m)
			}
		}
	}

	if len(allModels) == 0 {
		for i := range gw.cfg.Providers {
			p := &gw.cfg.Providers[i]
			for _, m := range p.Models {
				if !seen[m] {
					seen[m] = true
					allModels = append(allModels, ModelInfo{ID: m, Object: "model", OwnedBy: p.Name, Created: 0})
				}
			}
		}
	}

	writeJSON(w, http.StatusOK, ModelListResponse{Object: "list", Data: allModels})
}

// handleHealth returns a lightweight, deadlock-free local status.
// External ValidateKey probes are intentionally NOT called here to avoid
// holding gw.mu while an unbounded upstream HTTP request may stall.
func (gw *Gateway) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		writeJSON(w, http.StatusMethodNotAllowed, APIError{Message: "method not allowed"})
		return
	}
	providers := make([]ProviderHealth, 0, len(gw.cfg.Providers))
	for _, p := range gw.cfg.Providers {
		ph := ProviderHealth{Name: p.Name, Platform: p.Platform}
		if h, ok := gw.health[p.Name]; ok {
			ph.Status = h.Status
			ph.LastCheck = h.LastCheck
			ph.Error = h.Error
		}
		providers = append(providers, ph)
	}
	writeJSON(w, http.StatusOK, HealthResponse{
		Status:    "ok",
		Uptime:    time.Since(startTime).String(),
		Version:   buildVersion,
		Providers: providers,
	})
}

// ──────────────────────────────────────────────
// Logging
// ──────────────────────────────────────────────

func (gw *Gateway) logRequest(model, provider string, ok bool, errMsg string, start time.Time) {
	lat := time.Since(start).Milliseconds()
	rec := ReqLog{
		Time:      time.Now().Format(time.RFC3339),
		Model:     model,
		Provider:  provider,
		OK:        ok,
		Error:     errMsg,
		LatencyMs: lat,
	}
	b, _ := json.Marshal(rec)
	out := gw.logOut
	if out == nil {
		out = log.Writer()
	}
	fmt.Fprintln(out, string(b))
}

// ──────────────────────────────────────────────
// JSON helpers
// ──────────────────────────────────────────────

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ──────────────────────────────────────────────
// Unused helper suppression (for potential use)
// ──────────────────────────────────────────────
var _ = strconv.Itoa // keep import alive for potential future use