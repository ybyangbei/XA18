module("luci.cbi.ai_gateway", package.seeall)

local fs = require "nixio.fs"
local util = require "luci.util"
local json = require "luci.jsonc"

-- 读取现有配置
local config_file = "/etc/ai-gateway/config.json"
local config = {}
if fs.access(config_file) then
	local f = io.open(config_file, "r")
	if f then
		local content = f:read("*all")
		f:close()
		local success, parsed = pcall(json.parse, content)
		if success then
			config = parsed
		end
	end
end

-- 主配置表
local m = Map("ai_gateway", translate("AI Gateway"),
	translate("轻量 AI LLM API 网关（FR-25/27）。支持多 provider 聚合、fallback、统一鉴权。"))

-- 服务设置部分
local s = m:section(TypedSection, "settings", translate("服务设置"))
s.anonymous = true

local en = s:option(Flag, "enabled", translate("启用服务"))
en.rmempty = false
en.default = config.enabled ~= false

local cf = s:option(Value, "config_file", translate("配置文件路径"))
cf.default = config_file
cf.datatype = "string"
cf.rmempty = false

-- Provider 设置部分
local p = m:section(TypedSection, "provider", translate("Provider 配置"))
p.anonymous = true
p.addremove = true
p.template = "cbi/tblsection"

-- Provider 名称
local name = p:option(Value, "name", translate("Provider名称"))
name.datatype = "string"
name.rmempty = false

-- Provider 平台
local platform = p:option(ListValue, "platform", translate("平台"))
platform:value("openai", "OpenAI 兼容")
platform:value("groq", "Groq")
platform:value("google", "Google Gemini")
platform:value("openrouter", "OpenRouter")
platform:value("cerebras", "Cerebras")
platform:value("nvidia", "NVIDIA NIM")
platform:value("mistral", "Mistral")
platform:value("zhipu", "智谱AI")
platform:value("baidu", "百度千帆")
platform:value("kimi", "月之暗面")
platform:value("ollama", "Ollama")
platform:value("custom", "自定义")
platform.datatype = "string"
platform.rmempty = false

-- API Key
local api_key = p:option(Value, "api_key", translate("API Key"))
api_key.password = true
api_key.datatype = "string"
api_key.rmempty = false

-- Base URL（仅自定义平台）
local base_url = p:option(Value, "base_url", translate("Base URL"))
base_url:depends({ platform = "custom" })
base_url.datatype = "string"
base_url.placeholder = "https://api.example.com/v1"

-- 模型列表（JSON格式）
local models = p:option(TextValue, "models", translate("模型列表（JSON格式）"))
models.rows = 3
models.wrap = "off"
models.datatype = "json"
models.rmempty = false

-- 提交时验证并更新配置
function m.on_commit(self, section)
	-- 构建新的配置
	local new_config = {
		listen = config.listen or ":3001",
		gateway_key = config.gateway_key or "change-me-to-a-strong-key",
		log_file = config.log_file or "/tmp/ai-gateway.log",
		enabled = self:get(section, "enabled") ~= false,
		providers = {}
	}
	
	-- 收集所有 provider
	local idx = 1
	while true do
		local name = self:get(idx, "name")
		if not name then break end
		
		local provider = {
			name = name,
			platform = self:get(idx, "platform"),
			api_key = self:get(idx, "api_key"),
			models = json.parse(self:get(idx, "models")) or {}
		}
		
		-- 如果是自定义平台，添加 base_url
		if self:get(idx, "platform") == "custom" then
			provider.base_url = self:get(idx, "base_url")
		end
		
		table.insert(new_config.providers, provider)
		idx = idx + 1
	end
	
	-- 保存配置文件
	local f = io.open(config_file, "w")
	if f then
		f:write(json.stringify(new_config, true))
		f:close()
		
		-- 重启服务
		if new_config.enabled then
			util.exec("/etc/init.d/ai-gateway restart")
		else
			util.exec("/etc/init.d/ai-gateway stop")
		end
	end
end

-- 提供默认 provider 示例
function m.on_load(self)
	if #config.providers == 0 then
		-- 添加默认 provider
		local default_providers = {
			{
				name = "Groq 示例",
				platform = "groq",
				api_key = "YOUR_GROQ_API_KEY",
				models = {"llama-3.3-70b-versatile", "mixtral-8x7b-32768"}
			},
			{
				name = "Google 示例",
				platform = "google",
				api_key = "YOUR_GEMINI_API_KEY",
				models = {"gemini-2.0-flash"}
			}
		}
		
		for i, provider in ipairs(default_providers) do
			local section = self:section()			
			self:set(section, "name", provider.name)
			self:set(section, "platform", provider.platform)
			self:set(section, "api_key", provider.api_key)
			self:set(section, "models", json.stringify(provider.models, true))
		end
	end
end

return m