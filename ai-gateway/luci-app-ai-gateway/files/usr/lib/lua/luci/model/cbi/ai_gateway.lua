module("luci.cbi.ai_gateway", package.seeall)

-- 基础骨架：配置文件路径 + 启用开关（FR-26 待完善：provider 动态编辑 / 状态 / 日志）
local m = Map("ai_gateway", translate("AI Gateway"),
	translate("轻量 AI LLM API 网关（FR-25/27）。服务配置见 /etc/ai-gateway/config.json"))

local s = m:section(TypedSection, "settings", translate("服务设置"))
s.anonymous = true

local en = s:option(Flag, "enabled", translate("启用"))
en.rmempty = false

local cf = s:option(Value, "config_file", translate("配置文件路径"))
cf.default = "/etc/ai-gateway/config.json"

return m
