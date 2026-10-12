# SPDX-License-Identifier: GPL-3.0-only

include $(TOPDIR)/rules.mk

PKG_NAME:=qnatter
PKG_VERSION:=2.2.2
PKG_RELEASE:=2

# The Go sources live in this repository (../go-qnatter) and are what actually
# gets compiled. Pointing PKG_SOURCE at the upstream Natter tree only served to
# create PKG_BUILD_DIR: it added a network dependency and, with
# PKG_MIRROR_HASH:=skip, disabled integrity checking on a tarball that was
# never compiled. Build/Prepare below creates that directory itself.

PKG_LICENSE:=GPL-3.0-only
PKG_LICENSE_FILES:=LICENSE
PKG_MAINTAINER:=qimaoww
PKG_BUILD_DEPENDS:=golang/host

# GO_ARCH is only defined by the golang feed build framework; map the SDK
# target architecture the same way lang/golang/golang-values.mk does so a
# missing GO_ARCH never makes go build silently target the build host.
# Use := (not ?=) for these: an inherited environment variable such as
# QNATTER_GOARCH=amd64 must never override the target mapping -- that is
# exactly how "non-x86 builds silently produced x86 packages" happened.
QNATTER_GOOS:=linux
QNATTER_GOARCH:=$(subst aarch64,arm64,$(subst i386,386,$(subst loongarch64,loong64,$(subst mipsel,mipsle,$(subst mips64el,mips64le,$(subst powerpc64le,ppc64le,$(subst powerpc64,ppc64,$(subst x86_64,amd64,$(ARCH)))))))))
QNATTER_TARGET_FPU:=$(word 2,$(subst +,$(space),$(call qstrip,$(CONFIG_CPU_TYPE))))
QNATTER_GOARM:=$(if $(filter arm,$(QNATTER_GOARCH)),$(if $(QNATTER_TARGET_FPU),$(if $(filter vfp vfpv2,$(QNATTER_TARGET_FPU)),6,7),5))
QNATTER_GOMIPS:=$(if $(filter mips mipsle,$(QNATTER_GOARCH)),$(if $(filter y,$(CONFIG_HAS_FPU)),hardfloat,softfloat))
QNATTER_GOMIPS64:=$(if $(filter mips64 mips64le,$(QNATTER_GOARCH)),$(if $(filter y,$(CONFIG_HAS_FPU)),hardfloat,softfloat))
# Recomputed independently from $(ARCH) so the post-build architecture check
# cannot be defeated by overriding the variables above.
QNATTER_EXPECTED_GOARCH:=$(subst aarch64,arm64,$(subst i386,386,$(subst loongarch64,loong64,$(subst mipsel,mipsle,$(subst mips64el,mips64le,$(subst powerpc64le,ppc64le,$(subst powerpc64,ppc64,$(subst x86_64,amd64,$(ARCH)))))))))
# x86/geode and x86/legacy ship CPUs without SSE2 (AMD Geode GX/LX). The go
# cross toolchain defaults to GO386=sse2, so force softfloat there to avoid a
# binary that dies with SIGILL on the target (mirrors golang-values.mk).
QNATTER_GO386:=$(if $(filter 386,$(QNATTER_GOARCH)),$(if $(filter y,$(CONFIG_TARGET_x86_geode)$(CONFIG_TARGET_x86_legacy)),softfloat,sse2))
# Explicit sub-architecture baselines. Go's defaults match these today; pinning
# them keeps the target ABI stable even if a future toolchain changes its
# defaults. GOARM64 stays at the v8.0 baseline that every arm64 target
# supports -- raising it to v8.2/v9.x requires mapping core names through
# lang/golang's GO_ARM64_*_CORES tables (golang-values.mk:157-186).
QNATTER_GOAMD64:=$(if $(filter amd64,$(QNATTER_GOARCH)),v1)
QNATTER_GOARM64:=$(if $(filter arm64,$(QNATTER_GOARCH)),v8.0)
QNATTER_GO_PPC64:=$(if $(filter ppc64,$(QNATTER_GOARCH)),power8)
# Go architectures this package can be built for. Anything else (e.g. OpenWrt's
# powerpc/armeb targets, which Go does not implement) fails early with a clear
# message instead of a cryptic "unsupported GOOS/GOARCH pair" from go build.
QNATTER_SUPPORTED_GOARCH:=386 amd64 arm arm64 loong64 mips mipsle mips64 mips64le ppc64 ppc64le riscv64 s390x

