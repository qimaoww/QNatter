# SPDX-License-Identifier: GPL-3.0-or-later

include $(TOPDIR)/rules.mk

PKG_NAME:=natter
PKG_VERSION:=2.2.1
PKG_RELEASE:=1

PKG_SOURCE:=$(PKG_NAME)-$(PKG_VERSION).tar.gz
PKG_SOURCE_PROTO:=git
PKG_SOURCE_URL:=https://github.com/MikeWang000000/Natter.git
PKG_SOURCE_VERSION:=ce1427650178167d2330c490d2e5cd979223e9a4
PKG_MIRROR_HASH:=skip

PKG_LICENSE:=GPL-3.0-or-later
PKG_LICENSE_FILES:=LICENSE
PKG_MAINTAINER:=qimaoww

include $(INCLUDE_DIR)/package.mk

define Package/natter
  SECTION:=net
  CATEGORY:=Network
  SUBMENU:=Web Servers/Proxies
  TITLE:=Expose ports behind full-cone NAT
  URL:=https://github.com/MikeWang000000/Natter
  DEPENDS:=+python3 +curl +nftables +firewall4 +kmod-nft-nat
endef

define Package/natter/description
  Natter opens and keeps a full-cone NAT mapping alive, with optional
  forwarding and qBittorrent Web API integration for OpenWrt.
endef

define Package/natter/conffiles
/etc/config/natter
endef

define Build/Compile
endef

define Package/natter/install
	$(INSTALL_DIR) $(1)/usr/bin
	$(INSTALL_BIN) $(PKG_BUILD_DIR)/natter.py $(1)/usr/bin/natter.py
	$(LN) python3 $(1)/usr/bin/Natter

	$(INSTALL_DIR) $(1)/usr/share/natter
	$(INSTALL_BIN) ./files/natter-common.sh $(1)/usr/share/natter/natter-common.sh
	$(INSTALL_BIN) ./files/natter-qbittorrent.sh $(1)/usr/share/natter/natter-qbittorrent.sh
	$(INSTALL_BIN) ./files/natter-notify $(1)/usr/share/natter/natter-notify
	$(INSTALL_BIN) ./files/natter-run $(1)/usr/share/natter/natter-run

	$(INSTALL_DIR) $(1)/etc/config
	$(INSTALL_CONF) ./files/natter.config $(1)/etc/config/natter

	$(INSTALL_DIR) $(1)/etc/init.d
	$(INSTALL_BIN) ./files/natter.init $(1)/etc/init.d/natter

	$(INSTALL_DIR) $(1)/etc/hotplug.d/iface
	$(INSTALL_BIN) ./files/natter.hotplug $(1)/etc/hotplug.d/iface/95-natter
endef

$(eval $(call BuildPackage,natter))
