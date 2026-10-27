module("luci.cbi.ai_gateway", package.seeall)

local translate = luci.i18n.translate
local cbi = require "luci.cbi"
local Map, TypedSection, Flag, Value, ListValue, TextValue = cbi.Map, cbi.TypedSection, cbi.Flag, cbi.Value, cbi.ListValue, cbi.TextValue

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
local m = Map("ai-gateway", translate("AI Gateway"),
	translate("轻量 AI LLM API 网关（FR-25/27）。支持多 provider 聚合、fallback、统一鉴权。"))

-- 保存时立即 commit + apply，不走 LuCI 的 90s 回滚等待流程
m.apply_on_parse = true

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

-- 提交后同步：把 uci 中的设置与 provider 写入 config.json。
-- 注意：on_commit / on_after_commit 均为无参 hook（LuCI CBI 通过 _run_hooks 调用），
-- 不能用 self:get(section, ...) 依赖 section 参数；这里改从 uci 全量读取。
function m.on_after_commit(self)
	local uci = self.uci
	local all = uci:get_all("ai-gateway") or {}

	local config_file = "/etc/ai-gateway/config.json"
	for sid, sec in pairs(all) do
		if sec and sec[".type"] == "settings" then
			if sec.config_file and sec.config_file ~= "" then config_file = sec.config_file end
		end
	end

	-- 收集表单中添加的 provider（仅当 uci 中存在 provider 记录时才覆盖 json）
	local providers = {}
	local has_provider = false
	for sid, sec in pairs(all) do
		if sec and sec[".type"] == "provider" then
			has_provider = true
			local p = {
				name = sec.name or "",
				platform = sec.platform or "",
				api_key = sec.api_key or ""
			}
			if sec.base_url and sec.base_url ~= "" then p.base_url = sec.base_url end
			local ok, models = pcall(json.parse, sec.models or "[]")
			p.models = (ok and type(models) == "table") and models or {}
			table.insert(providers, p)
		end
	end

	-- 读现有 json，仅更新有变更的部分
	local cfg = {}
	local f = io.open(config_file, "r")
	if f then
		local c = f:read("*a")
		f:close()
		local ok, parsed = pcall(json.parse, c)
		if ok and type(parsed) == "table" then cfg = parsed end
	end
	if has_provider then
		cfg.providers = providers
	end

	local w = io.open(config_file, "w")
	if w then
		w:write(json.stringify(cfg, true))
		w:close()
	end
end

-- 应用后同步服务启停：必须在 uci:apply 之后执行。
-- LuCI 的 apply 流程会因配置名与服务名同名而自动重启本服务，放在 on_after_commit
-- 会被 apply 的重启覆盖；on_after_apply 在 apply 完成后运行，保证最终启停状态正确。
function m.on_after_apply(self)
	local all = self.uci:get_all("ai-gateway") or {}
	local enabled = true
	for sid, sec in pairs(all) do
		if sec and sec[".type"] == "settings" and sec.enabled == "0" then
			enabled = false
		end
	end
	if enabled then
		util.exec("/etc/init.d/ai-gateway restart")
	else
		util.exec("/etc/init.d/ai-gateway stop")
	end
end

-- 提供默认 provider 示例
function m.on_load(self)
	-- providers 由 /etc/ai-gateway/config.json 管理（CBI 绑定 uci，无法直接映射 json 记录）。
	-- 此前的默认示例填充调用了不存在的 self:section()/self:set()，会在 providers 为空时导致页面 500，
	-- 已移除。Provider 请直接在下方表格中添加，提交时由 on_commit 写入 json。
end

return m