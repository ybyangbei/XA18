module("luci.controller.ai_gateway", package.seeall)

function index()
	entry({"admin", "services", "ai_gateway"}, cbi("ai_gateway"), _("AI Gateway"), 80).dependent = true
end
