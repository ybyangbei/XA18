module("luci.controller.ai_gateway", package.seeall)

function index()
	local page
	
	-- 主配置页
	page = entry({"admin", "services", "ai_gateway"}, cbi("ai_gateway"), _("AI Gateway"), 80)
	page.dependent = true
	
	-- 状态页
	page = entry({"admin", "services", "ai_gateway", "status"}, template("ai_gateway/status"), _("服务状态"), 81)
	page.dependent = true
	
	-- 健康检查页
	page = entry({"admin", "services", "ai_gateway", "health"}, template("ai_gateway/health"), _("健康检查"), 82)
	page.dependent = true
end