include $(INCLUDE_DIR)/package.mk

define Package/qnatter
  SECTION:=net
  CATEGORY:=Network
  SUBMENU:=Web Servers/Proxies
  TITLE:=QNatter port exposure for full-cone NAT
  URL:=https://github.com/MikeWang000000/Natter
  DEPENDS:=+curl +nftables +firewall4 +kmod-nft-nat @(aarch64||arm||i386||i686||loongarch64||mips||mips64||mips64el||mipsel||powerpc64||powerpc64le||riscv64||x86_64)
endef

define Package/qnatter/description
  QNatter opens and keeps a full-cone NAT mapping alive, with optional
  forwarding and qBittorrent Web API integration for OpenWrt.
endef

define Package/qnatter/conffiles
/etc/config/qnatter
endef

# The build directory is created here (no upstream tarball is downloaded any
# more); a LICENSE is copied in so that PKG_LICENSE_FILES can still be
# collected. Never let a missing LICENSE fail the build: in the feeds layout
# the repository root is one level up (feeds/qnatter/LICENSE), but when this
# directory is dropped straight into package/qnatter the parent is package/
# and holds no LICENSE, so the copy that ships inside the package wins.
define Build/Prepare
	mkdir -p $(PKG_BUILD_DIR)
	if [ -f $(CURDIR)/LICENSE ]; then \
		$(CP) $(CURDIR)/LICENSE $(PKG_BUILD_DIR)/LICENSE; \
	elif [ -f $(CURDIR)/../LICENSE ]; then \
		$(CP) $(CURDIR)/../LICENSE $(PKG_BUILD_DIR)/LICENSE; \
	fi
endef

# The post-build GOARCH self-check lives in scripts/check-goarch.sh on purpose.
# Inside a define, a doubled-dollar reference reaches the shell only after two
# rounds of make expansion (OpenWrt wraps package recipes in several layers of
# define), so inline shell variables silently collapse to their last letters and
# inline command substitution becomes the empty string - which is exactly how a
# perfectly good build reported "built binary GOARCH=uilt". Keeping the quoting
# inside a script means shell variables stay shell variables.
define Build/Compile
	[ -n "$(QNATTER_GOARCH)" ] || { echo "qnatter: cannot map target architecture '$(ARCH)' to GOARCH" >&2; exit 1; }
	[ -n "$(filter $(QNATTER_GOARCH),$(QNATTER_SUPPORTED_GOARCH))" ] || { echo "qnatter: target architecture '$(ARCH)' maps to unsupported GOARCH '$(QNATTER_GOARCH)'" >&2; exit 1; }
	cd $(CURDIR)/../go-qnatter && CGO_ENABLED=0 GOOS="$(QNATTER_GOOS)" GOARCH="$(QNATTER_GOARCH)" GOMIPS="$(QNATTER_GOMIPS)" GOMIPS64="$(QNATTER_GOMIPS64)" GOARM="$(QNATTER_GOARM)" GO386="$(QNATTER_GO386)" GOAMD64="$(QNATTER_GOAMD64)" GOARM64="$(QNATTER_GOARM64)" GO_PPC64="$(QNATTER_GO_PPC64)" go build -buildvcs=false -trimpath -ldflags="-s -w" -o $(PKG_BUILD_DIR)/qnatter-go ./cmd/qnatter
	sh $(CURDIR)/scripts/check-goarch.sh "$(PKG_BUILD_DIR)/qnatter-go" "$(QNATTER_EXPECTED_GOARCH)" "$(ARCH)"
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
