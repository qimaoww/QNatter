# SPDX-License-Identifier: GPL-3.0-only

include $(TOPDIR)/rules.mk

PKG_NAME:=qnatter
PKG_VERSION:=2.2.2
PKG_RELEASE:=2

PKG_SOURCE:=$(PKG_NAME)-$(PKG_VERSION).tar.gz
PKG_SOURCE_PROTO:=git
PKG_SOURCE_URL:=https://github.com/MikeWang000000/Natter.git
PKG_SOURCE_VERSION:=ce1427650178167d2330c490d2e5cd979223e9a4
PKG_MIRROR_HASH:=skip

PKG_LICENSE:=GPL-3.0-only
PKG_LICENSE_FILES:=LICENSE
PKG_MAINTAINER:=qimaoww
PKG_BUILD_DEPENDS:=golang/host

# GO_ARCH is only defined by the golang feed build framework; map the SDK
# target architecture the same way lang/golang/golang-values.mk does so a
# missing GO_ARCH never makes go build silently target the build host.
QNATTER_GOOS?=linux
QNATTER_GOARCH?=$(subst aarch64,arm64,$(subst i386,386,$(subst loongarch64,loong64,$(subst mipsel,mipsle,$(subst mips64el,mips64le,$(subst powerpc64,ppc64,$(subst x86_64,amd64,$(ARCH))))))))
QNATTER_TARGET_FPU=$(word 2,$(subst +,$(space),$(call qstrip,$(CONFIG_CPU_TYPE))))
QNATTER_GOARM?=$(if $(filter arm,$(QNATTER_GOARCH)),$(if $(QNATTER_TARGET_FPU),$(if $(filter vfp vfpv2,$(QNATTER_TARGET_FPU)),6,7),5))
QNATTER_GOMIPS?=$(if $(filter mips mipsle,$(QNATTER_GOARCH)),$(if $(filter y,$(CONFIG_HAS_FPU)),hardfloat,softfloat))
QNATTER_GOMIPS64?=$(if $(filter mips64 mips64le,$(QNATTER_GOARCH)),$(if $(filter y,$(CONFIG_HAS_FPU)),hardfloat,softfloat))

include $(INCLUDE_DIR)/package.mk

define Package/qnatter
  SECTION:=net
  CATEGORY:=Network
  SUBMENU:=Web Servers/Proxies
  TITLE:=QNatter port exposure for full-cone NAT
  URL:=https://github.com/MikeWang000000/Natter
  DEPENDS:=+curl +nftables +firewall4 +kmod-nft-nat
endef

define Package/qnatter/description
  QNatter opens and keeps a full-cone NAT mapping alive, with optional
  forwarding and qBittorrent Web API integration for OpenWrt.
endef

define Package/qnatter/conffiles
/etc/config/qnatter
endef

define Build/Compile
	[ -n "$(QNATTER_GOARCH)" ] || { echo "qnatter: cannot map target architecture '$(ARCH)' to GOARCH" >&2; exit 1; }
	cd $(CURDIR)/../go-qnatter && CGO_ENABLED=0 GOOS="$(QNATTER_GOOS)" GOARCH="$(QNATTER_GOARCH)" GOMIPS="$(QNATTER_GOMIPS)" GOMIPS64="$(QNATTER_GOMIPS64)" GOARM="$(QNATTER_GOARM)" go build -trimpath -ldflags="-s -w" -o $(PKG_BUILD_DIR)/qnatter-go ./cmd/qnatter
endef

define Package/qnatter/install
	$(INSTALL_DIR) $(1)/usr/bin
	$(INSTALL_BIN) $(PKG_BUILD_DIR)/qnatter-go $(1)/usr/bin/qnatter-go
	$(LN) qnatter-go $(1)/usr/bin/QNatter

	$(INSTALL_DIR) $(1)/usr/share/qnatter
	$(INSTALL_BIN) ./files/qnatter-common.sh $(1)/usr/share/qnatter/qnatter-common.sh
	$(INSTALL_BIN) ./files/qnatter-qbittorrent.sh $(1)/usr/share/qnatter/qnatter-qbittorrent.sh
	$(INSTALL_BIN) ./files/qnatter-notify $(1)/usr/share/qnatter/qnatter-notify
	$(INSTALL_BIN) ./files/qnatter-run $(1)/usr/share/qnatter/qnatter-run

	$(INSTALL_DIR) $(1)/etc/config
	$(INSTALL_CONF) ./files/qnatter.config $(1)/etc/config/qnatter.default
	$(INSTALL_DIR) $(1)/etc/uci-defaults
	$(INSTALL_BIN) ./files/qnatter.uci-default $(1)/etc/uci-defaults/99-qnatter

	$(INSTALL_DIR) $(1)/etc/init.d
	$(INSTALL_BIN) ./files/qnatter.init $(1)/etc/init.d/qnatter

	$(INSTALL_DIR) $(1)/etc/hotplug.d/iface
	$(INSTALL_BIN) ./files/qnatter.hotplug $(1)/etc/hotplug.d/iface/95-qnatter
endef

define Package/qnatter/postinst
#!/bin/sh
root="$${IPKG_INSTROOT:-}"
for path in \
	/etc/config/qnatter.default \
	/etc/hotplug.d/iface/95-qnatter \
	/etc/init.d/qnatter \
	/etc/uci-defaults/99-qnatter \
	/usr/bin/qnatter-go \
	/usr/share/qnatter/qnatter-common.sh \
	/usr/share/qnatter/qnatter-notify \
	/usr/share/qnatter/qnatter-qbittorrent.sh \
	/usr/share/qnatter/qnatter-run
do
	[ -e "$$root$$path" ] || continue
	chown root:root "$$root$$path" 2>/dev/null || true
done
chown -h root:root "$$root/usr/bin/QNatter" 2>/dev/null || true
exit 0
endef

$(eval $(call BuildPackage,qnatter))
