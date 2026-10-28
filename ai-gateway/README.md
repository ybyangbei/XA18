# ai-gateway — 轻量 AI LLM API 网关（v0.3.0）

自托管的 OpenAI 兼容 LLM API 网关，专为嵌入式路由器（本项目 CMIOT AX18，aarch64）设计。
**纯 Go 标准库实现，零第三方依赖**，交叉编译无任何原生依赖坑。

对应需求文档：FR-25 / FR-26 / FR-27（第 6.11 节）。

## 功能

### 核心功能

- **OpenAI 兼容**：`POST /v1/chat/completions`（含 `stream=true` SSE 流式透传），客户端零改动接入
- **模型目录**：`GET /v1/models` 聚合所有上游模型
- **健康检查**：`GET /health` 返回所有 provider 状态 + 版本号
- **统一鉴权**：单一 API key（Bearer）校验
- **多 provider 聚合**：配置文件中声明 35+ 内置平台或任意自定义端点
- **fallback 链**：首选 provider 失败时自动切换下一个
- **工具调用（Function Calling）**：支持 tools + tool_choice，含 Google Gemini 格式自动转换
- **结构化输出**：response_format（JSON mode）支持
- **JSONL 轻量记账**：每请求一行，含延迟统计

### 内置平台

`google` (Gemini API v1beta)、`groq`、`cerebras`、`nvidia` (NVIDIA NIM)、`mistral`、`openrouter`、`ollama` (Ollama Cloud)、`kilo` (Kilo Gateway)、`cloudflare` (Cloudflare Workers AI)、`cohere`、`modelscope`、`huggingface` (HuggingFace Router)、`zhipu` (Z.ai)、`radeon` (AMD Radeon Cloud)、`volcengine` (Volcengine Ark)、`qianfan` (Baidu Qianfan)、`longcat` (LongCat)、`xfyun` (iFlytek Spark)、`opencode` (OpenCode Zen)、`ovh` (OVH AI Endpoints)、`agnes` (Agnes AI)、`reka`、`siliconflow`、`routeway`、`bazaarlink`、`ainative`、`aion`、`requesty`、`navy`、`nara`、`sealion`、`orcarouter`、`unorouter`、`xkiro`、`llm7`、`anyapi`

## 构建

Go 源码位于 `luci-app-ai-gateway/src/`（单文件 `gateway.go`），所有构建命令需在该子目录中执行：

本地（x86 调试）：

```sh
cd luci-app-ai-gateway/src
go build -o ai-gateway .
./ai-gateway -config ../../config.example.json
```

交叉编译（IPQ60xx = aarch64）：

```sh
cd luci-app-ai-gateway/src
GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o ai-gateway .
```

## 本地运行

```sh
cd luci-app-ai-gateway/src
./ai-gateway -config ../../config.example.json
curl http://127.0.0.1:3001/healthz   # => ok
```

调用示例（任意 OpenAI 兼容客户端，base_url 指向 `http://<路由器IP>:3001/v1`）：

```sh
curl http://127.0.0.1:3001/v1/chat/completions \
  -H "Authorization: Bearer change-me-to-a-strong-key" \
  -H "Content-Type: application/json" \
  -d '{"model":"groq-llama-3.3-70b-versatile","messages":[{"role":"user","content":"hi"}]}'
```

## 配置

见 `config.example.json`。要点：

| 字段 | 说明 |
| --- | --- |
| `listen` | 监听地址，默认 `:3001` |
| `gateway_key` | 统一 API key；为空则不鉴权（生产环境务必配置） |
| `log_file` | JSONL 记账文件；为空输出到 stdout |
| `providers[].platform` | 内置平台标识（如 `groq`, `google`, `mistral`）或留空自定义 |
| `providers[].base_url` | 上游 OpenAI 兼容根地址（不含 `/chat/completions`） |
| `providers[].api_key` | 该服务商的 API key |
| `providers[].models` | 该服务商可服务的模型列表；为空表示可服务任意模型 |
| `providers[].timeout_ms` | 每请求超时（毫秒），默认 30000 |
| `providers[].extra_headers` | 额外 HTTP 请求头（如 User-Agent） |
| `providers[].keyless` | 是否为无密钥平台（如 Kilo, OVHcloud） |
| `providers[].force_single_tool` | 强制单工具调用（NVIDIA NIM / Radeon Cloud 需要） |

## 路由规则

1. 请求中的 `model` 精确匹配某个 provider 的 `models` 列表 → 使用匹配的第一个
2. `models` 为空的 provider 匹配任意模型（配置顺序优先）
3. 均不匹配时回退到第一个 provider
4. fallback 按同规则取下一个可用 provider

## OpenWrt 打包（LuCI）

`luci-app-ai-gateway/` 为自包含的 OpenWrt 包（Go 源码在 `src/` 子目录）：

```sh
# 在 OpenWrt/LibWrt/LEDE 源码树中
cp -r luci-app-ai-gateway package/
./scripts/feeds install luci-app-ai-gateway   # 如需 feeds 注册
make package/luci-app-ai-gateway/compile
```

包安装后：

- 二进制：`/usr/bin/ai-gateway`
- 配置：`/etc/ai-gateway/config.json`（刷入后按实际 key/provider 修改）
- 服务：`/etc/init.d/ai-gateway enable && /etc/init.d/ai-gateway start`
- LuCI：管理界面 → 服务 → AI Gateway

## 与需求文档的映射

- FR-25：本网关二进制交叉编译并打包进固件
- FR-26：LuCI 管理界面（`luci-app-ai-gateway`）
- FR-27：OpenAI 兼容 /v1、多 provider、fallback、统一鉴权、JSONL 记账
- D-04：上游为被墙服务商时，出站流量按代理分流规则（豁免 ECM）走 OpenClash

## 已知边界 / 待完善

- LuCI 面板当前为基础骨架（服务配置入口 + 状态说明），provider 的动态编辑界面待完善
- 无前端 dashboard（按需求 FR-27 仅要求 /v1 接口）
- 限流（per-key QPS）为后续增强项，未纳入 MVP
- Google Gemini 的图片输入（inlineData）当前仅支持文本格式
