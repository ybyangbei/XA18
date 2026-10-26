# AI Gateway Makefile for OpenWrt
# 轻量 AI LLM API 网关 - 交叉编译 Makefile

include $(TOPDIR)/rules.mk

PKG_NAME:=ai-gateway
PKG_RELEASE:=1

PKG_BUILD_DIR := $(BUILD_DIR)/$(PKG_NAME)

include $(INCLUDE_DIR)/package.mk

define Package/ai-gateway
  SECTION:=utils
  CATEGORY:=Utilities
  TITLE:=Lightweight AI LLM API Gateway
  URL:=https://github.com/your-org/ai-gateway
  DEPENDS:=+libustream-mbedtls
endef

define Package/ai-gateway/description
  轻量级 AI LLM API 网关，支持多 provider 聚合、fallback、统一鉴权。
  纯 Go 标准库实现，零第三方依赖，OpenAI 兼容接口。
endef

define Build/Prepare
	mkdir -p $(PKG_BUILD_DIR)
	$(CP) ./src/* $(PKG_BUILD_DIR)/
endef

define Build/Configure
	# 无需特殊配置
endef

define Build/Compile
	# 交叉编译 for arm64
	cd $(PKG_BUILD_DIR) && \
		GOOS=linux \
		GOARCH=arm64 \
		CGO_ENABLED=0 \
		go build -trimpath -ldflags "-s -w" -o $(PKG_BUILD_DIR)/ai-gateway .
endef

define Package/ai-gateway/install
	$(INSTALL_DIR) $(1)/usr/bin
	$(INSTALL_BIN) $(PKG_BUILD_DIR)/ai-gateway $(1)/usr/bin/
	
	# 安装 init 脚本
	$(INSTALL_DIR) $(1)/etc/init.d
	$(INSTALL_BIN) ./files/etc/init.d/ai-gateway $(1)/etc/init.d/
	
	# 安装配置文件
	$(INSTALL_DIR) $(1)/etc/ai-gateway
	$(INSTALL_CONF) ./files/etc/ai-gateway/config.json $(1)/etc/ai-gateway/config.json
	
	# 安装 LuCI 应用文件
	$(INSTALL_DIR) $(1)/usr/lib/lua/luci/controller
	$(INSTALL_DATA) ./files/usr/lib/lua/luci/controller/ai_gateway.lua $(1)/usr/lib/lua/luci/controller/
	
	$(INSTALL_DIR) $(1)/usr/lib/lua/luci/model/cbi
	$(INSTALL_DATA) ./files/usr/lib/lua/luci/model/cbi/ai_gateway.lua $(1)/usr/lib/lua/luci/model/cbi/
	
	$(INSTALL_DIR) $(1)/usr/share/luci/menu.d
	$(INSTALL_DATA) ./files/usr/share/luci/menu.d/ai_gateway.json $(1)/usr/share/luci/menu.d/
	
	$(INSTALL_DIR) $(1)/usr/share/rpcd/acl.d
	$(INSTALL_DATA) ./files/usr/share/rpcd/acl.d/luci-app-ai-gateway.json $(1)/usr/share/rpcd/acl.d/
	
	$(INSTALL_DIR) $(1)/usr/share/luci/view/ai_gateway
	$(INSTALL_DATA) ./files/usr/share/luci/view/ai_gateway/status.htm $(1)/usr/share/luci/view/ai_gateway/
	$(INSTALL_DATA) ./files/usr/share/luci/view/ai_gateway/health.htm $(1)/usr/share/luci/view/ai_gateway/
endef

$(eval $(call BuildPackage,ai-gateway))