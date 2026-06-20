# SPDX-License-Identifier: GPL-3.0-or-later

include $(TOPDIR)/rules.mk

PKG_NAME:=natter
PKG_VERSION:=2.2.1
PKG_RELEASE:=15

PKG_SOURCE:=$(PKG_NAME)-$(PKG_VERSION).tar.gz
PKG_SOURCE_PROTO:=git
PKG_SOURCE_URL:=https://github.com/MikeWang000000/Natter.git
PKG_SOURCE_VERSION:=ce1427650178167d2330c490d2e5cd979223e9a4
PKG_MIRROR_HASH:=skip

PKG_LICENSE:=GPL-3.0-or-later
PKG_LICENSE_FILES:=LICENSE
PKG_MAINTAINER:=qimaoww
PKG_BUILD_DEPENDS:=golang/host
NATTER_GOOS?=linux
NATTER_GOARCH?=$(GO_ARCH)
NATTER_GOMIPS?=
NATTER_GOARM?=

include $(INCLUDE_DIR)/package.mk

define Package/natter
  SECTION:=net
  CATEGORY:=Network
  SUBMENU:=Web Servers/Proxies
  TITLE:=Expose ports behind full-cone NAT
  URL:=https://github.com/MikeWang000000/Natter
  DEPENDS:=+curl +nftables +firewall4 +kmod-nft-nat
endef

define Package/natter/description
  Natter opens and keeps a full-cone NAT mapping alive, with optional
  forwarding and qBittorrent Web API integration for OpenWrt.
endef

define Package/natter/conffiles
/etc/config/natter
endef

define Build/Compile
	cd $(CURDIR)/../go-natter && CGO_ENABLED=0 GOOS="$(NATTER_GOOS)" GOARCH="$(NATTER_GOARCH)" GOMIPS="$(NATTER_GOMIPS)" GOARM="$(NATTER_GOARM)" go build -trimpath -ldflags="-s -w" -o $(PKG_BUILD_DIR)/natter-go ./cmd/natter
endef

define Package/natter/install
	$(INSTALL_DIR) $(1)/usr/bin
	$(INSTALL_BIN) $(PKG_BUILD_DIR)/natter-go $(1)/usr/bin/natter-go
	$(LN) natter-go $(1)/usr/bin/Natter

	$(INSTALL_DIR) $(1)/usr/share/natter
	$(INSTALL_BIN) ./files/natter-common.sh $(1)/usr/share/natter/natter-common.sh
	$(INSTALL_BIN) ./files/natter-qbittorrent.sh $(1)/usr/share/natter/natter-qbittorrent.sh
	$(INSTALL_BIN) ./files/natter-notify $(1)/usr/share/natter/natter-notify
	$(INSTALL_BIN) ./files/natter-run $(1)/usr/share/natter/natter-run

	$(INSTALL_DIR) $(1)/etc/config
	$(INSTALL_CONF) ./files/natter.config $(1)/etc/config/natter.default
	$(INSTALL_DIR) $(1)/etc/uci-defaults
	$(INSTALL_BIN) ./files/natter.uci-default $(1)/etc/uci-defaults/99-natter

	$(INSTALL_DIR) $(1)/etc/init.d
	$(INSTALL_BIN) ./files/natter.init $(1)/etc/init.d/natter

	$(INSTALL_DIR) $(1)/etc/hotplug.d/iface
	$(INSTALL_BIN) ./files/natter.hotplug $(1)/etc/hotplug.d/iface/95-natter
endef

$(eval $(call BuildPackage,natter))
