include $(TOPDIR)/rules.mk

PKG_NAME:=luci-app-qnatter
PKG_VERSION:=1.1.0
PKG_RELEASE:=1

LUCI_TITLE:=LuCI Support for QNatter
LUCI_DEPENDS:=+qnatter +luci-base +rpcd +uhttpd +uhttpd-mod-ubus +uci +jsonfilter
LUCI_PKGARCH:=all

define Package/$(PKG_NAME)/conffiles
/etc/config/qnatter
endef

include $(TOPDIR)/feeds/luci/luci.mk
# call BuildPackage - OpenWrt buildroot signature
