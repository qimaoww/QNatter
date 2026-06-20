# QNatter

QNatter 是一个面向 OpenWrt/ImmortalWrt 的端口打洞集成包，基于 [MikeWang000000/Natter](https://github.com/MikeWang000000/Natter) 构建。

本仓库主要提供：

- `qnatter`：OpenWrt 软件包，包含 QNatter Go 运行核心、init 脚本、热重载、状态文件、通知脚本和自动防火墙规则。
- `luci-app-qnatter`：LuCI 管理界面，用于配置多实例、WAN 绑定、转发方式、qBittorrent、Cloudflare SRV 和日志查看。
- `go-qnatter`：用于 OpenWrt 打包的 QNatter Go 版本实现，保留 QNatter 打洞核心行为，并补充 OpenWrt 多 WAN/策略路由集成。
- `tests`：针对 init、hotplug、notify、rpcd、状态输出、包结构和 LuCI 静态检查的回归测试。

## 功能

- 支持多个 QNatter 实例分别绑定不同 WAN 接口。
- 支持 `nftables` / `nftables-snat` / `socket`，并在存在 `socat` 或 `gost` 时显示可选转发方式。
- 为每个实例维护持久 `route_slot`，生成独立的 `fwmark`、策略路由表和优先级，避免多 WAN 或同接口多实例互相覆盖。
- 自动清理 QNatter 自己创建的旧 nftables 规则和策略路由规则。
- 可选自动添加单独防火墙放行规则，不全局改防火墙策略。
- 可选更新 qBittorrent 监听端口。
- 可选根据映射公网端口更新 Cloudflare SRV 记录。
- LuCI 界面支持实例重命名、状态查看和日志查看。

## 构建

在 ImmortalWrt/OpenWrt 源码树中放置本仓库的包目录后，可以编译 APK：

```sh
cd /openwrt/immortalwrt
make package/qnatter/compile V=s CONFIG_PACKAGE_qnatter=m
make package/luci-app-qnatter/compile V=s CONFIG_PACKAGE_luci-app-qnatter=m CONFIG_PACKAGE_luci-i18n-qnatter-zh-cn=m
```

当前项目按 APK 包安装流程使用，编译产物通常位于：

```text
/openwrt/immortalwrt/bin/packages/<arch>/base/
```

## 安装

将编译出的 APK 上传到路由器后安装：

```sh
apk add --allow-untrusted --force-overwrite /tmp/qnatter-*.apk
apk add --allow-untrusted --force-overwrite /tmp/luci-app-qnatter-*.apk
apk add --allow-untrusted --force-overwrite /tmp/luci-i18n-qnatter-zh-cn-*.apk
/etc/init.d/qnatter restart
```

安装后可在 LuCI 的 `服务 -> QNatter` 中配置实例。

## 测试

本仓库提供本地回归测试：

```sh
cd /root/codex
(cd go-qnatter && go test -count=1 ./...)
for t in tests/test_qnatter_*.sh; do "$t" || exit $?; done
node --check luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances.js
node --check luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/status.js
node --check luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/log.js
```

## 致谢

NAT 打洞核心思路与上游项目来自 [MikeWang000000/Natter](https://github.com/MikeWang000000/Natter)。本仓库是在该项目基础上进行 OpenWrt/ImmortalWrt 打包、LuCI 配置界面、多实例策略路由、qBittorrent 和 Cloudflare SRV 集成。
