# ai-gateway — 轻量 AI LLM API 网关

自托管的 OpenAI 兼容 LLM API 网关，专为嵌入式路由器（本项目 CMIOT AX18，aarch64）设计。
**纯 Go 标准库实现，零第三方依赖**，交叉编译无任何原生依赖坑。

对应需求文档：FR-25 / FR-26 / FR-27（第 6.11 节）。

## 功能

- OpenAI 兼容 `POST /v1/chat/completions`（含 `stream=true` SSE 流式透传），客户端零改动接入
- 多 provider 聚合：Groq / Google Gemini / OpenRouter / Cerebras 等任意 OpenAI 兼容服务
- fallback 链：首选 provider 连接失败或返回 5xx 时自动切换下一个
- 统一 Bearer 鉴权（单一 gateway key）
- JSONL 轻量记账（每请求一行）
- `GET /healthz` 健康检查

## 构建

本地（x86 调试）：

```sh
go build -o ai-gateway .
```

交叉编译（IPQ60xx = aarch64，本机无 Go 也可在任何装有 Go 的机器 / CI 执行）：

```sh
GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o ai-gateway .
```

## 本地运行

```sh
./ai-gateway -config config.example.json
curl http://127.0.0.1:3001/healthz   # => ok
```

调用示例（任意 OpenAI 兼容客户端，base_url 指向 `http://<路由器IP>:3001/v1`）：

```sh
curl http://127.0.0.1:3001/v1/chat/completions \
  -H "Authorization: Bearer change-me-to-a-strong-key" \
  -H "Content-Type: application/json" \
  -d '{"model":"llama-3.3-70b-versatile","messages":[{"role":"user","content":"hi"}]}'
```

## 配置

见 `config.example.json`。要点：

| 字段 | 说明 |
| --- | --- |
| `listen` | 监听地址，默认 `:3001` |
| `gateway_key` | 统一 API key；为空则不鉴权（生产环境务必配置） |
| `log_file` | JSONL 记账文件；为空输出到 stdout |
| `providers[].base_url` | 上游 OpenAI 兼容根地址（不含 `/chat/completions`） |
| `providers[].models` | 该服务商可服务的模型；留空表示可服务任意模型 |

模型路由规则：请求中的 `model` 匹配某个 provider 的 `models` 列表则用该 provider（按配置顺序），
均不匹配时回退到第一个 provider。fallback 按同规则取下一个可用 provider。

## OpenWrt 打包（LuCI）

`luci-app-ai-gateway/` 为 OpenWrt 包工程：

```sh
# 在 OpenWrt/LibWrt 源码树中
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
