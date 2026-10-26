include $(TOPDIR)/rules.mk

PKG_NAME:=luci-app-ai-gateway
PKG_VERSION:=0.2.0
PKG_RELEASE:=1

PKG_BUILD_DIR:=$(BUILD_DIR)/$(PKG_NAME)

include $(INCLUDE_DIR)/package.mk

# 目标架构（本项目 IPQ60xx = aarch64/arm64；其他平台按需覆盖）
GOARCH ?= arm64

define Package/$(PKG_NAME)
  SECTION:=luci
  CATEGORY:=LuCI
  SUBMENU:=3. Applications
  TITLE:=Lightweight AI LLM API gateway
  DEPENDS:=+luci-base +luci-compat
endef

define Package/$(PKG_NAME)/description
  Lightweight self-hosted OpenAI-compatible LLM gateway:
  multi-provider routing, fallback, unified auth.
  Pure Go static binary, no runtime dependencies.
endef

# Go 源码位于本包目录的 src/ 子目录（CI 会把 ai-gateway 根目录的 *.go 和 go.mod 复制进来）
define Build/Prepare
	mkdir -p $(PKG_BUILD_DIR)
	$(CP) $(CURDIR)/src/go.mod $(PKG_BUILD_DIR)/
	$(CP) $(CURDIR)/src/main.go $(PKG_BUILD_DIR)/
	$(CP) $(CURDIR)/src/config.go $(PKG_BUILD_DIR)/
	$(CP) $(CURDIR)/src/proxy.go $(PKG_BUILD_DIR)/
endef

define Build/Compile
	cd $(PKG_BUILD_DIR) && GOOS=linux GOARCH=$(GOARCH) CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o ai-gateway .
endef

define Package/$(PKG_NAME)/install
	$(INSTALL_DIR) $(1)/usr/bin
	$(INSTALL_DIR) $(1)/etc/init.d
	$(INSTALL_DIR) $(1)/etc/ai-gateway
	$(INSTALL_DIR) $(1)/etc/config
	$(INSTALL_DIR) $(1)/usr/share/luci/menu.d
	$(INSTALL_DIR) $(1)/usr/share/rpcd/acl.d
	$(INSTALL_DIR) $(1)/usr/lib/lua/luci/controller
	$(INSTALL_DIR) $(1)/usr/lib/lua/luci/model/cbi
	$(INSTALL_BIN) $(PKG_BUILD_DIR)/ai-gateway $(1)/usr/bin/
	$(INSTALL_BIN) ./files/etc/init.d/ai-gateway $(1)/etc/init.d/
	$(INSTALL_DATA) ./files/etc/ai-gateway/config.json $(1)/etc/ai-gateway/
	$(INSTALL_DATA) ./files/etc/config/ai-gateway $(1)/etc/config/
	$(INSTALL_DATA) ./files/usr/share/rpcd/acl.d/*.json $(1)/usr/share/rpcd/acl.d/
	$(INSTALL_DATA) ./files/usr/share/luci/menu.d/*.json $(1)/usr/share/luci/menu.d/
	$(INSTALL_DATA) ./files/usr/lib/lua/luci/controller/*.lua $(1)/usr/lib/lua/luci/controller/
	$(INSTALL_DATA) ./files/usr/lib/lua/luci/model/cbi/*.lua $(1)/usr/lib/lua/luci/model/cbi/
endef

$(eval $(call BuildPackage,$(PKG_NAME)))
