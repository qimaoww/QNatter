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

define Package/luci-app-qnatter/postinst
#!/bin/sh
root="$${IPKG_INSTROOT:-}"
for path in \
	/usr/libexec/qnatter-log \
	/usr/libexec/qnatter-status \
	/usr/libexec/rpcd/luci.qnatter \
	/usr/share/luci/menu.d/luci-app-qnatter.json \
	/usr/share/rpcd/acl.d/luci-app-qnatter.json \
	/www/luci-static/resources/qnatter/qnatter.css \
	/www/luci-static/resources/view/qnatter/instances.js \
	/www/luci-static/resources/view/qnatter/instances-v11.js \
	/www/luci-static/resources/view/qnatter/log.js \
	/www/luci-static/resources/view/qnatter/status.js \
	/www/luci-static/resources/view/qnatter/stun.js
do
	[ -e "$$root$$path" ] || continue
	chown root:root "$$root$$path" 2>/dev/null || true
done
exit 0
endef

include $(TOPDIR)/feeds/luci/luci.mk
# call BuildPackage - OpenWrt buildroot signature
