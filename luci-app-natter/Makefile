include $(TOPDIR)/rules.mk

PKG_NAME:=luci-app-natter
PKG_VERSION:=1.0.0
PKG_RELEASE:=2

LUCI_TITLE:=LuCI Support for Natter
LUCI_DEPENDS:=+natter +luci-base +rpcd +uhttpd +uhttpd-mod-ubus +uci
LUCI_PKGARCH:=all

define Package/$(PKG_NAME)/conffiles
/etc/config/natter
endef

include $(TOPDIR)/feeds/luci/luci.mk
# call BuildPackage - OpenWrt buildroot signature
