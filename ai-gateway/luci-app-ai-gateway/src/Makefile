# AI Gateway Makefile for OpenWrt
# 轻量 AI LLM API 网关 - 交叉编译 Makefile

include $(TOPDIR)/rules.mk

PKG_NAME:=ai-gateway
PKG_RELEASE:=1

# 使用 feeds 中的 golang host package 进行交叉编译
HOST_BUILD_DEPENDS:=golang/host

PKG_BUILD_DIR := $(BUILD_DIR)/$(PKG_NAME)

include $(INCLUDE_DIR)/package.mk

# 使用 GoPackage 宏简化交叉编译
include $(INCLUDE_DIR)/golang.mk

# Go 包路径
GO_PKG:=github.com/your-org/ai-gateway
GO_PKG_LDFLAGS:=-s -w
GO_PKG_BUILD_PKG:=$(GO_PKG)

# 设置目标架构
export GOOS:=linux
export GOARCH:=arm64
export CGO_ENABLED:=0

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
	mkdir -p $(PKG_BUILD_DIR)/src/$(GO_PKG)
	$(CP) ./src/* $(PKG_BUILD_DIR)/src/$(GO_PKG)/
endef

# 使用 golang.mk 提供的 Build/Compile
# 它会自动调用正确的交叉编译器

define Build/Compile
	# 使用 golang.mk 的编译逻辑
	$(call GoPackage/Build/Compile)
	# 确保二进制在预期位置
	cp $(PKG_BUILD_DIR)/src/$(GO_PKG)/ai-gateway $(PKG_BUILD_DIR)/ai-gateway
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