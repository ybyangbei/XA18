# AI Gateway LuCI 界面增强版

> 基于 FR-25/27 需求实现的完整 AI Gateway LuCI 管理界面

## 功能特性

### ✅ 已实现功能

1. **服务管理**
   - 启动/停止服务控制
   - 实时状态监控
   - 进程 ID 显示
   - 服务日志查看

2. **Provider 配置**
   - 动态添加/删除 Provider
   - 支持 12+ 预置平台（Groq、Google、OpenAI 等）
   - 自定义平台支持
   - API Key 安全存储
   - 模型列表 JSON 配置

3. **健康监控**
   - 服务可用性检查
   - /health 端点状态
   - /v1/models 模型列表
   - 自动刷新（30秒）

4. **用户界面**
   - 中文 LuCI 界面
   - 响应式设计
   - 实时状态显示
   - 配置验证

### 📋 界面导航

```
admin/services/ai_gateway          # 主配置页面
├── Provider 配置                  # 多 Provider 管理
├── 服务状态                        # 实时状态监控
└── 健康检查                        # 端点健康检查
```

## 配置说明

### 基础配置

```json
{
  "listen": ":3001",
  "gateway_key": "your-secret-key",
  "log_file": "/tmp/ai-gateway.log",
  "enabled": true,
  "providers": [...]
}
```

### Provider 配置示例

```json
{
  "name": "Groq 示例",
  "platform": "groq", 
  "api_key": "sk-xxx",
  "models": ["llama-3.3-70b-versatile"]
}
```

### 支持的平台

| 平台 | 标识符 | 说明 |
|------|--------|------|
| OpenAI 兼容 | `openai` | 标准 OpenAI API |
| Groq | `groq` | Groq 快速推理 |
| Google Gemini | `google` | Google Gemini API |
| OpenRouter | `openrouter` | OpenRouter 市场 |
| Cerebras | `cerebras` | Cerebras AI |
| NVIDIA NIM | `nvidia` | NVIDIA AI 云服务 |
| Mistral | `mistral` | Mistral AI |
| 智谱 AI | `zhipu` | 智谱 AI 服务 |
| 百度千帆 | `baidu` | 百度千帆平台 |
| 月之暗面 | `kimi` | Kimi AI 服务 |
| Ollama | `ollama` | Ollama 本地部署 |
| 自定义 | `custom` | 自定义 API 端点 |

## 使用方法

### 1. 访问管理界面

- 路由器地址: `http://192.168.x.1`
- 路径: `服务 → AI Gateway`

### 2. 配置 Provider

1. 点击"Provider 配置"标签
2. 点击"添加"按钮
3. 填写 Provider 信息：
   - 名称: 便于识别的名称
   - 平台: 选择对应的服务商
   - API Key:服务商提供的密钥
   - 模型列表: JSON 格式的模型数组
4. 点击"保存并应用"

### 3. 查看状态

1. 点击"服务状态"标签
2. 查看运行状态、端口、进程信息
3. 查看实时日志（最后1000字符）
4. 使用启停按钮控制服务

### 4. 健康检查

1. 点击"健康检查"标签
2. 查看服务可用性
3. 查看健康检查详情
4. 查看可用模型列表
5. 点击"测试连接"进行快速诊断

## API 使用

### 基本调用

```bash
# 健康检查
curl http://localhost:3001/health

# 获取模型列表
curl http://localhost:3001/v1/models

# 调用聊天接口
curl -X POST http://localhost:3001/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer your-gateway-key" \
  -d '{
    "model": "groq/llama-3.3-70b-versatile",
    "messages": [{"role": "user", "content": "Hello"}]
  }'
```

### 客户端配置

```python
# Python 客户端示例
import openai

# 使用网关地址和密钥
client = openai.OpenAI(
    base_url="http://192.168.1.1:3001/v1",
    api_key="your-gateway-key"
)

response = client.chat.completions.create(
    model="groq/llama-3.3-70b-versatile",
    messages=[{"role": "user", "content": "Hello"}]
)
```

## 故障排除

### 常见问题

1. **服务无法启动**
   - 检查配置文件格式
   - 验证 API Key 有效性
   - 查看 `/tmp/ai-gateway.log` 日志

2. **Provider 连接失败**
   - 检查网络连接
   - 验证 API Key 和配额
   - 尝试切换其他 Provider

3. **模型不可用**
   - 检查模型名称是否正确
   - 确认 Provider 支持该模型
   - 查看健康检查页面

### 测试脚本

使用提供的测试脚本进行诊断：

```bash
chmod +x test-gateway.sh
./test-gateway.sh
```

## 开发说明

### 文件结构

```
ai-gateway/
├── luci-app-ai-gateway/
│   ├── src/
│   │   ├── gateway.go          # Go 源码
│   │   └── Makefile           # 编译配置
│   ├── files/
│   │   ├── etc/init.d/ai-gateway    # 启动脚本
│   │   ├── usr/lib/lua/luci/controller/ai_gateway.lua  # 控制器
│   │   ├── usr/lib/lua/luci/model/cbi/ai_gateway.lua    # 配置模型
│   │   ├── usr/share/luci/view/ai_gateway/             # 模板文件
│   │   └── usr/share/rpcd/acl.d/                      # 权限配置
│   └── config.example.json    # 配置示例
├── test-gateway.sh            # 测试脚本
└── ENHANCED-README.md         # 本文档
```

### 编译安装

```bash
# 在 OpenWrt 编译环境中
cd package/luci-app-ai-gateway
make clean
make
make install
```

## 版本信息

- **版本**: v0.3.0 (增强版)
- **兼容**: OpenWrt 25.12-nss
- **架构**: arm64 (IPQ60xx)
- **依赖**: 无外部依赖（纯 Go 标准库）

---

*此增强版实现了 FR-25/27 的所有要求，提供完整的 LuCI 管理界面和功能。*