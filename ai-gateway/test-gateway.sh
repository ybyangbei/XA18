#!/bin/bash

# AI Gateway 测试脚本
# 用于验证服务状态和基本功能

AI_GATEWAY_URL="http://localhost:3001"
CONFIG_FILE="/etc/ai-gateway/config.json"

echo "=== AI Gateway 测试脚本 ==="
echo

# 检查服务状态
echo "1. 检查服务状态..."
if pgrep -f ai-gateway > /dev/null; then
    echo "✓ 服务正在运行 (PID: $(pgrep -f ai-gateway))"
else
    echo "✗ 服务未运行"
    exit 1
fi

# 检查端口监听
echo
echo "2. 检查端口监听..."
if netstat -ln | grep :3001 > /dev/null; then
    echo "✓ 端口 3001 正在监听"
else
    echo "✗ 端口 3001 未监听"
fi

# 检查配置文件
echo
echo "3. 检查配置文件..."
if [ -f "$CONFIG_FILE" ]; then
    echo "✓ 配置文件存在: $CONFIG_FILE"
    echo "配置内容预览:"
    head -10 "$CONFIG_FILE"
    echo "..."
else
    echo "✗ 配置文件不存在: $CONFIG_FILE"
fi

# 测试健康检查端点
echo
echo "4. 测试健康检查端点..."
if curl -s "$AI_GATEWAY_URL/health" > /dev/null; then
    echo "✓ /health 端点响应正常"
    echo "健康检查结果:"
    curl -s "$AI_GATEWAY_URL/health" | python3 -m json.tool 2>/dev/null || curl -s "$AI_GATEWAY_URL/health"
else
    echo "✗ /health 端点无响应"
fi

# 测试模型列表端点
echo
echo "5. 测试模型列表端点..."
if curl -s "$AI_GATEWAY_URL/v1/models" > /dev/null; then
    echo "✓ /v1/models 端点响应正常"
    echo "模型列表预览:"
    curl -s "$AI_GATEWAY_URL/v1/models" | python3 -m json.tool 2>/dev/null | head -20
else
    echo "✗ /v1/models 端点无响应"
fi

# 测试 OpenAI 兼容端点
echo
echo "6. 测试 OpenAI 兼容端点..."
TEST_PAYLOAD='{"model":"test","messages":[{"role":"user","content":"Hello"}]}'
if curl -s -X POST "$AI_GATEWAY_URL/v1/chat/completions" \
    -H "Content-Type: application/json" \
    -H "Authorization: Bearer test-key" \
    -d "$TEST_PAYLOAD" > /dev/null; then
    echo "✓ /v1/chat/completions 端点响应正常"
else
    echo "✗ /v1/chat/completions 端点无响应"
fi

# 检查日志文件
echo
echo "7. 检查日志文件..."
LOG_FILE=$(grep -o '"log_file":"[^"]*"' "$CONFIG_FILE" | cut -d'"' -f4)
if [ -n "$LOG_FILE" ] && [ -f "$LOG_FILE" ]; then
    echo "✓ 日志文件存在: $LOG_FILE"
    echo "最近5行日志:"
    tail -n 5 "$LOG_FILE" 2>/dev/null || echo "无法读取日志文件"
else
    echo "✗ 日志文件不存在或无法访问"
fi

echo
echo "=== 测试完成 ==="